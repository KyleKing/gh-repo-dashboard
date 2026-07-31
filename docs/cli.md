# Command line

```bash
# Scan the config file's scan_paths, or with no config the enclosing repo
# (walking up from the current directory), or the current directory itself
gh repo-dashboard

# Scan specific directories
gh repo-dashboard ~/projects ~/work

# Limit scan depth (default 1)
gh repo-dashboard -depth 2 ~/Developer
```

Flags and positional paths override the [config file](./configuration.md).

## Flags

| Flag | Effect |
|------|--------|
| `-depth N` | Maximum directory depth to scan, default 1 |
| `--cli` | Print repo summaries as JSON instead of launching the TUI |
| `--fresh` | With `--cli`, fetch fresh GitHub PR data instead of reading the cache |
| `--filter <expr>` | Narrow `--cli` output by a [predicate expression](./filtering.md) |
| `--script <path>` | Run `:command` lines headlessly from a file, or `-` for stdin |
| `--version` | Print version information |

## JSON output

```bash
# Cached GitHub data only, so PR fields may be missing
gh repo-dashboard --cli ~/projects

# Invoke gh per repo for current PR data
gh repo-dashboard --cli --fresh ~/projects

# Only the repos matching a predicate
gh repo-dashboard --cli --filter 'dirty and has_notes' ~/projects
```

## Script files

A script holds one `:command` per line. The leading `:` is optional, and the
runner skips blank lines and `#` comments.

```text
# fetch everything that is behind, then report
:fetch behind
:filter dirty
```

```bash
gh repo-dashboard --script maintenance.txt ~/projects
```

## Commands

The same commands work in the TUI command bar (`:`) and in scripts, with tab
completion on names and arguments: `cleanup`, `fetch`, `filter`, `help`,
`history`, `prune`, `quit`, `refresh`, `select`, and `sort`. A unique prefix
resolves, so `:fil` reaches `:filter`.

`:history` lists the recognized commands you have run, capped at 50, shared
across the command bar, the `@:` repeat key, and script runs.

The test fixtures generate worked examples in [USAGE.md](./USAGE.md).
