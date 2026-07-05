# Design

Project-specific architecture, design decisions, and domain context for
gh-repo-dashboard. Generic Go and workflow conventions live in [AGENTS.md](AGENTS.md);
setup and task commands live in [CONTRIBUTING.md](CONTRIBUTING.md).

## Overview

K9s-inspired Bubble Tea TUI for managing multiple git and jj repositories with
progressive loading, filtering, GitHub PR integration, and batch maintenance tasks.

- Framework: Bubble Tea (Go TUI framework)
- Theme: Catppuccin Macchiato
- Philosophy: minimal color, single unified background, borders for hierarchy, vim-style keybindings

## Architecture

```
├── cmd/gh-repo-dashboard/    # CLI entry point (main.go)
├── internal/
│   ├── app/                  # Bubble Tea app
│   │   ├── app.go           # Model definition, Init
│   │   ├── update.go        # Update function (message handling)
│   │   ├── view.go          # View rendering
│   │   ├── keymap.go        # Key bindings
│   │   ├── commands.go      # Tea commands
│   │   └── messages.go      # Message types
│   ├── models/               # Data structures (repo, branch, pr, filter, enums)
│   ├── vcs/                  # VCS abstraction (operations, git, jj, factory, mock)
│   ├── filters/              # Filter/sort/search logic
│   ├── discovery/            # Repo discovery
│   ├── batch/                # Batch operations (runner, tasks)
│   ├── github/               # GitHub integration (pr, workflow)
│   ├── cache/                # Generic TTL cache
│   └── ui/styles/            # Lipgloss styles
```

## VCS Abstraction

An interface-based abstraction supports multiple version control systems.

- `Operations` (in `vcs/operations.go`) composes three narrower interfaces: `StatusReader`
  (summary-level queries), `DetailReader` (branch/stash/worktree/commit drill-downs), and
  `Mutator` (write operations)
- `GitOperations` and `JJOperations` implement it
- `DetectVCSType()` auto-detects by directory presence (`.git` or `.jj`)
- `GetOperations()` returns the appropriate implementation
- Colocated repos (both `.git` and `.jj`) prefer jj

### Git vs JJ concept mapping

| Concept | Git | JJ (Jujutsu) | Notes |
|---------|-----|--------------|-------|
| Current location | HEAD | @ (working copy) | jj always has a working copy change |
| Branch | branch | bookmark | jj bookmarks are similar to git branches |
| Staged changes | index/staging | N/A | jj automatically tracks all changes |
| Uncommitted | unstaged + staged | working copy | Different mental model |
| Ahead/behind | ahead/behind | ahead/behind | Similar concept |
| Remote tracking | upstream branch | tracking bookmark | Similar |
| Stash | stash | N/A | jj can create changes instead |
| Worktree | worktree | workspace | jj workspaces are more powerful |

