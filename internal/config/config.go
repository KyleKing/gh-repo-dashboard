// Package config loads the optional TOML config file, applied below flags and
// above built-in defaults.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds the user-configurable settings read from config.toml. Zero
// values mean "not set"; callers fall back to their built-in defaults.
type Config struct {
	ScanPaths       []string `toml:"scan_paths"`
	Depth           int      `toml:"depth"`
	NotesFilenames  []string `toml:"notes_filenames"`
	CacheTTLMinutes int      `toml:"cache_ttl_minutes"`
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
)
