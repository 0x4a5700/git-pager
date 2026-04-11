package pager

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/0x4a5700/git-pager/internal/git"
)

var statusBarStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("15")).
	Foreground(lipgloss.Color("0")).
	Bold(true)

// reservedLines is the number of fixed lines above the content area
// (status bar + blank line + hint line).
const reservedLines = 3

// Source is the read-only view of a file's git history that Model
// consumes. Keeping it as an interface lets tests drop in a fake
// without spawning git subprocesses.
type Source interface {
	Commits() []git.Commit
	Content(hash string) (string, error)
}

// Model pages through the history of a single file. idx == 0 is the
// newest commit; it grows as the user walks backwards in time.
type Model struct {
	path    string
	src     Source
	idx     int
	content string
	lines   []string
	scrollY int
	height  int
	width   int
	err     error
}

func NewModel(path string, src Source) Model {
	m := Model{path: path, src: src}
	m.load()
	return m
}

// load refreshes m.content for the current idx and resets scroll.
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
	m.content = content
	m.lines = strings.Split(content, "\n")
	m.scrollY = 0
	m.err = nil
}

// contentHeight returns the number of lines available for file content.
// Returns 0 (show all) when the terminal height is not yet known.
func (m Model) contentHeight() int {
	if m.height <= reservedLines {
		return 0
	}
	return m.height - reservedLines
}

// clampScroll keeps scrollY within valid bounds for the current content.
func (m *Model) clampScroll() {
	ch := m.contentHeight()
	if ch == 0 {
		m.scrollY = 0
		return
	}
	maxScroll := len(m.lines) - ch
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}
	if m.scrollY < 0 {
		m.scrollY = 0
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
		case "down", "j":
			m.scrollY++
			m.clampScroll()
		case "up", "k":
			m.scrollY--
			m.clampScroll()
		case "pgdown", " ":
			m.scrollY += ch
			m.clampScroll()
		case "pgup", "b":
			m.scrollY -= ch
			m.clampScroll()
		case "g", "home":
			m.scrollY = 0
		case "G", "end":
			m.scrollY = len(m.lines) - ch
			m.clampScroll()
		}
	}
	return m, nil
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
	end := m.scrollY + ch
	if end > len(m.lines) {
		end = len(m.lines)
	}
	visibleLines := m.lines[m.scrollY:end]

	totalLines := len(m.lines)
	scrollInfo := fmt.Sprintf("%d%%", 100*(m.scrollY+ch)/totalLines)
	if m.scrollY+ch >= totalLines {
		scrollInfo = "100%"
	}
	if m.scrollY == 0 && ch >= totalLines {
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

	status := fmt.Sprintf("%s  •  %s  •  [%d/%d %s]  %s  %s",
		m.path, c.ShortHash, m.idx+1, len(commits), position, c.Subject, scrollInfo)
	hint := "(esc: back to picker, ←/→: older/newer, j/k: scroll, g/G: top/bottom)"

	paddedLines := make([]string, len(visibleLines))
	for i, l := range visibleLines {
		paddedLines[i] = lineStyle.Render(l)
	}

	return statusBarStyle.Width(padWidth).MaxWidth(padWidth).Render(status) + "\n" +
		lineStyle.Render(hint) + "\n" +
		lineStyle.Render("") + "\n" +
		strings.Join(paddedLines, "\n")
}
