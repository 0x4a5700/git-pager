package pager

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0x4a5700/git-pager/internal/git"
)

var statusBarStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("15")).
	Foreground(lipgloss.Color("0")).
	Bold(true)

var (
	lineNumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	diffAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))  // green
	diffDelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("160")) // red
	diffHunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("44"))  // cyan
	diffMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // dim
)

// reservedLines is the number of fixed lines above the content area
// (status bar + blank line + hint line).
const reservedLines = 3

// Source is the read-only view of a file's git history that Model
// consumes. Keeping it as an interface lets tests drop in a fake
// without spawning git subprocesses.
type Source interface {
	Commits() []git.Commit
	Content(hash string) (string, error)
	Diff(hash string) (string, error)
}

// pane holds the scroll state and cached text for one of the two
// views (file content or diff) the pager can display.
//
// For the diff pane, oldNums/newNums are parallel to lines and hold
// the old- and new-file line numbers for each diff row (0 means no
// number applies — e.g. file/hunk headers, '+' has no old #, '-' has
// no new #). addedNums is the set of new-file line numbers that were
// newly added by this commit; it is used by inline mode to highlight
// those lines when rendering the file pane. All three are nil for
// the file pane.
type pane struct {
	content   string
	lines     []string
	oldNums   []int
	newNums   []int
	addedNums map[int]bool
	scrollY   int
	loaded    bool
	err       error
}

// viewMode selects how the pager renders the current commit.
type viewMode int

const (
	// modeFile shows the plain file content at the current commit.
	modeFile viewMode = iota
	// modeDiff shows the unified diff against the parent commit.
	modeDiff
	// modeInline shows the file content with lines added in this
	// commit highlighted in green. Scroll state is shared with file
	// mode so flipping between them doesn't jump around.
	modeInline
	numModes
)

func (v viewMode) label() string {
	switch v {
	case modeFile:
		return "file"
	case modeDiff:
		return "diff"
	case modeInline:
		return "inline"
	}
	return "?"
}

// needsDiff reports whether the given mode requires the diff to be
// loaded — either to render it directly or to know which lines are
// newly added in the current commit.
func (v viewMode) needsDiff() bool {
	return v == modeDiff || v == modeInline
}

// Model pages through the history of a single file. idx == 0 is the
// newest commit; it grows as the user walks backwards in time.
type Model struct {
	path   string
	src    Source
	idx    int
	file   pane
	diff   pane
	mode   viewMode
	height int
	width  int
	err    error
}

func NewModel(path string, src Source) Model {
	m := Model{path: path, src: src}
	m.load()
	return m
}

// load fetches the file content at m.idx and resets both panes. The
// diff pane is lazy-loaded on first toggle into diff mode.
func (m *Model) load() {
	commits := m.src.Commits()
	if len(commits) == 0 {
		m.err = fmt.Errorf("no history for %s", m.path)
		return
	}
	if m.idx < 0 || m.idx >= len(commits) {
		m.err = fmt.Errorf("index %d out of range [0,%d)", m.idx, len(commits))
		return
	}
	content, err := m.src.Content(commits[m.idx].Hash)
	if err != nil {
		m.err = err
		return
	}
	m.file = pane{
		content: content,
		lines:   strings.Split(content, "\n"),
		loaded:  true,
	}
	m.diff = pane{}
	m.err = nil
	// If the current mode needs diff data (diff or inline), refetch
	// it now — otherwise diff mode would render an empty pane and
	// divide by zero on the scroll percentage, and inline mode would
	// be missing its highlight set.
	if m.mode.needsDiff() {
		m.loadDiff()
	}
}

// loadDiff fetches and caches the diff for the current commit.
func (m *Model) loadDiff() {
	if m.diff.loaded {
		return
	}
	commits := m.src.Commits()
	diff, err := m.src.Diff(commits[m.idx].Hash)
	if err != nil {
		m.diff = pane{loaded: true, err: err}
		return
	}
	lines := strings.Split(diff, "\n")
	oldNums, newNums := diffLineNumbers(lines)
	added := make(map[int]bool)
	for i, line := range lines {
		if newNums[i] > 0 && strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added[newNums[i]] = true
		}
	}
	m.diff = pane{
		content:   diff,
		lines:     lines,
		oldNums:   oldNums,
		newNums:   newNums,
		addedNums: added,
		loaded:    true,
	}
}

// active returns the pane currently being displayed. Inline mode
// reuses the file pane so its scroll state is shared.
func (m *Model) active() *pane {
	if m.mode == modeDiff {
		return &m.diff
	}
	return &m.file
}

