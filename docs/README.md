# gh-repo-dashboard docs

A Bubble Tea TUI that scans a set of directories, lists every git and jj
repository it finds, and loads working-tree state, branches, and GitHub pull
request data for each one. Two headless modes share the same internals: `--cli`
prints repo summaries as JSON, and `--script` replays `:command` lines from a
file or stdin.

## Pages

- [Interface](./interface.md) for the views, the columns, the status symbols, and the handful of keys worth memorizing
- [Filtering and sorting](./filtering.md) for filter modes, predicate expressions, sorting, and search
- [Batch operations](./batch-operations.md) for fetch, prune, and merged-branch cleanup across many repos
- [Command line](./cli.md) for flags, JSON output, and script files
- [Configuration](./configuration.md) for the TOML config file
- [Troubleshooting](./troubleshooting.md) for the common blank columns and empty lists
- [Development](./development.md) for the contributor workflow [CONTRIBUTING.md](../CONTRIBUTING.md) leaves out
- [Alternatives](./alternatives.md) for how this compares to other multi-repo and single-repo tools
- [Usage examples](./USAGE.md), generated from the command fixtures, so they cannot drift from the code

## Version control support

Git repositories work fully. Jujutsu (jj) support is best-effort for both
colocated and non-colocated repositories, and jj's CLI output formats still
change between releases, so some fields may be missing on newer jj versions.

The dashboard detects the VCS type from the directory contents, and reads a
colocated repository, one holding both `.git` and `.jj`, through jj.

## Requirements

- git CLI, to manage git repos
- jj CLI, to manage jj repos
- gh CLI, optional, for pull request and workflow data in both git and jj repos
- Go 1.25+, only to build from source

## What it gives you

- Progressive loading, so the list appears immediately and each repository fills
  in as its data arrives
- TTL caching for pull request data, workflow status, and VCS reads
- Multi-repo filters, predicate expressions, fuzzy search, and reversible sorts
- Batch fetch, prune, and merged-branch cleanup over whatever is filtered
- Per-repo notes files shown as a badge, a preview line, and a detail tab
- Worktree and workspace detection, plus git stash counts
- Copier template version, showing the installed tag and whether it is behind
- Catppuccin Macchiato theme, which uses color only for actionable elements
