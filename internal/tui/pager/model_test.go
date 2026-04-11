package pager

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0x4a5700/git-pager/internal/git"
)

type fakeSource struct {
	commits  []git.Commit
	contents map[string]string
	err      error
}

func (f *fakeSource) Commits() []git.Commit { return f.commits }
func (f *fakeSource) Content(hash string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.contents[hash], nil
}

func newFake() *fakeSource {
	return &fakeSource{
		commits: []git.Commit{
			{Hash: "hhhhhhhhhh", ShortHash: "hhhhhhh", Subject: "newest-msg"},
			{Hash: "gggggggggg", ShortHash: "ggggggg", Subject: "middle-msg"},
			{Hash: "ffffffffff", ShortHash: "fffffff", Subject: "oldest-msg"},
		},
		contents: map[string]string{
			"hhhhhhhhhh": "body-newest",
			"gggggggggg": "body-middle",
			"ffffffffff": "body-oldest",
		},
	}
}

func keyType(kt tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: kt} }
func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// withSize sends a WindowSizeMsg so View() renders content.
func withSize(m Model) Model {
	m, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	return m
}

func TestNewModel_StartsAtNewest(t *testing.T) {
	m := NewModel("a.txt", newFake())
	if m.idx != 0 {
		t.Errorf("idx = %d, want 0", m.idx)
	}
	if m.content != "body-newest" {
		t.Errorf("content = %q, want body-newest", m.content)
	}
}

func TestLeftArrow_StepsBack(t *testing.T) {
	m := NewModel("a.txt", newFake())
	m, _ = m.Update(keyType(tea.KeyLeft))
	if m.idx != 1 {
		t.Errorf("idx after left = %d, want 1", m.idx)
	}
	if m.content != "body-middle" {
		t.Errorf("content = %q, want body-middle", m.content)
	}
}

func TestLeftArrow_StopsAtOldest(t *testing.T) {
	m := NewModel("a.txt", newFake())
	for range 10 {
		m, _ = m.Update(keyType(tea.KeyLeft))
	}
	if m.idx != 2 {
		t.Errorf("idx = %d, want 2 (oldest)", m.idx)
	}
	if m.content != "body-oldest" {
		t.Errorf("content = %q, want body-oldest", m.content)
	}
}

func TestRightArrow_StepsForward(t *testing.T) {
	m := NewModel("a.txt", newFake())
	m, _ = m.Update(keyType(tea.KeyLeft))
	m, _ = m.Update(keyType(tea.KeyLeft))
	if m.idx != 2 {
		t.Fatalf("setup: idx = %d, want 2", m.idx)
	}
	m, _ = m.Update(keyType(tea.KeyRight))
	if m.idx != 1 {
		t.Errorf("idx after right = %d, want 1", m.idx)
	}
	if m.content != "body-middle" {
		t.Errorf("content = %q, want body-middle", m.content)
	}
}

func TestRightArrow_StopsAtNewest(t *testing.T) {
	m := NewModel("a.txt", newFake())
	m, _ = m.Update(keyType(tea.KeyRight))
	if m.idx != 0 {
		t.Errorf("idx = %d, want 0", m.idx)
	}
	if m.content != "body-newest" {
		t.Errorf("content = %q, want body-newest", m.content)
	}
}

func TestRoundTrip_LeftThenRightReturns(t *testing.T) {
	m := NewModel("a.txt", newFake())
	m, _ = m.Update(keyType(tea.KeyLeft))
	m, _ = m.Update(keyType(tea.KeyLeft))
	m, _ = m.Update(keyType(tea.KeyRight))
	m, _ = m.Update(keyType(tea.KeyRight))
	if m.idx != 0 {
		t.Errorf("idx after round trip = %d, want 0", m.idx)
	}
	if m.content != "body-newest" {
		t.Errorf("content = %q, want body-newest", m.content)
	}
}

func assertQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestQuitKey_Q(t *testing.T) {
	m := NewModel("a.txt", newFake())
	_, cmd := m.Update(keyRune('q'))
	assertQuit(t, cmd)
}

func TestQuitKey_CtrlC(t *testing.T) {
	m := NewModel("a.txt", newFake())
	_, cmd := m.Update(keyType(tea.KeyCtrlC))
	assertQuit(t, cmd)
}

func TestView_ContainsStatusFields(t *testing.T) {
	m := withSize(NewModel("sub/a.txt", newFake()))
	view := m.View()
	for _, want := range []string{"body-newest", "sub/a.txt", "hhhhhhh", "1/3", "current", "newest-msg"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestView_PositionUpdatesAsWeWalkBack(t *testing.T) {
	m := withSize(NewModel("a.txt", newFake()))
	m, _ = m.Update(keyType(tea.KeyLeft))
	view := m.View()
	for _, want := range []string{"body-middle", "ggggggg", "2/3", "1 back", "middle-msg"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestView_NoCommits(t *testing.T) {
	m := NewModel("a.txt", &fakeSource{})
	if !strings.Contains(m.View(), "no history") {
		t.Errorf("expected 'no history' in view:\n%s", m.View())
	}
}

func TestView_ContentError(t *testing.T) {
	fake := newFake()
	fake.err = fmt.Errorf("boom")
	m := NewModel("a.txt", fake)
	if !strings.Contains(m.View(), "boom") {
		t.Errorf("expected 'boom' in view:\n%s", m.View())
	}
}

func TestUnknownKey_NoOp(t *testing.T) {
	m := NewModel("a.txt", newFake())
	before := m.idx
	m, cmd := m.Update(keyRune('x'))
	if m.idx != before {
		t.Errorf("idx changed on unknown key: %d -> %d", before, m.idx)
	}
	if cmd != nil {
		t.Errorf("unknown key produced cmd: %v", cmd)
	}
}
