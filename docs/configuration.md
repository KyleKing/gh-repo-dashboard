# Configuration

An optional TOML file at `$XDG_CONFIG_HOME/gh-repo-dashboard/config.toml`
(defaulting to `~/.config/gh-repo-dashboard/config.toml`) sets defaults, so you
can launch from anywhere with no arguments. Every key is optional, and a missing
file means the built-in defaults.

```toml
# Directories scanned when no paths are passed on the command line
scan_paths = ["~/Developer", "~/work"]

# Default for -depth
depth = 2

# Per-repo notes filenames checked at each repo root, every match is shown
notes_filenames = [".doing", "doing.md", "doing.txt", "TODO.md"]

# Lifetime of cached GitHub data (PRs, workflow runs), in minutes
cache_ttl_minutes = 5
```

Flags and positional paths on the command line take precedence over the file.

## Caching

The dashboard caches GitHub pull request and workflow data for
`cache_ttl_minutes`, defaulting to 5. Press `r` in the TUI to clear the caches
and reload, or pass `--fresh` alongside `--cli`.
