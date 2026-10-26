// Package config loads the optional TOML config file, applied below flags and
// above built-in defaults.
package config

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
)

// Config holds the user-configurable settings read from config.toml. Zero
// values mean "not set"; callers fall back to their built-in defaults.
type Config struct {
	ScanPaths       []string `toml:"scan_paths"`
	Depth           int      `toml:"depth"`
	NotesFilenames  []string `toml:"notes_filenames"`
	CacheTTLMinutes int      `toml:"cache_ttl_minutes"`
	// CacheToDisk is a pointer because an absent key means "on": persistence is
	// an opt-out, and false is a value the user has to write.
	CacheToDisk *bool      `toml:"cache_to_disk"`
	Diff        DiffConfig `toml:"diff"`
	// FleetActive is how recently one of FleetAuthors must have committed for
	// the TUI's fleet scope to list a repo, in the syntax of a task's every.
	FleetActive string `toml:"fleet_active"`
	// FleetAuthors are the author emails whose commits keep a repo in the fleet
	// scope. Empty means the global git user.email.
	FleetAuthors []string `toml:"fleet_authors"`
	Tasks        []Task   `toml:"tasks"`
}

// DiffConfig selects how a patch is rendered.
type DiffConfig struct {
	// External is a diff command in the form git's diff.external takes, and
	// wins over whatever git config sets. Empty means git decides.
	External string `toml:"external"`
}

// PersistCache reports whether remote-derived values may be written to the
// per-upstream cache files, which they may unless the file says otherwise.
func (c Config) PersistCache() bool {
	return c.CacheToDisk == nil || *c.CacheToDisk
}

// DefaultFleetActive is the fleet scope's window when fleet_active is unset.
const DefaultFleetActive = "180d"

// FleetWindow returns FleetActive as a duration, DefaultFleetActive when unset.
func (c Config) FleetWindow() time.Duration {
	window, err := filters.ParseAge(cmp.Or(c.FleetActive, DefaultFleetActive))
	if err != nil {
		return 0
	}

	return window
}

// CacheTTL returns the configured cache TTL, or zero when unset.
func (c Config) CacheTTL() time.Duration {
	return time.Duration(c.CacheTTLMinutes) * time.Minute
}

// Path returns the config file location: $XDG_CONFIG_HOME or ~/.config,
// joined with gh-repo-dashboard/config.toml.
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}

	return filepath.Join(base, "gh-repo-dashboard", "config.toml"), nil
}

// Load reads the config file at Path. A missing file is not an error and
// yields the zero Config; a malformed file is an error so typos aren't
// silently ignored.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}

	return loadFile(path)
}

func loadFile(path string) (Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // fixed path under the user's config dir
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	if cfg.Depth < 0 {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, errNegativeDepth)
	}
	if cfg.CacheTTLMinutes < 0 {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, errNegativeTTL)
	}

	if cfg.FleetActive != "" && cfg.FleetWindow() <= 0 {
		return Config{}, fmt.Errorf("parsing config %s: fleet_active: %w", path, errFleetActive)
	}

	if err := validateTasks(cfg.Tasks); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	expanded, err := expandHomePaths(cfg.ScanPaths)
	if err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	cfg.ScanPaths = expanded

	return cfg, nil
}

// expandHomePaths replaces a leading "~/" in each path with the user's home
// directory, since config values don't pass through shell expansion.
func expandHomePaths(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "~" || strings.HasPrefix(p, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("expanding %q: %w", p, err)
			}
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
		out = append(out, p)
	}

	return out, nil
}

var (
	errNegativeDepth = errors.New("depth must be non-negative")
	errNegativeTTL   = errors.New("cache_ttl_minutes must be non-negative")
	errFleetActive   = errors.New("must be a positive duration such as 180d or 12h")
)
