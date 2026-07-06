# Roadmap

Phased, shippable milestones for gh-repo-dashboard. Each milestone stands on its
own and can be released independently; later milestones assume earlier ones landed
but the roadmap can stop at any point without leaving the app half-migrated.

Architecture and domain context live in [DESIGN.md](DESIGN.md); Go and workflow
conventions live in [AGENTS.md](AGENTS.md).

## Vision

Grow the dashboard into a composable, vim-paradigm TUI: text objects (dirty repos,
repos with PRs), operators (fetch, prune, cleanup), and composition (`Fdr` fetches
dirty repos) driven either by keys or a `:command` mode with predicates
(`:filter dirty and has_pr`). The composable command layer is also the seam that
makes behavior testable without keyboard simulation and documentable from fixtures.

This is deliberately sliced so each capability ships as a complete feature rather
than as one large rewrite. The external `tui-commander` package extraction is
deferred until a second TUI actually shares the code (extract on second use).

## Testing strategy

A layered pyramid, adopted incrementally across the milestones below:

- Direct state-transition tests are the base layer: fast, dependency-free, and where
  the current coverage gap is (`app` ~25%, `github` ~11%, `vcs` ~9.5%). Most new
  tests land here
- teatest golden files are a thin regression layer over a few stable screens (repo
  list, filter modal, detail, batch progress). Kept deliberately small so snapshots
  do not go brittle. Run under a build tag (`go test -tags=golden ./...`, `-update`
  to refresh)
- Fixture-based tests (catwalk-style) arrive with command mode, where scripted
  command sequences are the natural unit and can double as generated usage docs

## Milestones

### M1: Bubble Tea v2 upgrade (shipped)

Foundational. v2 reworks input/key handling and the cursor model and ships companion
`bubbles`/`lipgloss` v2 releases. It touches the same `Model`/`Update`/`View` surface
the command-architecture milestones rewrite, so it goes first to avoid porting the
same code twice.

- Upgrade `bubbletea`, `bubbles`, and `lipgloss` to v2
- Port key handling, cursor, and any changed message/command signatures
- Confirm the existing suite passes and the TUI renders unchanged against real git and
  jj repos
- Ship: no behavior change, v2 on `main`, green CI

### M2: Test foundation (shipped)

Raise the floor before building new surfaces on top of it.

- Direct `Update()` tests for each message type; lift `app`, `github`, and `vcs`
  coverage toward the level of `models`/`filters`
- Boundary tests (empty lists, cursor clamping on filter, window resize)
- Add the thin teatest golden layer for the stable screens under the `golden` build tag
- Ship: coverage targets met, golden snapshots committed, CI runs both tag sets

### M3: Command mode (shipped)

First vim slice and the seam the rest depends on. An in-repo command layer, no
external package yet.

- `:command` input line with a small command registry and auto-completion
- Wire a starter set to existing behavior (`:filter`, `:sort`, `:fetch`, `:refresh`)
  alongside the current keybindings (both work)
- Ship: command mode usable for the common filter/fetch flows

### M4: Predicates (shipped)

- Predicate parser for `dirty and has_pr`, `behind or ahead`, etc.
- `:filter <predicate>` and `:select where <predicate>` over the repo set
- Ship: compositional filtering by expression, no visual mode needed

### M5: Text objects and operators (shipped)

- Text objects: `dr` (dirty), `pr` (has PR), `br` (behind), `ar` (all)
- Operators: `F` (fetch), `P` (prune), `C` (cleanup)
- Composition: operator × text object (`Fdr`, `Cpr`) plus a keybinding layer
- Ship: composable batch actions scoped by text object

### M6: Fixture-based tests and docs (shipped)

Couples to command mode: fixtures script command sequences and generate usage docs.

- Fixture format and harness replaying command sequences against serializable state
- Auto-generate usage docs from fixtures (keyboard and command mode) so examples
  cannot go stale
- Serializable `AppState`/`UIState` split if not already forced by earlier milestones
- Optional: extract `tui-commander` once a second TUI (for example jj-diff) shares it
- Ship: fixtures cover the core workflows, docs regenerate from them

### M7: vcs.Operations split (shipped)

Small, low-risk refactor that keeps the interface from growing further when M9 adds
mutator methods. Two independent simplifications:

- Drop the four file-status count methods (`GetStagedCount`, `GetUnstagedCount`,
  `GetUntrackedCount`, `GetConflictedCount`) from the interface entirely. They have
  no callers outside `vcs` (git's `GetRepoSummary` already uses the internal
  `getStatusCounts` helper); keep whatever concrete methods the exec tests exercise,
  remove the four `Fn` fields from `MockOperations`. 18 methods becomes 14
