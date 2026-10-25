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

[diff]
external = "difft --color=always"

[[tasks]]
name = "gc"
run = "git gc"
where = "git_size > 1G and last_run(gc) > 30d"
concurrency = 1
confirm = true
every = "30d"
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
	if !cfg.PersistCache() {
		t.Error("a file that says nothing about cache_to_disk turned persistence off")
	}
	if want := "difft --color=always"; cfg.Diff.External != want {
		t.Errorf("diff.external = %q; want %q", cfg.Diff.External, want)
	}
	assertGCTask(t, cfg.Tasks)
}

func assertGCTask(t *testing.T, got []config.Task) {
	t.Helper()
	if len(got) != 1 || got[0].Name != "gc" || got[0].Concurrency != 1 || !got[0].Confirm {
		t.Fatalf("tasks = %+v", got)
	}
	if interval, err := got[0].Interval(); err != nil || interval != 30*24*time.Hour {
		t.Errorf("interval = %v, %v; want 30 days", interval, err)
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
	if !cfg.PersistCache() {
		t.Error("persistence is an opt-out, so a missing file leaves it on")
	}
}

// Writing the key is the only way to turn the cache files off, so false has to
// survive the round trip that an absent key reads as true.
func TestCacheToDiskOptOut(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", writeConfig(t, "cache_to_disk = false\n"))

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PersistCache() {
		t.Error("cache_to_disk = false still persists")
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    error
	}{
		{"malformed toml", "scan_paths = [unclosed", nil},
		{"negative depth", "depth = -1", nil},
		{"negative ttl", "cache_ttl_minutes = -5", nil},
		{"task without name", "[[tasks]]\nrun = 'true'", config.ErrTaskNameMissing},
		{"task name with space", "[[tasks]]\nname = 'a b'\nrun = 'true'", config.ErrTaskNameInvalid},
		{
			"duplicate task", "[[tasks]]\nname = 'a'\nrun = 'true'\n[[tasks]]\nname = 'a'\nrun = 'true'",
			config.ErrTaskNameDuplicate,
		},
		{"task without run", "[[tasks]]\nname = 'a'", config.ErrTaskRunMissing},
		{
			"negative concurrency", "[[tasks]]\nname = 'a'\nrun = 'true'\nconcurrency = -1",
			config.ErrTaskNegativeWorkers,
		},
		{"bad every", "[[tasks]]\nname = 'a'\nrun = 'true'\nevery = 'soon'", config.ErrTaskEveryInvalid},
		{"bad where", "[[tasks]]\nname = 'a'\nrun = 'true'\nwhere = 'git_size >'", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeConfig(t, tt.content)
			t.Setenv("XDG_CONFIG_HOME", dir)

			_, err := config.Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("error = %v; want %v", err, tt.want)
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
