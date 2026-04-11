package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0x4a5700/git-pager/internal/git"
)

// Source is the read-only view of a file's git history that Model
// consumes. Keeping it as an interface lets tests drop in a fake
// without spawning git subprocesses.
type Source interface {
	Commits() []git.Commit
	Content(hash string) (string, error)
}

// The root Bubble Tea model. idx == 0 is the newest commit; it grows
// as the user walks backwards in time.
type Model struct {
	path    string
	src     Source
	idx     int
	content string
	err     error
}

func NewModel(path string, src Source) Model {
	m := Model{path: path, src: src}
	m.load()
	return m
}

// load refreshes m.content for the current idx. It is a pointer
// method so it can be called on the Update value receiver's local m.
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
	m.err = nil
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
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
	status := fmt.Sprintf("%s  •  %s  •  [%d/%d %s]  %s",
		m.path, c.ShortHash, m.idx+1, len(commits), position, c.Subject)
	return m.content + "\n" + status + "\n"
}
