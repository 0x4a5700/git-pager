package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/0x4a5700/git-pager/internal/git"
	"github.com/0x4a5700/git-pager/internal/tui"
)

func main() {
	start := "."
	switch len(os.Args) {
	case 1:
	case 2:
		start = os.Args[1]
	default:
		fmt.Fprintln(os.Stderr, "usage: git-pager [repo-dir]")
		os.Exit(2)
	}

	repoDir, err := git.DiscoverRepo(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	files, err := git.List(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "error: repo has no tracked files")
		os.Exit(1)
	}

	p := tea.NewProgram(tui.NewModel(repoDir, files), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