Read operations include `GetRepoSummary`, `GetCurrentBranch`, `GetBranchList`,
`GetStashList` (git only), `GetWorktreeList`, `GetCommitLog`, and `GetAheadBehind`.
File-status counts are computed internally by `GetRepoSummary` rather than exposed
as separate interface methods. `models.BranchInfo` carries a `Head` tip-OID field
(git `for-each-ref`'s `%(objectname)`, jj's bookmark target commit id) used to
detect squash-merged branches whose tip matches a merged PR's head OID even though
the branch itself was never merged. Write operations used by batch tasks
(`FetchAll`, `PruneRemote`, `CleanupMergedBranches`) return `(success bool, message
string)` for UI feedback. `CleanupMergedBranches(ctx, repoPath, squashMerged
[]string)` additionally takes the caller-verified squash-merged branch names: git
deletes them with `-D` (skipping the current branch and any branch checked out in
a worktree) alongside true-merges deleted with `-d`, and reports per-branch
failures in the result message rather than swallowing them. `GitOperations` and
`JJOperations` each also expose a `PreviewMergedBranches` method (outside the
`Mutator` interface, since it's read-only) that reports what cleanup would delete
without deleting anything, backing the `:cleanup --dry-run` command.

### GitHub CLI integration

GitHub integration works for both git and jj via the `gh` CLI. For git repos and
colocated jj repos it uses the `.git` directory; for non-colocated jj repos it sets
`GIT_DIR` to `.jj/repo/store/git`. The `GetGitHubEnv()` helper in `vcs/factory.go`
handles this transparently. If `gh` is missing, PR columns show a dash rather than failing.

## Batch Tasks

`BatchTaskRunner` runs maintenance tasks sequentially across the currently filtered
repositories, using the VCS factory per repo, tracking progress, and continuing on
failure (failures are highlighted, not fatal). Progress is reported via Tea messages.

Batch operations are read-only by default; write operations require an explicit
keybinding. Scope is always the filtered set, making the blast radius explicit.

`batch.CleanupMerged` and `batch.PreviewCleanup` detect squash-merged branches by
comparing `internal/github.GetMergedPRHeads` (cached merged PR head OIDs) against
`GetBranchList`'s `Head` field, reading through swappable `getMergedPRHeads`/
`getOperations` package-level seams (`internal/batch/export_test.go`) so tests can
stub gh/git access without shelling out. `PreviewCleanup` backs `:cleanup
--dry-run`: it runs the same detection plus each VCS's `PreviewMergedBranches` and
reports what would be deleted without calling `CleanupMergedBranches`.

Adding a new batch task:

1. Add the method to the `Mutator` interface (`vcs/operations.go`)
2. Implement it in both `GitOperations` (`vcs/git.go`) and `JJOperations` (`vcs/jj.go`)
3. Add a task function in `batch/tasks.go` wrapping the VCS call
4. Handle it in `app/update.go` via `m.startBatchTask(...)`
5. Register the keybinding in `app/keymap.go`
6. Add tests in `internal/batch/batch_test.go`

## Filtering Architecture

Filtering is compositional: `FilterMode -> SearchText -> SortMode -> Display`. For
example, the `DIRTY` filter plus an `api` search yields dirty repos containing "api".

- Filter modes: `ALL`, `DIRTY`, `AHEAD`, `BEHIND`, `HAS_PR`, `HAS_STASH`, `HAS_NOTES` (multi-filter with AND logic)
- Sort modes: `NAME`, `MODIFIED`, `STATUS`, `BRANCH`, with multi-field priority and ASC/DESC direction
- Search: case-insensitive fuzzy matching via `sahilm/fuzzy`, applied after filter mode and before sort, updating in real time

Adding a filter mode: add the const to `models/enums.go`, a filter function in
`filters/filter.go`, a case in `FilterRepos()`, and tests in `filters/filter_test.go`.

## UI Design

Catppuccin Macchiato palette. Color is reserved for actionable elements (badges,
accents) over a single unified background; borders carry the visual hierarchy and
the cursor uses Surface0.

| Role | Hex |
|------|-----|
| Base (background) | `#24273a` |
| Surface0 (cursor, elevated) | `#363a4f` |
| Text | `#cad3f5` |
| Subtext0 | `#a5adcb` |
| Blue (primary accent, borders) | `#8aadf4` |
| Mauve (search) | `#c6a0f6` |
| Yellow (filter) | `#eed49f` |
| Green (success, PRs) | `#a6da95` |
| Peach (dirty repos) | `#f5a97f` |

### View hierarchy

`ViewModeRepoList` (initial) lists repositories with Name/Branch/Status/PR/Modified
columns. `ViewModeRepoDetail` (Enter) drills into branches, stashes, worktrees,
PRs, and notes with tab switching. `ViewModeFilter` (f), `ViewModeSort` (s), and `ViewModeHelp`
(?) are modals, and `ViewModeBatchProgress` shows a progress bar during batch runs.

Adding a view mode: add the const in `app/app.go`, rendering in `view.go`, update
handling in `update.go`, and enter/exit navigation.

## Bubble Tea Patterns

The `Model` holds view state (mode, loading, cursor), data (repo paths, filtered
paths, summaries map), and UI dimensions. `Update` switches on message type and
returns an updated model plus an optional command. Long-running work runs in Tea
commands that return messages (for example `loadRepoSummary` returns
`RepoSummaryLoadedMsg` or `RepoSummaryErrorMsg`). Views render with Lipgloss, reusing
cached style objects.

Adding a keybinding: register it in `keymap.go`, handle it in `handleKey()`
(`update.go`), update help text in `view.go`, and add a test in `app_test.go`.

## Key Features

- Progressive loading: the repo list appears immediately with placeholder data while goroutines load each `RepoSummary` concurrently and the table updates incrementally via Tea messages, never blocking on slow git operations
- Caching: a generic TTL cache with mutex protection backs `prCache`, `branchCache`, and `summaryCache`; refresh clears all caches
- Notes detection: a per-repo notes file (`.doing`, `doing.md`, `doing.txt`, or `TODO.md` at the repo root, first match wins) surfaces as a badge in the Status column, a Notes tab in repo detail, and the `has_notes` filter/predicate with the `nr` text object; detection is a plain file check outside the VCS abstraction
- Cancellation: use `context.Context` and cancel when leaving views or quitting to avoid goroutine leaks

## Testing

The strategy is a layered pyramid: direct state-transition tests as the base, a thin
teatest golden-file layer for visual regression on stable screens, and fixture-based
(catwalk-style) sequences once command mode lands. Golden-file tests run under a build
tag (`go test -tags=golden ./...`, add `-update` to refresh snapshots). See
[ROADMAP.md](ROADMAP.md) for how these phase in.

## External Dependencies

- git and jj CLIs are needed only for the VCS types you actually manage; each is assumed to be on `PATH`
- gh (GitHub CLI) is optional and enables PR features for both git and jj repos; non-colocated jj repos get `GIT_DIR` set automatically

## Release Checklist

1. `go test ./...` and `go test -race ./...`
2. Manually test with real git and jj repositories, including batch operations (fetch, prune, cleanup)
3. Update `README.md` if features changed
4. Tag a conventional-commit-driven release (commitizen bump)
