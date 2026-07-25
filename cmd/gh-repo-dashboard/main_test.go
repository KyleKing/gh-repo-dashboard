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
