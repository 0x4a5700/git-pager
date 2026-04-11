package picker

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// SelectedMsg is emitted when the user picks a file. Path is
// repo-root relative.
type SelectedMsg struct{ Path string }

type entry struct {
	name  string
	isDir bool
}

// Model navigates a flat list of tracked files as a directory tree,
// one level at a time. Up/down moves the cursor; right/enter
// descends into a directory or emits SelectedMsg for a file;
// left/backspace ascends.
type Model struct {
	label   string
	files   []string
	dir     string // current subdirectory, "" at root
	cursor  int
	entries []entry
}

func NewModel(label string, files []string) Model {
	m := Model{label: label, files: files}
	m.refresh()
	return m
}

func (m *Model) refresh() {
	m.entries = listEntries(m.files, m.dir)
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case "enter", "right", "l":
		if len(m.entries) == 0 {
			return m, nil
		}
		e := m.entries[m.cursor]
		if e.isDir {
			m.dir = joinDir(m.dir, e.name)
			m.cursor = 0
			m.refresh()
			return m, nil
		}
		path := joinDir(m.dir, e.name)
		return m, func() tea.Msg { return SelectedMsg{Path: path} }
	case "left", "backspace", "h":
		if m.dir == "" {
			return m, nil
		}
		if i := strings.LastIndex(m.dir, "/"); i >= 0 {
			m.dir = m.dir[:i]
		} else {
			m.dir = ""
		}
		m.cursor = 0
		m.refresh()
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	header := m.label
	if m.dir != "" {
		header += "/" + m.dir
	}
	b.WriteString(header + "\n\n")
	if len(m.entries) == 0 {
		b.WriteString("(empty)\n")
	}
	for i, e := range m.entries {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		name := e.name
		if e.isDir {
			name += "/"
		}
		b.WriteString(marker + name + "\n")
	}
	b.WriteString("\n↑/↓ move  ← up-dir  →/enter open  q quit\n")
	return b.String()
}

func joinDir(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// listEntries returns the immediate children of dir among the flat
// list of repo-relative files. files must be lexicographically
// sorted (git ls-files output is). Directories come before regular
// files in the result; within each group the order is preserved
// from the input, so already-sorted input yields sorted output.
func listEntries(files []string, dir string) []entry {
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	seen := map[string]bool{}
	var dirs, regs []entry
	for _, f := range files {
		if !strings.HasPrefix(f, prefix) {
			continue
		}
		rest := f[len(prefix):]
		if rest == "" {
			continue
		}
		if i := strings.Index(rest, "/"); i >= 0 {
			name := rest[:i]
			if !seen[name] {
				seen[name] = true
				dirs = append(dirs, entry{name: name, isDir: true})
			}
		} else {
			regs = append(regs, entry{name: rest, isDir: false})
		}
	}
	return append(dirs, regs...)
}
