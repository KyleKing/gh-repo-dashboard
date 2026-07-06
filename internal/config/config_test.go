package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh-repo-dashboard", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestLoadFullConfig(t *testing.T) {
	dir := writeConfig(t, `
scan_paths = ["~/Developer", "/tmp/repos"]
depth = 3
notes_filenames = ["NOTES.md"]
cache_ttl_minutes = 10
`)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", "/home/tester")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	wantFirst := "/home/tester/Developer"
	if len(cfg.ScanPaths) != 2 || cfg.ScanPaths[0] != wantFirst {
		t.Errorf("scan_paths = %v; want first %q", cfg.ScanPaths, wantFirst)
	}
	if cfg.Depth != 3 {
		t.Errorf("depth = %d; want 3", cfg.Depth)
	}
	if len(cfg.NotesFilenames) != 1 || cfg.NotesFilenames[0] != "NOTES.md" {
		t.Errorf("notes_filenames = %v", cfg.NotesFilenames)
	}
	if cfg.CacheTTL() != 10*time.Minute {
		t.Errorf("cache ttl = %v; want 10m", cfg.CacheTTL())
	}
}

func TestLoadMissingFileIsZeroConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ScanPaths) != 0 || cfg.Depth != 0 || cfg.CacheTTL() != 0 {
		t.Errorf("expected zero config, got %+v", cfg)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"malformed toml", "scan_paths = [unclosed"},
		{"negative depth", "depth = -1"},
		{"negative ttl", "cache_ttl_minutes = -5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeConfig(t, tt.content)
			t.Setenv("XDG_CONFIG_HOME", dir)

			if _, err := config.Load(); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")

	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	want := "/custom/xdg/gh-repo-dashboard/config.toml"
	if path != want {
		t.Errorf("path = %q; want %q", path, want)
	}
}

func TestPathFallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/tester")

	path, err := config.Path()
	if err != nil && errors.Is(err, os.ErrNotExist) {
		t.Skip("no home dir in environment")
	}
	if err != nil {
		t.Fatal(err)
	}
	want := "/home/tester/.config/gh-repo-dashboard/config.toml"
	if path != want {
		t.Errorf("path = %q; want %q", path, want)
	}
}
