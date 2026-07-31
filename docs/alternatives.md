# Alternatives

gh-repo-dashboard borrows its shape from [K9s](https://k9scli.io/): one list of
many things, filters and sorts over that list, drill-down into any row, and a
`:command` bar. If you know K9s, the navigation will already be familiar.

## Multi-repository TUIs

[Git-Scope](https://github.com/Bharath-code/git-scope) is the closest match,
also built with Bubble Tea.

| Feature | gh-repo-dashboard | Git-Scope |
|---------|-------------------|-----------|
| VCS support | Git and Jujutsu (jj) | Git only |
| GitHub integration | PR details, checks, and status via gh CLI | Contribution graphs |
| Filtering | 7 modes plus predicate expressions | Dirty filter and pagination |
| Batch operations | Fetch, prune, cleanup merged branches | None |
| Search | Fuzzy search by name | Fuzzy search by name, path, or branch |
| Also has | Worktrees and workspaces, stash tracking, copier template version | Editor launch, disk usage, timeline view |

## Other multi-repository tools

- [Gita](https://github.com/nosarthur/gita) manages many git repos from the CLI with custom groups and batch operations
- [gitbatch](https://github.com/isacikgoz/gitbatch) manages git repositories in one interactive TUI
- [mgitstatus](https://github.com/fboender/multi-git-status) reports uncommitted, untracked, and unpushed changes across repos
- [Mani](https://github.com/alajmo/mani) is a Go CLI with YAML config, a built-in TUI, batch operations, and parallel command execution

## Single-repository TUIs

Use one of these to work inside a single repo, which
[gh-repo-dashboard deliberately does not do](../README.md#what-it-does-not-do).

- [lazygit](https://github.com/jesseduffield/lazygit) is a terminal UI for git commands
- [GitUI](https://github.com/extrawurst/gitui) is a fast terminal UI for git written in Rust
- [Gitu](https://github.com/altsem/gitu) is a git TUI inspired by Magit