- Split the remainder into embedded sub-interfaces so `Operations` stays a composite
  and no caller changes are forced: `StatusReader` (summary-level queries:
  `GetRepoSummary`, `GetCurrentBranch`, `GetUpstream`, `GetAheadBehind`,
  `CompareBranches`, `GetLastModified`, `GetRemoteURL`, `VCSType`), `DetailReader`
  (`GetBranchList`, `GetStashList`, `GetWorktreeList`, `GetCommitLog`), and `Mutator`
  (`FetchAll`, `PruneRemote`, `CleanupMergedBranches`). Each stays under the
  interfacebloat threshold, so the `//nolint:interfacebloat` suppression goes away
- Narrow the consumers that only need a slice: `batch.TaskFunc` takes `vcs.Mutator`
  (write tasks) and the `cli` package's summary collection takes the readers. The
  factory keeps returning `Operations`
- Ship: same behavior, lint suppression removed, `mise run ci` green

### M8: Surface per-repo notes (doing.txt)

Answer "what was I doing here?" from the repo list without drilling in. Notes are
plain files the user leaves at a repo root; detection is a cheap stat plus
first-line read, independent of the VCS layer (no `Operations` growth).

- Detect a small default filename list at the repo root during summary load:
  `.doing`, `doing.md`, `doing.txt`, `TODO.md` (first match wins; make the list
  configurable later only if needed). Add `NotesFile` and `NotesFirstLine` to
  `RepoSummary`
- Repo list: compact badge in the Status column (consistent with existing badge
  styling); no dedicated column, keeping the list uncluttered
- Repo detail: a Notes tab rendering the file content alongside the existing
  branches/stashes/worktrees/PRs tabs
- Filtering: `HAS_NOTES` filter mode, `has_notes` predicate, and an `nr` text object
  so `:filter dirty and has_notes` and operator composition work
- `--cli` JSON gains `notes_file` / `notes_first_line`
- Tests: filter table tests, a fixture for the filter/predicate, golden update for
  the detail tab
- Ship: badge visible, tab renders, filterable by predicate

### M9: Safe-to-delete branch detection (gh-poi equivalent) (shipped)

`CleanupMergedBranches` only caught true merges (`git branch --merged`), so
squash-merged branches survived forever. Rather than shelling out to the gh-poi
extension (human-readable output only, extra install) or importing its unstable
internals, reimplemented the core detection with the `gh --json` pattern the
`internal/github` package already uses. PR data is VCS-agnostic, so this works
for jj bookmarks too.

- `internal/github.GetMergedPRHeads` fetches merged PR head refs
  (`gh pr list --state merged --json headRefName,headRefOid --limit 100`), cached
  like the existing PR lookups
- Added a `Head` tip-OID field to `models.BranchInfo` (both git `for-each-ref` and
  jj bookmark-list template implementations); a branch is safe to delete when its
  tip matches a merged PR's `headRefOid`
- Fixed default-branch detection: `GitOperations.resolveDefaultBranch` tries `git
  symbolic-ref refs/remotes/origin/HEAD` first, falling back to the previous
  main/master probing
- Safety rails: squash-merged deletion skips the current branch and any branch
  checked out in a worktree; a gh-poi-style lock/pin list stayed out of scope
- Reworked `vcs.Mutator.CleanupMergedBranches` to `(ctx, repoPath, squashMerged
  []string) (bool, string, error)`. Git deletes true-merges with `-d` and the
  caller-verified squash-merged set with `-D`, collecting per-branch failures
  into the result message (`"Deleted N branches: ...; failed: name (reason)"`)
  instead of silently ignoring them. jj deletes matching bookmarks the same way
  via `jj bookmark delete`, since it doesn't distinguish a true merge from a
  squash merge
- `batch.CleanupMerged` computes the squash-merged set itself (merged PR heads
  intersected with local branch tips) via swappable `getMergedPRHeads`/
  `getOperations` seams so tests can stub gh/git access; `batch.PreviewCleanup`
  backs `:cleanup --dry-run`, reporting the same detection without deleting
- UI: the repo detail Branches tab marks deletable branches with a green
  "merged" badge, computed best-effort when the tab loads (missing `gh` yields
  no annotations rather than failing the load)
- Ship: squash-merged branches detected and cleanable (git and jj), dry-run
  preview via `:cleanup --dry-run`, per-branch failure reporting
