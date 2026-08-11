# Design

Project-specific architecture, design decisions, and domain context for
gh-repo-dashboard. Generic Go and workflow conventions live in [AGENTS.md](AGENTS.md);
setup and task commands live in [CONTRIBUTING.md](CONTRIBUTING.md).

## Overview

K9s-inspired Bubble Tea TUI for managing multiple git and jj repositories with
progressive loading, filtering, GitHub PR integration, and batch maintenance tasks.
Besides the TUI, two headless modes share the same internals: `--cli` prints repo
summaries as JSON (optionally narrowed by `--filter <predicate>`) and `--script`
replays `:command` lines from a file or stdin.

- Framework: Bubble Tea (Go TUI framework)
- Theme: Catppuccin Macchiato
- Philosophy: minimal color, single unified background, borders for hierarchy, vim-style keybindings

## Architecture

```
├── cmd/gh-repo-dashboard/    # CLI entry point (main.go, flag/config wiring)
├── internal/
│   ├── app/                  # Bubble Tea app
│   │   ├── app.go           # Model definition, Init
│   │   ├── update.go        # Update function (message handling)
│   │   ├── view.go          # Shared rendering scaffolding
│   │   ├── view_*.go        # Per-view-mode rendering (repolist, panels, palette, ...)
│   │   ├── panels.go        # Focused view's panel model and vertical sizing
│   │   ├── palette.go       # Universal find's query grammar and result types
│   │   ├── keymap.go        # Key bindings
│   │   ├── command.go       # :command registry and commands
│   │   ├── commands.go      # tea.Cmd constructors
│   │   ├── script.go        # --script headless runner
│   │   ├── textobject.go    # Text objects and operators
│   │   └── messages.go      # Message types
│   ├── batch/                # Batch operations (runner, tasks)
│   ├── cache/                # Generic TTL cache with registry
│   ├── cli/                  # --cli JSON output mode
│   ├── config/               # TOML config file loading
│   ├── discovery/            # Repo discovery
│   ├── filters/              # Filter/sort/search and predicate logic
│   ├── github/               # GitHub integration (pr, workflow)
│   ├── models/               # Data structures (repo, branch, pr, notes, enums)
│   ├── vcs/                  # VCS abstraction (operations, git, jj, factory, mock)
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
the branch itself was never merged. Every `Mutator` method returns
`(success bool, message string)` alongside an error, so the UI can report a
per-repo outcome even where the operation itself did not error. Beyond the batch
tasks (`FetchAll`, `PruneRemote`, `CleanupMergedBranches`) it covers the
single-item writes the focused view's panels offer: `SwitchBranch`,
`PushBranch`, `DeleteBranch`, `ApplyStash`, and `DropStash`. `DeleteBranch`
refuses an unmerged branch unless the caller passes `force`, which only a caller
that has verified the branch is squash-merged should do. jj has no stash, so its
`ApplyStash` and `DropStash` report that rather than pretending to work.
`CleanupMergedBranches(ctx, repoPath, squashMerged
[]string)` additionally takes the caller-verified squash-merged branch names: git
deletes them with `-D` alongside true-merges deleted with `-d`, reports per-branch
failures in the result message, and returns success only when every deletion
succeeded. Both paths skip the current branch and anything checked out in a
worktree. Merged branches come from `for-each-ref --merged`, not `git branch
--merged`, whose porcelain marks (`* `, `+ `, and a detached-HEAD line) are not
branch names. `GitOperations` and
`JJOperations` each also expose a `PreviewMergedBranches` method (outside the
`Mutator` interface, since it's read-only) that reports what cleanup would delete
without deleting anything, backing the `:cleanup --dry-run` command.

### GitHub CLI integration

GitHub integration works for both git and jj via the `gh` CLI. For git repos and
colocated jj repos it uses the `.git` directory; for non-colocated jj repos it sets
`GIT_DIR` to `.jj/repo/store/git`. The `GetGitHubEnv()` helper in `vcs/factory.go`
handles this transparently. If `gh` is missing, PR columns show a dash rather than failing.

`GetPRsForRepo` reads the list as two filtered pages rather than one unfiltered
page. The `statusCheckRollup`, `comments`, and `reviews` fields are each walked
per pull request, so the cost is linear in the page size, and a repo with enough
open pull requests takes GitHub's GraphQL gateway past its own timeout and
returns 504. Thirty per page fits with room to spare. The pages are split by
author (`-author:@me` and `--author @me`) because a repo whose pull requests all
belong to a bot has nothing on the operator's page, while the operator's older
work has already fallen off a busy repo's recent list, so neither page alone
covers both. Results merge by number, newest first. A failed fetch is never
cached: an empty list would otherwise read as "this repo has no pull requests"
for the whole TTL and hide the panel on the strength of a timeout.

## Batch Tasks

`BatchTaskRunner` runs maintenance tasks sequentially across the currently filtered
repositories, using the VCS factory per repo, tracking progress, and continuing on
failure (failures are highlighted, not fatal). Progress is reported via Tea messages.

Batch operations are read-only by default; write operations require an explicit
keybinding. Scope is always the filtered set, making the blast radius explicit.
Tasks that delete refs are parked behind `ViewModeConfirm` first, naming the
repos and the count, whether they were started by the operator keys, `:cleanup`,
or a text object.

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

## Copier Template Info

`internal/copier` reads a repo's `.copier-answers.yml` (copier's default
answers-file name) for `_src_path` and `_commit`, exposed as
`models.RepoSummary.TemplateInfo` (nil for non-copier repos). When `_commit`
parses as a semver tag, it's compared against the template's latest upstream
tag (`git ls-remote --tags --refs <_src_path>`); when it doesn't (a raw commit
SHA or branch ref), currency can't be judged, so the TUI just flags it as
non-tag rather than guessing. The latest-tag lookup is cached in
`cache.CopierLatestTagCache` keyed by `_src_path` rather than by repo path, so
every repo generated from the same template shares one network call. The repo
list's TEMPLATE column shows the installed tag, `tag→latest` when behind, or
the installed ref plus a warning icon when it isn't a tag.

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

`ViewModeRepoList` (initial) lists repositories with
Name/Branch/Status/Peers/PR/Template/Modified columns. A linked checkout whose parent
was discovered too is left out of that list: a git worktree or jj workspace is a place
inside its parent repo, and the parent's Peers panel already names it. Scanned on its
own it has no parent in the set and stays listed. `ViewModeRepoDetail` (Enter) is a
grid of panels (Status, Branches, PRs, Peers, Stashes, Notes, less the ones with nothing
to list) beside a detail pane rendering whatever the cursor sits on; when discovery finds exactly one repo
it opens directly, and esc still falls back to the one-row list. `ViewModePalette`
(`space` in the focused view, `;` in the list) is the universal find.
`ViewModeFilter` (f), `ViewModeSort` (s), and `ViewModeHelp` (?) are modals,
`ViewModeConfirm` gates every write action, and `ViewModeBatchProgress` shows a progress bar
during batch runs.

### Panel grid

`internal/app/panels.go` holds the panel model and the vertical size math;
`view_panels.go` builds each panel's rows from the Model and renders the grid.
Below the compact breakpoint the grid becomes a single stack that scrolls to
keep the focused panel and its detail on screen.

Height is shared in three passes: every panel that wants them gets up to an
equal share, the focused panel is then filled to its own content, and a
relevance score derived from cached data splits what is left. The equal share
comes first so relevance cannot starve one list beside a busier one. No panel
drops below its border plus one line, and lines nothing claims stay unspent, so
the column ends where its content does instead of padding boxes with blank rows.

A panel with nothing to list is dropped rather than drawn around the word
"none": a jj repo has no Stashes panel, and neither does a repo with no pull
requests. Everything stays on screen while `detailLoading` is set, so the grid
settles once instead of reflowing under the cursor as data lands, and
`statusAbsences` puts what was dropped on one Status line so an absence is still
reported. `snapFocusToShownPanel` runs wherever a message replaces those lists,
because a cursor parked on a panel that is not drawn would render nothing.

Jump keys live in `panelKeys`, fixed per panel rather than per grid position so
hiding one cannot move another out from under the fingers, and `panelTitle`
brackets each inside the panel's own border (`[B]ranches`) instead of naming
them in a footer legend. The column reads as one vertical list: `moveDetailCursor`
carries a move off either end of a panel into its neighbor, so a panel holding
one row (or none, as Status does) cannot swallow `j`/`k`.

Write actions live behind the `!` leader (`panelActionsFor` in `actions.go`),
the same key the universal find uses to act on a result set. Scoping the verbs
to the focused panel keeps them mnemonic without spending a top-level key each,
and leaves the single-key namespace for movement and panel jumps. Adding one is
a row in `panelActionsFor` plus a method with the `func(Model) (tea.Model,
tea.Cmd)` shape; anything that reaches the remote or destroys work routes
through `confirmAction` first.

### Universal find

`internal/app/palette.go` parses the query grammar (`#12` a PR number, a
one-letter prefix for branches/stashes/notes/repos, `*` to widen the scope, bare
text for everything) and `view_palette.go` answers it from cache-resident data
only, so no keystroke costs a fetch. `tab` marks rows, `!` opens the verbs for
the target set, and selecting a repo set commits it to the selected-repos text
object so `F`/`P`/`C`/`R` compose with a find.

