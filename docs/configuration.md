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

# Keep pull request data between runs, one file per remote (default true)
cache_to_disk = true

# Diff viewer, in the form git's diff.external takes; unset means git decides
[diff]
external = "difft --color=always --display=inline --syntax-highlight=off"
```

Flags and positional paths on the command line take precedence over the file.

## Diff viewer

A stash's full patch (`!o` in the Stashes panel) renders through the same
external diff command git itself would run, so a repo or global
`diff.external = difft` is picked up with no further setup. `[diff] external`
overrides it for this tool alone.

The viewer runs without a terminal, which is how a tool decides to drop its
color and assume eighty columns, so the pane's width arrives in `COLUMNS` and
`DFT_WIDTH`, and color is forced through `CLICOLOR_FORCE`, `DFT_COLOR`, and
`DFT_DISPLAY=inline`. A flag written into the command itself still wins over
all of them, which is the way to configure a viewer that reads neither.

## Caching

The dashboard caches GitHub pull request and workflow data for
`cache_ttl_minutes`, defaulting to 5. Press `r` in the TUI to clear the caches
and reload, or pass `--fresh` alongside `--cli`.

With `cache_to_disk` left on, the pull request list and the merged-PR head map
also outlive the process, in one file per remote under
`os.UserCacheDir()/gh-repo-dashboard` (`~/Library/Caches` on macOS,
`$XDG_CACHE_HOME` or `~/.cache` on Linux), so a cold start on a large fleet
renders without one `gh pr list` per repo. A file is read when the row that
needs it is drawn, never all of them at startup.

What lands there is pull request numbers, states, check counts, and titles. A
title from a private repository is the most any of it reveals, so the files are
written at mode 0600 under a 0700 directory. Pull request bodies and comment
text stay in memory and are never written. Set `cache_to_disk = false` to keep
everything in memory; `r` drops the files along with the memory caches.
