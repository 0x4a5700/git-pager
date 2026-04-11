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
	diffs    map[string]string
	err      error
	diffErr  error
}

func (f *fakeSource) Commits() []git.Commit { return f.commits }
func (f *fakeSource) Content(hash string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.contents[hash], nil
}
func (f *fakeSource) Diff(hash string) (string, error) {
	if f.diffErr != nil {
		return "", f.diffErr
	}
	return f.diffs[hash], nil
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
		diffs: map[string]string{
			"hhhhhhhhhh": "diff-newest",
			"gggggggggg": "diff-middle",
			"ffffffffff": "diff-oldest",
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
	if m.file.content != "body-newest" {
		t.Errorf("content = %q, want body-newest", m.file.content)
	}
}

func TestLeftArrow_StepsBack(t *testing.T) {
	m := NewModel("a.txt", newFake())
	m, _ = m.Update(keyType(tea.KeyLeft))
	if m.idx != 1 {
		t.Errorf("idx after left = %d, want 1", m.idx)
	}
	if m.file.content != "body-middle" {
		t.Errorf("content = %q, want body-middle", m.file.content)
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
	if m.file.content != "body-oldest" {
		t.Errorf("content = %q, want body-oldest", m.file.content)
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
	if m.file.content != "body-middle" {
		t.Errorf("content = %q, want body-middle", m.file.content)
	}
}

func TestRightArrow_StopsAtNewest(t *testing.T) {
	m := NewModel("a.txt", newFake())
	m, _ = m.Update(keyType(tea.KeyRight))
	if m.idx != 0 {
		t.Errorf("idx = %d, want 0", m.idx)
	}
	if m.file.content != "body-newest" {
		t.Errorf("content = %q, want body-newest", m.file.content)
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
	if m.file.content != "body-newest" {
		t.Errorf("content = %q, want body-newest", m.file.content)
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

func TestDiffMode_PagingBetweenCommitsDoesNotDivideByZero(t *testing.T) {
	// Regression: in diff mode, pressing left/right reset the diff
	// pane to empty without reloading, so View() computed scroll
	// percent as 100*(0+ch)/0 and panicked.
	m := withSize(NewModel("a.txt", newFake()))
	m, _ = m.Update(keyRune('d')) // enter diff mode
	m, _ = m.Update(keyType(tea.KeyLeft))
	view := m.View()
	if !strings.Contains(view, "diff-middle") {
		t.Errorf("view missing %q after paging in diff mode:\n%s", "diff-middle", view)
	}
	m, _ = m.Update(keyType(tea.KeyRight))
	view = m.View()
	if !strings.Contains(view, "diff-newest") {
		t.Errorf("view missing %q after paging back:\n%s", "diff-newest", view)
	}
}

func TestParseHunkHeader(t *testing.T) {
	cases := []struct {
		in                 string
		wantOld, wantNew   int
		wantOk             bool
	}{
		{"@@ -10,7 +10,8 @@", 10, 10, true},
		{"@@ -10 +10 @@", 10, 10, true},
		{"@@ -1,0 +1,5 @@ func bar()", 1, 1, true},
		{"@@ -0,0 +1,3 @@", 0, 1, true},
		{"not a header", 0, 0, false},
		{"@@ garbage", 0, 0, false},
		{"@@ -a,b +c,d @@", 0, 0, false},
	}
	for _, c := range cases {
		o, n, ok := parseHunkHeader(c.in)
		if o != c.wantOld || n != c.wantNew || ok != c.wantOk {
			t.Errorf("%q: got (%d,%d,%v), want (%d,%d,%v)",
				c.in, o, n, ok, c.wantOld, c.wantNew, c.wantOk)
		}
	}
}

func TestDiffLineNumbers(t *testing.T) {
	lines := strings.Split(
		"diff --git a/a.txt b/a.txt\n"+
			"--- a/a.txt\n"+
			"+++ b/a.txt\n"+
			"@@ -10,4 +10,5 @@\n"+
			" ctx-a\n"+
			"-del-a\n"+
			"+add-a\n"+
			"+add-b\n"+
			" ctx-b\n", "\n")
	oldNums, newNums := diffLineNumbers(lines)
	want := []struct{ o, n int }{
		{0, 0},   // diff --git
		{0, 0},   // ---
		{0, 0},   // +++
		{0, 0},   // @@ (header itself has no line number)
		{10, 10}, // " ctx-a"
		{11, 0},  // "-del-a"
		{0, 11},  // "+add-a"
		{0, 12},  // "+add-b"
		{12, 13}, // " ctx-b"
		{0, 0},   // trailing empty from split
	}
	if len(oldNums) != len(want) || len(newNums) != len(want) {
		t.Fatalf("len(old)=%d len(new)=%d want %d", len(oldNums), len(newNums), len(want))
	}
	for i, w := range want {
		if oldNums[i] != w.o || newNums[i] != w.n {
			t.Errorf("line %d %q: got (%d,%d), want (%d,%d)",
				i, lines[i], oldNums[i], newNums[i], w.o, w.n)
		}
	}
}

// stripAnsi removes ANSI SGR escape sequences so view output can be
// matched against literal strings in assertions.
func stripAnsi(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestDiffMode_RendersTwoSidedLineNumbers(t *testing.T) {
	fake := newFake()
	fake.diffs["hhhhhhhhhh"] = "diff --git a/a.txt b/a.txt\n" +
		"--- a/a.txt\n" +
		"+++ b/a.txt\n" +
		"@@ -10,3 +10,3 @@\n" +
		" ctx-a\n" +
		"-del-a\n" +
		"+add-a\n" +
		" ctx-b\n"
	m := withSize(NewModel("a.txt", fake))
	m, _ = m.Update(keyRune('d'))
	view := stripAnsi(m.View())

	// Context line " ctx-a": old=10, new=10. Gutter is "10 10 ", then
	// the raw content begins with a leading space.
	if !strings.Contains(view, "10 10  ctx-a") {
		t.Errorf("context gutter missing:\n%s", view)
	}
	// Deleted line: old=11, new blank. Gutter is "11    " (11 + sep +
	// 2 blanks + trailing space), content starts with '-'.
	if !strings.Contains(view, "11    -del-a") {
		t.Errorf("delete gutter missing:\n%s", view)
	}
	// Added line: old blank, new=11. Gutter is "   11 " (2 blanks +
	// sep + 11 + trailing space), content starts with '+'.
	if !strings.Contains(view, "   11 +add-a") {
		t.Errorf("add gutter missing:\n%s", view)
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