Adding a view mode: add the const in `app/app.go`, rendering in a `view_*.go`
file (dispatched from `renderView` in `view.go`), update handling in
`update.go`, and enter/exit navigation.

## Bubble Tea Patterns

The `Model` holds view state (mode, loading, cursor), data (repo paths, filtered
paths, summaries map), and UI dimensions. `Update` switches on message type and
returns an updated model plus an optional command. Long-running work runs in Tea
commands that return messages (for example `loadRepoSummary` returns
`RepoSummaryLoadedMsg` or `RepoSummaryErrorMsg`). Views render with Lipgloss, reusing
cached style objects.

Adding a keybinding: register it in `keymap.go`, handle it in `handleKey()`
(`update.go`), update help text in `view_modals.go`, and add a test in `app_test.go`.

## Key Features

- Progressive loading: the repo list appears immediately with placeholder data while goroutines load each `RepoSummary` concurrently and the table updates incrementally via Tea messages, never blocking on slow git operations
- Caching: a generic TTL cache with mutex protection backs `prCache`, `branchCache`, and `summaryCache`; refresh clears all caches
- Notes detection (surfacing as a Notes panel in the focused view): every configured notes filename (`.doing`, `doing.md`, `doing.txt`, `TODO.md` by default; overridable via config) present at a repo root is collected as a `models.NoteFile`, not just the first match; surfaces as a count badge in the Status column, a first-line preview toggled with `v` on the repo list, the focused view's Notes panel with the file body in the detail pane, and the `has_notes` filter/predicate with the `nr` text object; detection is a plain file check outside the VCS abstraction. The Notes panel's `!e` verb hands the file to `$EDITOR` through `tea.ExecProcess`, which is the only path by which the dashboard causes a notes file to be written
- Configuration: optional TOML at `$XDG_CONFIG_HOME/gh-repo-dashboard/config.toml` (`internal/config`) supplies scan paths, depth, notes filenames, and cache TTLs; flags take precedence
- Command history: `ExecuteCommand` records recognized commands (capped at 50), shared by the command bar, `:history`, the `@:` repeat key, and `--script` runs
- Parallel checkouts: repos sharing a remote (`RepoSummary.RemoteRepo`, derived from the remote URL) are peers of each other, as are a repo's own worktrees/workspaces; `models.FindPeerCheckouts` and `models.WorktreeCheckouts` build the set, surfacing as the repo list's PEERS count, a repo-detail header badge, the branch list's CHECKED OUT column, and a branch-detail line. A repo with no known remote never peers with anything, since an empty remote would group every unrelated local-only repo
- Universal find: `space` (focused view) or `;` (fleet list) opens a typed-prefix palette over cache-resident data; `tab` marks rows, `!` runs a verb on the set, and a repo set commits to the selected-repos text object so batch operators compose with it
- Panel verbs: the focused view's write actions sit behind the `!` leader, scoped to the focused panel (`panelActionsFor` in `internal/app/actions.go`) so the letters stay mnemonic without each costing a top-level key. Everything that touches the remote or destroys work is parked behind `ViewModeConfirm` rather than running on the keypress; switching and branch deletion are refused up front when the branch is already checked out here or held by a parallel checkout, and a stash is applied rather than popped so the stash survives a mistake. Results arrive as `ActionResultMsg` and reload the repo's summary and detail
- Detail pane: renders whatever the panel cursor sits on, from data the panel load already holds. A pull request lists its failing and in-flight checks by name with the settled ones tallied, capped so a wall of failures cannot cost a frame; a stash shows its diffstat, with `!o` swapping in the full patch under a line cap. Both are bounded because `panelDetailLines` is rebuilt several times per frame
- Cancellation: use `context.Context` and cancel when leaving views or quitting to avoid goroutine leaks

## Testing

The strategy is a layered pyramid: direct state-transition tests as the base, a thin
teatest golden-file layer for visual regression on stable screens, and fixture-based
command sequences (`internal/app/testdata/fixtures/*.fix`) that also generate
`docs/USAGE.md` via `mise run docs:usage`. Golden-file tests run under a build tag
(`go test -tags=golden ./...`, add `-update` to refresh snapshots).

## External Dependencies

- git and jj CLIs are needed only for the VCS types you actually manage; each is assumed to be on `PATH`
- gh (GitHub CLI) is optional and enables PR features for both git and jj repos; non-colocated jj repos get `GIT_DIR` set automatically

## Release Checklist

1. `go test ./...` and `go test -race ./...`
2. Manually test with real git and jj repositories, including batch operations (fetch, prune, cleanup)
3. Update `README.md` if features changed
4. Tag a conventional-commit-driven release (commitizen bump)