// contentHeight returns the number of lines available for content.
// Returns 0 when the terminal height is not yet known.
func (m Model) contentHeight() int {
	if m.height <= reservedLines {
		return 0
	}
	return m.height - reservedLines
}

// clampScroll keeps the active pane's scrollY within valid bounds.
func (m *Model) clampScroll() {
	p := m.active()
	ch := m.contentHeight()
	if ch == 0 {
		p.scrollY = 0
		return
	}
	maxScroll := len(p.lines) - ch
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scrollY > maxScroll {
		p.scrollY = maxScroll
	}
	if p.scrollY < 0 {
		p.scrollY = 0
	}
}

func (m Model) Init() tea.Cmd { return nil }

// Update returns the concrete pager.Model rather than tea.Model. It
// is a sub-model: the root tui.Model is what actually satisfies the
// bubbletea program interface.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.clampScroll()
		return m, nil

	case tea.KeyMsg:
		ch := m.contentHeight()
		if ch == 0 {
			ch = 10 // sensible page size before first WindowSizeMsg
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "left":
			if m.idx < len(m.src.Commits())-1 {
				m.idx++
				m.load()
			}
		case "right":
			if m.idx > 0 {
				m.idx--
				m.load()
			}
		case "d":
			m.mode = (m.mode + 1) % numModes
			if m.mode.needsDiff() {
				m.loadDiff()
			}
		case "down", "j":
			m.active().scrollY++
			m.clampScroll()
		case "up", "k":
			m.active().scrollY--
			m.clampScroll()
		case "pgdown", " ":
			m.active().scrollY += ch
			m.clampScroll()
		case "pgup", "b":
			m.active().scrollY -= ch
			m.clampScroll()
		case "g", "home":
			m.active().scrollY = 0
		case "G", "end":
			m.active().scrollY = len(m.active().lines) - ch
			m.clampScroll()
		}
	}
	return m, nil
}

// diffLineNumbers walks a unified diff and returns, for each input
// line, the corresponding old- and new-file line numbers. A zero
// entry means that side has no number at that row (headers, '+' on
// the old side, '-' on the new side). Hunk header starts reset the
// counters to the values declared in the "@@ -a,b +c,d @@" marker.
func diffLineNumbers(lines []string) (oldNums, newNums []int) {
	oldNums = make([]int, len(lines))
	newNums = make([]int, len(lines))
	var oldN, newN int
	inHunk := false
	for i, line := range lines {
		if strings.HasPrefix(line, "@@") {
			if o, n, ok := parseHunkHeader(line); ok {
				oldN = o
				newN = n
				inHunk = true
			}
			continue
		}
		if !inHunk {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			// File markers appear before the first hunk so inHunk is
			// normally false here; guard defensively.
		case strings.HasPrefix(line, "+"):
			newNums[i] = newN
			newN++
		case strings.HasPrefix(line, "-"):
			oldNums[i] = oldN
			oldN++
		case strings.HasPrefix(line, " "):
			oldNums[i] = oldN
			newNums[i] = newN
			oldN++
			newN++
		}
	}
	return oldNums, newNums
}

// parseHunkHeader extracts the old and new starting line numbers
// from a unified diff hunk header like "@@ -10,7 +10,8 @@ func bar()".
// Accepts both "-a,b" and single-number "-a" forms.
func parseHunkHeader(line string) (oldStart, newStart int, ok bool) {
	rest := strings.TrimPrefix(line, "@@")
	end := strings.Index(rest, "@@")
	if end < 0 {
		return 0, 0, false
	}
	parts := strings.Fields(rest[:end])
	if len(parts) < 2 {
		return 0, 0, false
	}
	if !strings.HasPrefix(parts[0], "-") || !strings.HasPrefix(parts[1], "+") {
		return 0, 0, false
	}
	oldField := strings.SplitN(parts[0][1:], ",", 2)[0]
	newField := strings.SplitN(parts[1][1:], ",", 2)[0]
	o, err1 := strconv.Atoi(oldField)
	n, err2 := strconv.Atoi(newField)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return o, n, true
}

// formatDiffGutterCell renders one side of a diff gutter: a
// right-aligned number of the given width, or a blank pad when n == 0.
func formatDiffGutterCell(n, width int) string {
	if n == 0 {
		return strings.Repeat(" ", width)
	}
	return fmt.Sprintf("%*d", width, n)
}

