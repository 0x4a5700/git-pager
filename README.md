[![Release](https://github.com/0x4a5700/git-pager/actions/workflows/release.yml/badge.svg)](https://github.com/0x4a5700/git-pager/actions/workflows/release.yml)

# git-pager

A terminal UI for browsing the git history of files in a repository. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Usage

```
git-pager [--version] [repo-dir]
```

Run from anywhere inside a git repository, or pass a path to a repo. Defaults to the current directory.

| Flag | Description |
|------|-------------|
| `--version` | Print the version and exit |

## Navigation

### File picker

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `→` / `enter` / `l` | Enter directory or open file |
| `←` / `backspace` / `h` | Go up one directory |
| `q` | Quit |

### Pager

| Key | Action |
|-----|--------|
| `←` / `→` | Walk backwards / forwards through commit history |
| `↑` / `k` | Scroll up one line |
| `↓` / `j` | Scroll down one line |
| `pgup` | Scroll up one page |
| `pgdn` / `space` | Scroll down one page |
| `g` / `home` | Jump to top |
| `G` / `end` | Jump to bottom |
| `f` | File mode — plain file content |
| `d` | Diff mode — unified diff against parent commit |
| `i` | Inline mode — file content with newly added lines highlighted |
| `b` | Blame mode — per-line commit hash and date |
| `esc` | Return to file picker |
| `q` | Quit |

## Building

Requires Go 1.25+.

```
go build ./cmd/git-pager
```

## Running tests

```
go test ./...
```
