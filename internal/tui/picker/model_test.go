package picker

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// testFiles mimics the shape of a real `git ls-files` output,
// already sorted, with a mix of root files and nested dirs.
var testFiles = []string{
	"README.md",
	"cmd/git-pager/main.go",
	"internal/git/history.go",
	"internal/git/history_test.go",
	"internal/tui/pager/model.go",
	"internal/tui/picker/model.go",
}

func keyType(kt tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: kt} }
func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestListEntries_Root(t *testing.T) {
	got := listEntries(testFiles, "")
	want := []entry{
		{name: "cmd", isDir: true},
		{name: "internal", isDir: true},
		{name: "README.md", isDir: false},
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestListEntries_NestedDirHasTwoSubdirs(t *testing.T) {
	got := listEntries(testFiles, "internal/tui")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].name != "pager" || !got[0].isDir {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].name != "picker" || !got[1].isDir {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestListEntries_DirWithOnlyFiles(t *testing.T) {
	got := listEntries(testFiles, "internal/git")
	if len(got) != 2 {
		t.Fatalf("len = %d: %+v", len(got), got)
	}
	for _, e := range got {
		if e.isDir {
			t.Errorf("expected all files, got dir: %+v", e)
		}
	}
	if got[0].name != "history.go" || got[1].name != "history_test.go" {
		t.Errorf("order wrong: %+v", got)
	}
}

func TestListEntries_DedupesDirs(t *testing.T) {
	files := []string{"a/one.go", "a/two.go", "a/three.go"}
	got := listEntries(files, "")
	if len(got) != 1 || got[0].name != "a" || !got[0].isDir {
		t.Errorf("got %+v, want single dir 'a'", got)
	}
}

func TestPicker_StartsAtRoot(t *testing.T) {
	m := NewModel("repo", testFiles)
	if m.dir != "" {
		t.Errorf("dir = %q, want empty", m.dir)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestPicker_DownMovesCursor(t *testing.T) {
	m := NewModel("repo", testFiles)
	m, _ = m.Update(keyType(tea.KeyDown))
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

func TestPicker_DownStopsAtLast(t *testing.T) {
	m := NewModel("repo", testFiles)
	for range 100 {
		m, _ = m.Update(keyType(tea.KeyDown))
	}
	if m.cursor != len(m.entries)-1 {
		t.Errorf("cursor = %d, want %d", m.cursor, len(m.entries)-1)
	}
}

func TestPicker_UpStopsAtFirst(t *testing.T) {
	m := NewModel("repo", testFiles)
	m, _ = m.Update(keyType(tea.KeyUp))
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestPicker_EnterDescendsIntoDir(t *testing.T) {
	m := NewModel("repo", testFiles)
	// cursor defaults to 0, which is "cmd/" at root.
	m, cmd := m.Update(keyType(tea.KeyEnter))
	if cmd != nil {
		t.Errorf("descending should not emit cmd: %v", cmd)
	}
	if m.dir != "cmd" {
		t.Errorf("dir = %q, want cmd", m.dir)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 after descent", m.cursor)
	}
}

func TestPicker_EnterOnFileEmitsSelected(t *testing.T) {
	m := NewModel("repo", testFiles)
	// Walk cursor to README.md (entries at root: cmd, internal, README.md).
	m, _ = m.Update(keyType(tea.KeyDown))
	m, _ = m.Update(keyType(tea.KeyDown))
	_, cmd := m.Update(keyType(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected SelectedMsg cmd, got nil")
	}
	msg := cmd()
	sel, ok := msg.(SelectedMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want SelectedMsg", msg)
	}
	if sel.Path != "README.md" {
		t.Errorf("Path = %q, want README.md", sel.Path)
	}
}

func TestPicker_DescendThenSelectNestedFile(t *testing.T) {
	m := NewModel("repo", testFiles)
	m, _ = m.Update(keyType(tea.KeyEnter)) // into cmd
	m, _ = m.Update(keyType(tea.KeyEnter)) // into cmd/git-pager
	_, cmd := m.Update(keyType(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("expected selection cmd")
	}
	sel, ok := cmd().(SelectedMsg)
	if !ok {
		t.Fatalf("want SelectedMsg, got %T", cmd())
	}
	if sel.Path != "cmd/git-pager/main.go" {
		t.Errorf("Path = %q", sel.Path)
	}
}

func TestPicker_LeftAscends(t *testing.T) {
	m := NewModel("repo", testFiles)
	m, _ = m.Update(keyType(tea.KeyEnter)) // into cmd
	if m.dir != "cmd" {
		t.Fatalf("setup: dir = %q", m.dir)
	}
	m, _ = m.Update(keyType(tea.KeyLeft))
	if m.dir != "" {
		t.Errorf("dir = %q, want empty", m.dir)
	}
}

func TestPicker_LeftAscendsOneLevelOnly(t *testing.T) {
	m := NewModel("repo", testFiles)
	m, _ = m.Update(keyType(tea.KeyDown))  // cursor on internal
	m, _ = m.Update(keyType(tea.KeyEnter)) // into internal
	m, _ = m.Update(keyType(tea.KeyEnter)) // into internal/git (first entry)
	if m.dir != "internal/git" {
		t.Fatalf("setup: dir = %q", m.dir)
	}
	m, _ = m.Update(keyType(tea.KeyLeft))
	if m.dir != "internal" {
		t.Errorf("dir = %q, want internal", m.dir)
	}
}

func TestPicker_LeftAtRootIsNoop(t *testing.T) {
	m := NewModel("repo", testFiles)
	m, _ = m.Update(keyType(tea.KeyLeft))
	if m.dir != "" {
		t.Errorf("dir = %q, want empty", m.dir)
	}
}

func TestPicker_Quit(t *testing.T) {
	m := NewModel("repo", testFiles)
	_, cmd := m.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("got %T, want QuitMsg", cmd())
	}
}

func TestPicker_View_ShowsLabelAndEntries(t *testing.T) {
	m := NewModel("myrepo", testFiles)
	v := m.View()
	for _, want := range []string{"myrepo", "cmd/", "internal/", "README.md", "> "} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestPicker_View_ShowsSubdirPath(t *testing.T) {
	m := NewModel("myrepo", testFiles)
	m, _ = m.Update(keyType(tea.KeyEnter)) // into cmd
	v := m.View()
	if !strings.Contains(v, "myrepo/cmd") {
		t.Errorf("view missing 'myrepo/cmd':\n%s", v)
	}
}

func TestPicker_View_EmptyDir(t *testing.T) {
	m := NewModel("repo", nil)
	v := m.View()
	if !strings.Contains(v, "(empty)") {
		t.Errorf("view missing (empty):\n%s", v)
	}
}
