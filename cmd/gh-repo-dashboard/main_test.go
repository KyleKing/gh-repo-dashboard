package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
)

func TestFindGitRoot(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		marker string
	}{
		{"git repo", ".git"},
		{"jj repo", ".jj"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, tt.marker), 0o750); err != nil {
				t.Fatal(err)
			}
			nested := filepath.Join(root, "a", "b")
			if err := os.MkdirAll(nested, 0o750); err != nil {
				t.Fatal(err)
			}

			got, found := findGitRoot(nested)
			if !found {
				t.Fatalf("findGitRoot(%q) found = false; want true", nested)
			}
			if got != root {
				t.Errorf("findGitRoot(%q) = %q; want %q", nested, got, root)
			}
		})
	}

	t.Run("no repo above start path", func(t *testing.T) {
		t.Parallel()
		start := t.TempDir()

		got, found := findGitRoot(start)
		if found {
			t.Fatalf("findGitRoot(%q) found = true in %q; want false", start, got)
		}
		if got != start {
			t.Errorf("findGitRoot(%q) = %q; want start path back", start, got)
		}
	})
}

func TestResolveScanPathsRejectsUnusablePaths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "notadir")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "typo")

	tests := []struct {
		name    string
		args    []string
		cfg     config.Config
		wantErr bool
	}{
		{name: "existing positional directory", args: []string{dir}},
		{name: "missing positional directory", args: []string{missing}, wantErr: true},
		{name: "positional path is a file", args: []string{file}, wantErr: true},
		{name: "existing config scan path", cfg: config.Config{ScanPaths: []string{dir}}},
		{name: "missing config scan path", cfg: config.Config{ScanPaths: []string{missing}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			paths, err := resolveScanPaths(tt.args, tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveScanPaths(%v, %v) = %v, nil; want an error", tt.args, tt.cfg, paths)
				}

				return
			}
			if err != nil {
				t.Fatalf("resolveScanPaths(%v, %v) errored: %v", tt.args, tt.cfg, err)
			}
			if len(paths) != 1 || paths[0] != dir {
				t.Errorf("resolveScanPaths = %v; want [%s]", paths, dir)
			}
		})
	}
}

//nolint:paralleltest // applyConfig mutates global flag and notes-filename state
func TestApplyConfigDepth(t *testing.T) {
	tests := []struct {
		name     string
		cfgDepth int
		expected int
	}{
		{"config depth overrides default", 5, 5},
		{"zero config depth keeps default", 0, 1},
		{"negative config depth keeps default", -3, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depth := 1
			applyConfig(config.Config{Depth: tt.cfgDepth}, &depth)
			if depth != tt.expected {
				t.Errorf("applyConfig depth = %d; want %d", depth, tt.expected)
			}
		})
	}
}

//nolint:paralleltest // applyConfig mutates global flag and notes-filename state
func TestApplyConfigExplicitFlagWins(t *testing.T) {
	flag.Int("depth", 1, "test stand-in for main's depth flag")
	if err := flag.Set("depth", "2"); err != nil {
		t.Fatal(err)
	}

	depth := 2
	applyConfig(config.Config{Depth: 9}, &depth)
	if depth != 2 {
		t.Errorf("applyConfig depth = %d; want explicitly set flag value 2", depth)
	}
}
