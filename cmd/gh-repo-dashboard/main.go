// Package main implements gh-repo-dashboard: K9s-inspired Bubble Tea TUI for managing multiple git and jj repositories
package main

import (
	"bytes"
	"context"
	"errors"
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
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

var (
	errEmptyRoster  = errors.New("names no repos that exist on disk")
	errNotDirectory = errors.New("not a directory")
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
	vcs.SetExternalDiffCommand(cfg.Diff.External)
	models.SetPRViews(prViews(cfg))

	if ttl := cfg.CacheTTL(); ttl > 0 {
		cache.SetAllTTLs(ttl)
	}

	if cfg.PersistCache() {
		if store, err := cache.UserDiskCache(); err == nil {
			cache.SetDiskCache(store)
		}
	}
}

// prViews converts the config file's saved searches into the model's own type,
// which the app package reads without knowing where they were written.
func prViews(cfg config.Config) []models.PRView {
	views := make([]models.PRView, 0, len(cfg.PRViews))
	for _, view := range cfg.PRViews {
		views = append(views, models.PRView{Name: view.Name, Search: view.Search})
	}

	return views
}

func printUsage() {
	configPath, err := config.Path()
	if err != nil {
		configPath = "$XDG_CONFIG_HOME/gh-repo-dashboard/config.toml"
	}

	fmt.Fprintf(os.Stderr, `Usage: %s [flags] [paths...]

Positional paths are the directories to scan for repos. They take precedence
over the config file's scan_paths, which takes precedence over the enclosing
repo (walking up from the current directory) or the current directory itself.

Optional config file (scan_paths, depth, cache_ttl_minutes, notes_filenames,
cache_to_disk, diff.external, pr_views):
  %s

Run "%s --script -" with a "help" line to list the :commands.

Flags:
`, os.Args[0], configPath, os.Args[0])
	flag.PrintDefaults()
}

// resolveRoster picks the repo roster: a mani.yaml when one is named, and the
// usual scan-path resolution otherwise. Roster entries are repo directories,
// which discovery returns as-is at any depth.
func resolveRoster(maniPath string, args []string, cfg config.Config) ([]string, error) {
	if maniPath == "" {
		return resolveScanPaths(args, cfg)
	}

	roster, err := discovery.ManiPaths(maniPath)
	if err != nil {
		return nil, fmt.Errorf("reading the roster: %w", err)
	}
	if len(roster) == 0 {
		return nil, fmt.Errorf("%s: %w", maniPath, errEmptyRoster)
	}

	return roster, nil
}

// checkScanPaths rejects a scan root that is not an existing directory. Only
// the explicitly named roots are checked; the cwd and enclosing-repo fallbacks
// exist by construction.
func checkScanPaths(source string, paths []string) error {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("%s %s: %w", source, p, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s %s: %w", source, p, errNotDirectory)
		}
	}

	return nil
}

// resolveScanPaths picks the scan roots: positional args, then config, then the
// enclosing repo, then the current directory.
func resolveScanPaths(args []string, cfg config.Config) ([]string, error) {
	if len(args) > 0 {
		if err := checkScanPaths("scan path", args); err != nil {
			return nil, err
		}

		return args, nil
	}
	if len(cfg.ScanPaths) > 0 {
		if err := checkScanPaths("config scan_paths entry", cfg.ScanPaths); err != nil {
			return nil, err
		}

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
	fresh := flag.Bool("fresh", false,
		"With -cli, fetch fresh GitHub PR and CI data instead of relying on the cache")
	fetch := flag.Bool("fetch", false,
		"With -cli, git fetch each repo first so ahead/behind compares against the remote")
	filterExpr := flag.String("filter", "",
		"With -cli, narrow output by a predicate expression (e.g. 'dirty and has_notes')")
	maniPath := flag.String("mani", "",
		"Read the repo roster from a mani.yaml instead of scanning directories")
	scriptPath := flag.String("script", "",
		"Run :command lines from the given file (or - for stdin) instead of the TUI")
	flag.Parse()

	if *showVersion {
		fmt.Printf("gh-repo-dashboard %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if err := preflight(os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	applyConfig(cfg, depth)

	scanPaths, err := resolveRoster(*maniPath, flag.Args(), cfg)
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
		opts := cli.Options{MaxDepth: *depth, Fresh: *fresh, Fetch: *fetch, Predicate: *filterExpr}
		if err := cli.Run(context.Background(), os.Stdout, absPathList, opts); err != nil {
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
