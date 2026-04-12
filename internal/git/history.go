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
	// Path is the repo-relative path the file had at this commit. It
	// differs from the current path when the file has been renamed.
	Path string
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

// pathFor returns the repo-relative path the file had at the given
// commit hash. When the file has been renamed, older commits carry the
// old name; if the hash is not found the current path is returned as a
// safe fallback.
func (s *Source) pathFor(hash string) string {
	for _, c := range s.commits {
		if c.Hash == hash {
			return c.Path
		}
	}
	return s.relPath
}

// Content returns the file's contents at the given commit hash.
func (s *Source) Content(hash string) (string, error) {
	return FileAt(s.repoDir, hash, s.pathFor(hash))
}

// Diff returns the unified diff of the file introduced by the given
// commit (i.e. parent-vs-commit for this path). Root commits show the
// file as fully added.
func (s *Source) Diff(hash string) (string, error) {
	return DiffAt(s.repoDir, hash, s.pathFor(hash))
}

func revParseToplevel(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel") // #nosec G204 -- dir is user input but we're not invoking a shell, so the risk is essentially nonexistent.
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
// first, following renames. Deletions are filtered out so every
// returned commit is one where `git show HASH:relPath` will succeed.
// Each Commit.Path records the repo-relative path the file had at that
// commit, which may differ from relPath when the file was renamed.
func History(repoDir, relPath string) ([]Commit, error) {
	// %x00 acts as a record separator between commits so we can cleanly
	// split the commit-info line from the --name-status lines that follow.
	cmd := exec.Command("git", "-C", repoDir, "log", "--follow", // #nosec G204 -- relPath is the result of a filepath.Rel call and we are not invoking a shell, so the risk is essentially nonexistent.
		"--diff-filter=AMCR", "--format=%x00%H%x09%h%x09%s", "--name-status", "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", relPath, err)
	}
	var commits []Commit
	for record := range strings.SplitSeq(string(out), "\x00") {
		record = strings.TrimLeft(record, "\n")
		if record == "" {
			continue
		}
		header, rest, _ := strings.Cut(record, "\n")
		parts := strings.SplitN(header, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		c := Commit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Subject:   parts[2],
			Path:      parseNameStatusPath(rest),
		}
		if c.Path == "" {
			c.Path = relPath
		}
		commits = append(commits, c)
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no history for %s", relPath)
	}
	return commits, nil
}

// parseNameStatusPath extracts the file path from the --name-status
// block that follows a commit in git log output. For plain adds and
// modifications ("A\tpath", "M\tpath") it returns the single path; for
// renames and copies ("R090\told\tnew", "C090\told\tnew") it returns
// the new (destination) path, which is what exists in that commit's
// tree.
func parseNameStatusPath(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		status := fields[0]
		if (strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")) && len(fields) >= 3 {
			return fields[2]
		}
		if len(fields) >= 2 {
			return fields[1]
		}
	}
	return ""
}

// FileAt returns the contents of relPath at the given commit.
func FileAt(repoDir, hash, relPath string) (string, error) {
	// nosec explanation: relPath is the result of a filepath.Rel call
	// and we are not invoking a shell here, so the risk is essentially
	// nonexistent.
	cmd := exec.Command("git", "-C", repoDir, "show", hash+":"+relPath) // #nosec G204
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
	cmd := exec.Command("git", "-C", repoDir, "show", "--format=", "--no-color", hash, "--", relPath) // #nosec G204 -- relPath is the result of a filepath.Rel call and we are not invoking a shell, so the risk is essentially nonexistent.
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
	cmd := exec.Command("git", "-C", repoDir, "ls-files") // #nosec G204 -- repoDir is user input but we're not invoking a shell, so the risk is essentially nonexistent.
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
