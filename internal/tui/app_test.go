package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0x4a5700/git-pager/internal/git"
	"github.com/0x4a5700/git-pager/internal/tui/pager"
	"github.com/0x4a5700/git-pager/internal/tui/picker"
)

// fakeSource is a minimal pager.Source for the root tests.
type fakeSource struct {
	commits  []git.Commit
	contents map[string]string
	diffs    map[string]string
	blames   map[string][]git.BlameLine
}

func (f *fakeSource) Commits() []git.Commit                  { return f.commits }
func (f *fakeSource) Content(h string) (string, error)       { return f.contents[h], nil }
func (f *fakeSource) Diff(h string) (string, error)          { return f.diffs[h], nil }
func (f *fakeSource) Blame(h string) ([]git.BlameLine, error) { return f.blames[h], nil }

var stockFiles = []string{"a.txt", "sub/b.txt"}

func okFactory() sourceFactory {
	return func(repoDir, relPath string) (pager.Source, error) {
		return &fakeSource{
			commits: []git.Commit{
				{Hash: "h1", ShortHash: "h1", Subject: "only commit"},
			},
			contents: map[string]string{"h1": "content of " + relPath},
		}, nil
	}
}

func failFactory(err error) sourceFactory {
	return func(repoDir, relPath string) (pager.Source, error) {
		return nil, err
	}
}

func keyType(kt tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: kt} }
func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func asModel(t *testing.T, m tea.Model) Model {
	t.Helper()
	got, ok := m.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", m)
	}
	return got
}

// mustNext strips the Cmd from an Update result so tests can chain.
func mustNext(next tea.Model, _ tea.Cmd) tea.Model { return next }

func TestRoot_StartsInPicker(t *testing.T) {
	m := newModel("/repo", stockFiles, okFactory())
	if m.mode != modePicker {
		t.Errorf("mode = %d, want picker", m.mode)
	}
	if !strings.Contains(m.View(), "a.txt") {
		t.Errorf("picker view missing a.txt:\n%s", m.View())
	}
}

func TestRoot_ForwardsKeysToPicker(t *testing.T) {
	m := newModel("/repo", stockFiles, okFactory())
	before := m.View()
	got := asModel(t, mustNext(m.Update(keyType(tea.KeyDown))))
	if got.View() == before {
		t.Errorf("view unchanged after down key")
	}
}

func TestRoot_SelectedMsgTransitionsToPager(t *testing.T) {
	m := newModel("/repo", stockFiles, okFactory())
	got := asModel(t, mustNext(m.Update(picker.SelectedMsg{Path: "a.txt"})))
	if got.mode != modePager {
		t.Errorf("mode = %d, want pager", got.mode)
	}
	got = asModel(t, mustNext(got.Update(tea.WindowSizeMsg{Width: 200, Height: 40})))
	view := got.View()
	if !strings.Contains(view, "content of a.txt") {
		t.Errorf("view missing content:\n%s", view)
	}
	if !strings.Contains(view, "a.txt") {
		t.Errorf("view missing path:\n%s", view)
	}
}

func TestRoot_SelectedMsgFactoryErrorShowsError(t *testing.T) {
	m := newModel("/repo", stockFiles, failFactory(fmt.Errorf("boom")))
	got := asModel(t, mustNext(m.Update(picker.SelectedMsg{Path: "a.txt"})))
	if !strings.Contains(got.View(), "boom") {
		t.Errorf("view missing 'boom':\n%s", got.View())
	}
	if got.mode == modePager {
		t.Errorf("should not have transitioned to pager on error")
	}
}

func TestRoot_EscFromPagerReturnsToPicker(t *testing.T) {
	m := newModel("/repo", stockFiles, okFactory())
	m = asModel(t, mustNext(m.Update(picker.SelectedMsg{Path: "a.txt"})))
	if m.mode != modePager {
		t.Fatalf("setup: not in pager, mode = %d", m.mode)
	}
	m = asModel(t, mustNext(m.Update(keyType(tea.KeyEsc))))
	if m.mode != modePicker {
		t.Errorf("mode = %d, want picker", m.mode)
	}
}

func TestRoot_ForwardsKeysToPager(t *testing.T) {
	// Give the pager a multi-commit history so ← has something to do.
	f := func(repoDir, relPath string) (pager.Source, error) {
		return &fakeSource{
			commits: []git.Commit{
				{Hash: "h1", ShortHash: "h1", Subject: "new"},
				{Hash: "h0", ShortHash: "h0", Subject: "old"},
			},
			contents: map[string]string{"h1": "new-body", "h0": "old-body"},
		}, nil
	}
	m := newModel("/repo", stockFiles, f)
	m = asModel(t, mustNext(m.Update(picker.SelectedMsg{Path: "a.txt"})))
	m = asModel(t, mustNext(m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})))
	m = asModel(t, mustNext(m.Update(keyType(tea.KeyLeft))))
	if !strings.Contains(m.View(), "old-body") {
		t.Errorf("pager did not step back:\n%s", m.View())
	}
}

func TestRoot_ErrorStateQuitOnQ(t *testing.T) {
	m := newModel("/repo", stockFiles, failFactory(fmt.Errorf("boom")))
	m = asModel(t, mustNext(m.Update(picker.SelectedMsg{Path: "a.txt"})))
	_, cmd := m.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("got %T, want QuitMsg", cmd())
	}
}

func TestRoot_ErrorStateEscClears(t *testing.T) {
	m := newModel("/repo", stockFiles, failFactory(fmt.Errorf("boom")))
	m = asModel(t, mustNext(m.Update(picker.SelectedMsg{Path: "a.txt"})))
	if m.err == nil {
		t.Fatal("setup: err not set")
	}
	m = asModel(t, mustNext(m.Update(keyType(tea.KeyEsc))))
	if m.err != nil {
		t.Errorf("err not cleared: %v", m.err)
	}
}
