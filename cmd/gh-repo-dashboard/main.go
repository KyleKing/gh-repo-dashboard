// Package main implements gh-repo-dashboard: K9s-inspired Bubble Tea TUI for managing multiple git and jj repositories
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
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

// runScript executes :command lines from path ("-" for stdin) headlessly.
func runScript(path string, scanPaths []string, depth int) error {
	var script io.Reader = os.Stdin
	if path != "-" {
		data, err := os.ReadFile(path) //nolint:gosec // user-supplied script path is the point
		if err != nil {
			return fmt.Errorf("reading script: %w", err)
		}
		script = bytes.NewReader(data)
	}

	if err := app.RunScript(context.Background(), os.Stdout, scanPaths, depth, script); err != nil {
		return fmt.Errorf("running script: %w", err)
	}

	return nil
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

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %s [flags] [paths...]

Positional paths are the directories to scan for repos. They take precedence
over the config file's scan_paths, which takes precedence over the enclosing
repo (walking up from the current directory) or the current directory itself.

Flags:
`, os.Args[0])
	flag.PrintDefaults()
}

// resolveScanPaths picks the scan roots: positional args, then config, then the
// enclosing repo, then the current directory.
func resolveScanPaths(args []string, cfg config.Config) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	if len(cfg.ScanPaths) > 0 {
		return cfg.ScanPaths, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current directory: %w", err)
	}

	if repoRoot, found := findGitRoot(cwd); found {
		return []string{repoRoot}, nil
	}

	return []string{cwd}, nil
}

func absolutePaths(paths []string) ([]string, error) {
	absPathList := make([]string, 0, len(paths))
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("resolving path %s: %w", p, err)
		}
		absPathList = append(absPathList, absPath)
	}

	return absPathList, nil
}

func main() {
	flag.Usage = printUsage

	showVersion := flag.Bool("version", false, "Show version information")
	depth := flag.Int("depth", 1, "Maximum directory depth to scan")
	cliMode := flag.Bool("cli", false, "Print repo summaries as JSON instead of the TUI (cached GitHub data only)")
	fresh := flag.Bool("fresh", false, "With -cli, fetch fresh GitHub PR data instead of relying on the cache")
	filterExpr := flag.String("filter", "",
		"With -cli, narrow output by a predicate expression (e.g. 'dirty and has_notes')")
	scriptPath := flag.String("script", "",
		"Run :command lines from the given file (or - for stdin) instead of the TUI")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gh-repo-dashboard %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if err := preflight(context.Background(), os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	applyConfig(cfg, depth)

	scanPaths, err := resolveScanPaths(flag.Args(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	absPathList, err := absolutePaths(scanPaths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *cliMode {
		if err := cli.Run(context.Background(), os.Stdout, absPathList, *depth, *fresh, *filterExpr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		return
	}

	if *scriptPath != "" {
		if err := runScript(*scriptPath, absPathList, *depth); err != nil {
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
