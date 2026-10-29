package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
cache_ttl = "10m"
fleet_active = "90d"
fleet_authors = ["me@example.com"]

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
	if cfg.TTL() != 10*time.Minute {
		t.Errorf("cache ttl = %v; want 10m", cfg.TTL())
	}
	if !cfg.PersistCache() {
		t.Error("a file that says nothing about cache_to_disk turned persistence off")
	}
	if want := "difft --color=always"; cfg.Diff.External != want {
		t.Errorf("diff.external = %q; want %q", cfg.Diff.External, want)
	}
	if cfg.FleetWindow() != 90*24*time.Hour || len(cfg.FleetAuthors) != 1 {
		t.Errorf("fleet = %v %v; want 90d and one author", cfg.FleetWindow(), cfg.FleetAuthors)
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
	if len(cfg.ScanPaths) != 0 || cfg.Depth != 0 || cfg.CacheTTL != "" {
		t.Errorf("expected zero config, got %+v", cfg)
	}
	if cfg.TTL() != config.DefaultCacheTTL {
		t.Errorf("cache ttl = %v; want the %v default", cfg.TTL(), config.DefaultCacheTTL)
	}
	if !cfg.PersistCache() {
		t.Error("persistence is an opt-out, so a missing file leaves it on")
	}
	if cfg.FleetWindow() != 180*24*time.Hour {
		t.Errorf("fleet window = %v; want the 180d default", cfg.FleetWindow())
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
		{"negative ttl", "cache_ttl = '-5s'", nil},
		{"unparsable ttl", "cache_ttl = 'soon'", nil},
		{"unparsable fleet window", "fleet_active = 'recently'", nil},
		{"zero fleet window", "fleet_active = '0d'", nil},
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
		{
			"interactive with where",
			"[[tasks]]\nname = 'a'\nrun = 'lazygit'\ninteractive = true\nwhere = 'dirty'",
			config.ErrTaskInteractive,
		},
		{
			"interactive with every",
			"[[tasks]]\nname = 'a'\nrun = 'lazygit'\ninteractive = true\nevery = '1d'",
			config.ErrTaskInteractive,
		},
		{
			"interactive with concurrency",
			"[[tasks]]\nname = 'a'\nrun = 'lazygit'\ninteractive = true\nconcurrency = 2",
			config.ErrTaskInteractive,
		},
		{
			"interactive unknown placeholder",
			"[[tasks]]\nname = 'a'\nrun = 'x {sha}'\ninteractive = true",
			config.ErrTaskPlaceholder,
		},
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

// The whole-minutes key no longer applies, but it must warn rather than fail
// silently: a config carrying it was written for a TTL that is now different.
func TestCacheTTLMinutesWarns(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", writeConfig(t, "cache_ttl_minutes = 10\ncache_ttl = \"90s\"\n"))

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	warnings := cfg.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "cache_ttl_minutes") ||
		!strings.Contains(warnings[0], "10m0s") {
		t.Errorf("warnings = %v", warnings)
	}

	if cfg.TTL() != 90*time.Second {
		t.Errorf("cache ttl = %v; want the cache_ttl value, not the ignored one", cfg.TTL())
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
