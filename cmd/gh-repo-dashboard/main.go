// Package main implements gh-repo-dashboard: K9s-inspired Bubble Tea TUI for managing multiple git and jj repositories
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/app"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/cli"
	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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

// applyConfig applies config-file values below flag precedence: an explicitly
// set flag wins, otherwise a non-zero config value replaces the default.
func applyConfig(cfg config.Config, depth *int) {
	depthFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "depth" {
			depthFlagSet = true
		}
	})

	if !depthFlagSet && cfg.Depth > 0 {
		*depth = cfg.Depth
	}

	models.SetNotesFilenames(cfg.NotesFilenames)

	if ttl := cfg.CacheTTL(); ttl > 0 {
		cache.SetAllTTLs(ttl)
	}
}

func main() {
	showVersion := flag.Bool("version", false, "Show version information")
	depth := flag.Int("depth", 1, "Maximum directory depth to scan")
	cliMode := flag.Bool("cli", false, "Print repo summaries as JSON instead of the TUI (cached GitHub data only)")
	fresh := flag.Bool("fresh", false, "With -cli, fetch fresh GitHub PR data instead of relying on the cache")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gh-repo-dashboard %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	applyConfig(cfg, depth)

	scanPaths := flag.Args()
	if len(scanPaths) == 0 {
		scanPaths = cfg.ScanPaths
	}
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

	if *cliMode {
		if err := cli.Run(context.Background(), os.Stdout, absPathList, *depth, *fresh); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		return
	}

	model := app.New(absPathList, *depth)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