- Deferred: the `has_deletable` filter predicate (would need per-repo gh calls
  at summary-load time, which the roadmap explicitly avoids) and a jj-specific
  default-branch resolver (jj cleanup still assumes `main`/`master`/`trunk`,
  matching pre-M9 behavior)

### M10: Code-health quick wins and view split (shipped)

From the 2026-07-05 code-health survey. Two commits: cleanups, then pure file moves.

- `withSelection(style, selected)` helper replacing the ~10 repetitions of
  `if selected { style = style.Background(styles.Surface0) }` in `view.go`
- One shared definition of default-branch names (`vcs` consts, jj's
  `isProtectedBookmark` with its extra `trunk`, and app's `findDefaultBranch`
  currently disagree); git-side cleanup should protect the same set jj does
- Cache registry: `NewTTLCache` self-registers so `cache.ClearAll` stops
  hand-listing every cache (a missed entry silently breaks refresh)
- Refactor `CycleSortState` (gocognit 26, the last complexity-lint outlier)
- File moves: split `view.go` (~1750 lines) along view-mode seams into
  `view_repolist.go`, `view_detail.go`, `view_modals.go`, `view_branchdetail.go`,
  `view_prdetail.go`; move the `tea.Cmd` constructors from `update.go` into
  `commands.go`
- Ship: no behavior change, golden snapshots byte-identical, lint baseline shrinks

### M11: Config file (shipped)

TOML config at the XDG path (`~/.config/gh-repo-dashboard/config.toml`), flags
taking precedence over config over defaults. Saved scan paths are the headline:
launching from anywhere without retyping paths.

- New `internal/config` package (pelletier/go-toml/v2): `scan_paths` (with `~`
  expansion), `depth`, `notes_filenames`, `cache_ttl_minutes`; missing file is
  the zero config, malformed file is a hard error so typos aren't silently
  ignored
- Wired into `main.go` for both TUI and `--cli` modes via startup hooks
  (`models.SetNotesFilenames`, `cache.SetAllTTLs`); an explicitly set `-depth`
  flag wins over the config value
- Ship: config discovered and applied, flags still win, no config file required

### M12: Scripts and history (shipped)

The parked recording/script cluster, cheap-first:

- `:history` shows recent commands newest-first in the status bar; `@:` repeats
  the last one. History is recorded in `ExecuteCommand` (capped at 50) so the
  command bar, scripts, and repeats all share it
- `--cli --filter '<predicate>'` narrows JSON output via the predicate parser;
  PR data is attached before the predicate runs so `has_pr` works from cache
- `--script file.txt` (or `-` for stdin) loads summaries synchronously, then
  executes `:command` lines headlessly, printing status messages and per-repo
  batch results; `quit` stops early, `#` comments and blanks are skipped
- Macro registers (`:record @a` / `:replay @a`) stay parked
- Ship: scripted batch flows work headlessly; history/repeat in the TUI

## Deferred features

Low priority; slot between milestones when convenient rather than blocking the line
above.

- Full Catppuccin themes replacing the current textual themes
- Deep-DRY items from the code-health survey, to do opportunistically when work
  next touches these files: a shared repo-enrichment path for `cli.loadRepo` and
  app's summary/detail loading (same GetRepoSummary → worktrees → DetectNotes →
  PR-lookup sequence, differing only in cache policy), and guard/update helpers
  for the five `*LoadedMsg` handlers repeating the selected-repo check and
  summary read-modify-write
- Surface deletable-branch counts in the repo list from cache-resident data and a
  `has_deletable` predicate (M9 leftovers), plus gh-poi-style branch pinning
- `internal/app`'s 11 test files stay whitebox (`package app`, `//nolint:testpackage`)
  rather than converting to `app_test`. `Model` has 35+ unexported fields that tests
  construct and inspect directly across hundreds of call sites (`app_test.go`,
  `command_test.go`, `update_test.go`, etc.); a blackbox conversion would mean
  exporting most of `Model`'s internals via `export_test.go`, eroding encapsulation
  for state that's intentionally private. Revisit only if these tests are rewritten
  to drive `Model` through its exported `Update`/command-mode surface instead of
  direct field access

## Parked ideas

Captured from earlier planning, not yet on the line. Most of the recording/script
cluster moved onto the line as M12; what remains parked:

- Macro registers (`:record @a` / `:replay @a`); persistence and
  record-while-recording edge cases cost more than scripts deliver
- Watch/auto-refresh mode