// styleDiffLine colorizes one line of unified diff output.
func styleDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"),
		strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "),
		strings.HasPrefix(line, "new file"), strings.HasPrefix(line, "deleted file"),
		strings.HasPrefix(line, "similarity "), strings.HasPrefix(line, "rename "):
		return diffMetaStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return diffHunkStyle.Render(line)
	case strings.HasPrefix(line, "+"):
		return diffAddStyle.Render(line)
	case strings.HasPrefix(line, "-"):
		return diffDelStyle.Render(line)
	}
	return line
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("error: %v\n", m.err)
	}
	commits := m.src.Commits()
	c := commits[m.idx]
	position := "current"
	if m.idx > 0 {
		position = fmt.Sprintf("%d back", m.idx)
	}

	ch := m.contentHeight()
	if ch == 0 {
		return "" // wait for WindowSizeMsg before rendering content
	}

	p := m.active()
	if p.err != nil {
		return fmt.Sprintf("error: %v\n", p.err)
	}

	end := p.scrollY + ch
	if end > len(p.lines) {
		end = len(p.lines)
	}
	visibleLines := p.lines[p.scrollY:end]

	totalLines := len(p.lines)
	scrollInfo := fmt.Sprintf("%d%%", 100*(p.scrollY+ch)/totalLines)
	if p.scrollY+ch >= totalLines {
		scrollInfo = "100%"
	}
	if p.scrollY == 0 && ch >= totalLines {
		scrollInfo = "All"
	}

	// Pad to width-1 to leave the last column empty. Writing exactly
	// m.width chars triggers terminal auto-wrap on some terminals,
	// which turns each line into two rows.
	padWidth := m.width - 1
	if padWidth < 0 {
		padWidth = 0
	}
	lineStyle := lipgloss.NewStyle().Width(padWidth).MaxWidth(padWidth)

	// Gutter layout depends on mode.
	//   file/inline mode: one column of 1-based line numbers
	//   diff mode:        two columns (old | new file line numbers),
	//                     blank on the side that does not apply
	// Each column is sized to its widest number; a single space
	// separates columns and trails the gutter.
	var (
		fileDigits, oldDigits, newDigits, gutterWidth int
	)
	if m.mode == modeDiff {
		maxOld, maxNew := 0, 0
		for _, n := range p.oldNums {
			if n > maxOld {
				maxOld = n
			}
		}
		for _, n := range p.newNums {
			if n > maxNew {
				maxNew = n
			}
		}
		if maxOld > 0 || maxNew > 0 {
			oldDigits = len(strconv.Itoa(maxOld))
			newDigits = len(strconv.Itoa(maxNew))
			gutterWidth = oldDigits + newDigits + 2
		}
	} else {
		fileDigits = len(strconv.Itoa(totalLines))
		gutterWidth = fileDigits + 1
	}
	contentWidth := padWidth - gutterWidth
	if contentWidth < 1 {
		contentWidth = 1
	}
	contentStyle := lipgloss.NewStyle().Width(contentWidth).MaxWidth(contentWidth)
	// Inline mode highlights added lines with the same green used
	// for '+' lines in diff mode.
	inlineAddStyle := contentStyle.Foreground(lipgloss.Color("34"))

	status := fmt.Sprintf("%s  •  %s  •  [%d/%d %s]  %s  %s  %s",
		m.path, c.ShortHash, m.idx+1, len(commits), position, c.Subject, m.mode.label(), scrollInfo)
	hint := "(esc: back, ←/→: older/newer, d: file/diff/inline, j/k: scroll, g/G: top/bottom)"

	paddedLines := make([]string, len(visibleLines))
	for i, l := range visibleLines {
		idx := p.scrollY + i
		if m.mode == modeDiff {
			var gutter string
			if gutterWidth > 0 {
				gutter = lineNumStyle.Render(
					formatDiffGutterCell(p.oldNums[idx], oldDigits) + " " +
						formatDiffGutterCell(p.newNums[idx], newDigits) + " ",
				)
			}
			// Truncate/pad first, then colorize so ANSI codes don't
			// confuse width measurement.
			paddedLines[i] = gutter + styleDiffLine(contentStyle.Render(l))
			continue
		}
		lineNum := idx + 1
		gutter := lineNumStyle.Render(fmt.Sprintf("%*d ", fileDigits, lineNum))
		if m.mode == modeInline && m.diff.addedNums[lineNum] {
			paddedLines[i] = gutter + inlineAddStyle.Render(l)
			continue
		}
		paddedLines[i] = gutter + contentStyle.Render(l)
	}

	return statusBarStyle.Width(padWidth).MaxWidth(padWidth).Render(status) + "\n" +
		lineStyle.Render(hint) + "\n" +
		lineStyle.Render("") + "\n" +
		strings.Join(paddedLines, "\n")
}
