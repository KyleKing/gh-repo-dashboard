package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/app"
)

func findGitRoot(startPath string) (string, bool) {
	current := startPath
	for {
		gitDir := filepath.Join(current, ".git")
		jjDir := filepath.Join(current, ".jj")

		if _, err := os.Stat(gitDir); err == nil {
			return current, true
		}
		if _, err := os.Stat(jjDir); err == nil {
			return current, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return startPath, false
}

func main() {
	depth := flag.Int("depth", 1, "Maximum directory depth to scan")
	flag.Parse()

	scanPaths := flag.Args()
	if len(scanPaths) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		if repoRoot, found := findGitRoot(cwd); found {
			scanPaths = []string{repoRoot}
		} else {
			scanPaths = []string{cwd}
		}
	}

	absPathList := make([]string, 0, len(scanPaths))
	for _, p := range scanPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving path %s: %v\n", p, err)
			os.Exit(1)
		}
		absPathList = append(absPathList, absPath)
	}

	model := app.New(absPathList, *depth)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
