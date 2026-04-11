package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitRun shells out to git with a deterministic environment so the
// tests don't pick up user config (signing hooks, custom log formats,
// etc.) that might pollute parseable output.
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	gitRun(t, dir, "init", "-q", "-b", "main")
	return dir
}

func commit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", name)
	gitRun(t, dir, "commit", "-q", "-m", msg)
}

func TestHistory_NewestFirst(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "a.txt", "v1", "first")
	commit(t, dir, "a.txt", "v2", "second")
	commit(t, dir, "a.txt", "v3", "third")

	commits, err := History(dir, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 3 {
		t.Fatalf("len(commits) = %d, want 3", len(commits))
	}
	if got, want := commits[0].Subject, "third"; got != want {
		t.Errorf("newest subject = %q, want %q", got, want)
	}
	if got, want := commits[2].Subject, "first"; got != want {
		t.Errorf("oldest subject = %q, want %q", got, want)
	}
	for i, c := range commits {
		if len(c.Hash) != 40 {
			t.Errorf("commit[%d] hash = %q, want 40 chars", i, c.Hash)
		}
		if len(c.ShortHash) < 4 {
			t.Errorf("commit[%d] short = %q, want at least 4 chars", i, c.ShortHash)
		}
	}
}

func TestHistory_IgnoresUnrelatedCommits(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "a.txt", "v1", "touch a")
	commit(t, dir, "b.txt", "v1", "touch b")
	commit(t, dir, "a.txt", "v2", "touch a again")

	commits, err := History(dir, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("len(commits) = %d, want 2", len(commits))
	}
}

func TestHistory_MissingFile(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "a.txt", "v1", "only a")

	if _, err := History(dir, "missing.txt"); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestFileAt_ReturnsVersionedContent(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "a.txt", "v1", "first")
	commit(t, dir, "a.txt", "v2", "second")

	commits, err := History(dir, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	newest, err := FileAt(dir, commits[0].Hash, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if newest != "v2" {
		t.Errorf("newest contents = %q, want %q", newest, "v2")
	}
	oldest, err := FileAt(dir, commits[1].Hash, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if oldest != "v1" {
		t.Errorf("oldest contents = %q, want %q", oldest, "v1")
	}
}

func TestNewSource_ResolvesFromSubdir(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "sub/a.txt", "v1", "first")
	commit(t, dir, "sub/a.txt", "v2", "second")

	src, err := NewSource(filepath.Join(dir, "sub", "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := src.DisplayPath(), filepath.Join("sub", "a.txt"); got != want {
		t.Errorf("DisplayPath = %q, want %q", got, want)
	}
	commits := src.Commits()
	if len(commits) != 2 {
		t.Fatalf("len(commits) = %d, want 2", len(commits))
	}
	content, err := src.Content(commits[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if content != "v2" {
		t.Errorf("newest content = %q, want %q", content, "v2")
	}
}

func TestNewSource_NotARepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSource(path); err == nil {
		t.Fatal("expected error for non-repo path, got nil")
	}
}

func TestList_ReturnsTrackedFilesSorted(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "b.txt", "x", "add b")
	commit(t, dir, "a.txt", "y", "add a")
	commit(t, dir, "sub/c.txt", "z", "add sub/c")

	files, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.txt", "b.txt", "sub/c.txt"}
	if len(files) != len(want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
	for i, w := range want {
		if files[i] != w {
			t.Errorf("files[%d] = %q, want %q", i, files[i], w)
		}
	}
}

func TestList_ExcludesUntracked(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "tracked.txt", "x", "add tracked")
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f == "untracked.txt" {
			t.Errorf("untracked file leaked: %v", files)
		}
	}
}

func TestDiscoverRepo_FromRoot(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "a.txt", "x", "add")
	root, err := DiscoverRepo(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestDiscoverRepo_FromSubdir(t *testing.T) {
	dir := initRepo(t)
	commit(t, dir, "sub/a.txt", "x", "add")
	root, err := DiscoverRepo(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestDiscoverRepo_NotARepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := DiscoverRepo(dir); err == nil {
		t.Fatal("expected error for non-repo dir")
	}
}
