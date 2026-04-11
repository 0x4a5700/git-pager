package tui

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0x4a5700/git-pager/internal/git"
	"github.com/0x4a5700/git-pager/internal/tui/pager"
	"github.com/0x4a5700/git-pager/internal/tui/picker"
)

type mode int

const (
	modePicker mode = iota
	modePager
)

// sourceFactory builds a pager.Source for a repo-relative file. The
// production factory calls git.NewSource; tests inject a fake to
// drive the root model without shelling out to git.
type sourceFactory func(repoDir, relPath string) (pager.Source, error)

var prodFactory sourceFactory = func(repoDir, relPath string) (pager.Source, error) {
	return git.NewSource(filepath.Join(repoDir, relPath))
}

// Model is the root Bubble Tea model. It owns the picker and, once
// a file is chosen, the pager for that file. It is the only model
// that satisfies tea.Model; picker.Model and pager.Model are
// sub-models with concrete Update return types.
type Model struct {
	repoDir string
	mode    mode
	picker  picker.Model
	pager   pager.Model
	err     error
	factory sourceFactory
}

// NewModel builds the root model wired to the real git.NewSource.
func NewModel(repoDir string, files []string) Model {
	return newModel(repoDir, files, prodFactory)
}

// newModel is the test-friendly constructor that lets callers swap
// in a fake source factory.
func newModel(repoDir string, files []string, f sourceFactory) Model {
	return Model{
		repoDir: repoDir,
		mode:    modePicker,
		picker:  picker.NewModel(filepath.Base(repoDir), files),
		factory: f,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// In the error state, the user can only quit or dismiss with esc.
	if m.err != nil {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "esc":
				m.err = nil
				return m, nil
			}
		}
		return m, nil
	}

	// Intercept picker selections: build a pager and flip modes.
	if sel, ok := msg.(picker.SelectedMsg); ok {
		src, err := m.factory(m.repoDir, sel.Path)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.pager = pager.NewModel(sel.Path, src)
		m.mode = modePager
		return m, nil
	}

	// esc in pager mode returns to the picker.
	if key, ok := msg.(tea.KeyMsg); ok && m.mode == modePager && key.String() == "esc" {
		m.mode = modePicker
		return m, nil
	}

	switch m.mode {
	case modePicker:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	case modePager:
		var cmd tea.Cmd
		m.pager, cmd = m.pager.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("error: %v\n\nesc to dismiss, q to quit\n", m.err)
	}
	switch m.mode {
	case modePager:
		return m.pager.View()
	default:
		return m.picker.View()
	}
}
