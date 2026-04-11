package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Commit is the minimal view of a git commit that git-pager needs.
type Commit struct {
	Hash      string
	ShortHash string
	Subject   string
}

// Source is a file's git history plus a way to fetch that file's
// contents at any commit in that history. It is the concrete type
// wired into the TUI; the TUI consumes it through a narrower
// interface so tests can use a fake.
type Source struct {
	repoDir string
	relPath string
	commits []Commit
}

// NewSource resolves userPath to its enclosing git repository, loads
// the history of commits that touched the file, and returns a Source
// ready for paging. userPath may be relative, absolute, or inside a
// subdirectory of the repo.
func NewSource(userPath string) (*Source, error) {
	abs, err := filepath.Abs(userPath)
	if err != nil {
		return nil, err
	}
	// Resolve symlinks so filepath.Rel lines up with what git reports
	// as the repo toplevel (matters on macOS where t.TempDir is under
	// /var -> /private/var, but also for any symlinked checkout).
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	root, err := revParseToplevel(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return nil, err
	}
	commits, err := History(root, rel)
	if err != nil {
		return nil, err
	}
	return &Source{repoDir: root, relPath: rel, commits: commits}, nil
}

// Commits returns the file's history, newest first.
func (s *Source) Commits() []Commit { return s.commits }

// DisplayPath is the repo-relative path suitable for the status bar.
func (s *Source) DisplayPath() string { return s.relPath }

// Content returns the file's contents at the given commit hash.
func (s *Source) Content(hash string) (string, error) {
	return FileAt(s.repoDir, hash, s.relPath)
}

// Diff returns the unified diff of the file introduced by the given
// commit (i.e. parent-vs-commit for this path). Root commits show the
// file as fully added.
func (s *Source) Diff(hash string) (string, error) {
	return DiffAt(s.repoDir, hash, s.relPath)
}

func revParseToplevel(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", dir)
	}
	root := strings.TrimRight(string(out), "\n")
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return root, nil
}

// History returns commits that touched relPath within repoDir, newest
// first. Deletions are filtered out so every returned commit is one
// where `git show HASH:relPath` will succeed.
func History(repoDir, relPath string) ([]Commit, error) {
	cmd := exec.Command("git", "-C", repoDir, "log",
		"--diff-filter=AMCR",
		"--format=%H%x09%h%x09%s",
		"--", relPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", relPath, err)
	}
	var commits []Commit
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, Commit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Subject:   parts[2],
		})
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no history for %s", relPath)
	}
	return commits, nil
}

// FileAt returns the contents of relPath at the given commit.
func FileAt(repoDir, hash, relPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "show", hash+":"+relPath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git show %s:%s: %w", hash, relPath, err)
	}
	return string(out), nil
}

// DiffAt returns the unified diff for relPath introduced by the given
// commit. An empty format string suppresses the commit header so the
// output is just the diff body.
func DiffAt(repoDir, hash, relPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoDir, "show",
		"--format=", "--no-color", hash, "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git show --format= %s -- %s: %w", hash, relPath, err)
	}
	// `--format=` still emits a leading newline before the diff body.
	return strings.TrimLeft(string(out), "\n"), nil
}

// List returns the paths of all files tracked in the repo at
// repoDir, each relative to the repo root. The list is the raw
// output of `git ls-files`, which is already sorted and excludes
// anything ignored or untracked.
func List(repoDir string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoDir, "ls-files")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// DiscoverRepo resolves dir (relative or absolute, possibly inside a
// subdirectory of a repo) to the repo's toplevel. The returned path
// has its symlinks resolved so filepath.Rel lines up with what git
// reports internally.
func DiscoverRepo(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return revParseToplevel(abs)
}
