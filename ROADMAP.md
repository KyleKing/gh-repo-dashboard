# Roadmap

Where gh-repo-dashboard is going, and the shape of what it already has.
Architecture and domain context live in [DESIGN.md](DESIGN.md); Go and workflow
conventions live in [AGENTS.md](AGENTS.md); the design docs the shipped work was
built from live in `docs/design/`.

The column engine lives in `internal/ui/table` and every table is sized by it.
`internal/app/view_overview.go` holds the repo overview pane, mounted both by
the wide layout's preview panel and by the focused repo view. Scenes live in
`internal/app/scene.go` and are derived from the active tab, so other views can
adopt the same pattern by defining their own scene list. The `:prs` fleet map
lives in `internal/app/prmap.go`. Default-branch CI is fetched lazily for
visible rows only, tracked by `Model.ciRequested`.

## Vision

Grow the dashboard into a composable, vim-paradigm TUI: text objects (dirty repos,
repos with PRs), operators (fetch, prune, cleanup), and composition (`Fdr` fetches
dirty repos) driven either by keys or a `:command` mode with predicates
(`:filter dirty and has_pr`). The composable command layer is also the seam that
makes behavior testable without keyboard simulation, documentable from fixtures,
and scriptable headlessly.

The 2026-08 direction adds a second axis: the same data at three densities.
A compact narrow layout with its own UX, the standard table, and a wide layout
that spends surplus width on a live preview panel, with a focused single-repo
view that answers "what is the state of the repo I am standing in" without any
drilling.

The external `tui-commander` package extraction is deferred until a second TUI
actually shares the code (extract on second use).

## Testing strategy

A layered pyramid:

- Direct state-transition tests are the base layer: fast, dependency-free, and
  where most new tests land
- teatest golden files are a thin regression layer over a few stable screens.
  The golden set is per breakpoint (80x24, 120x35, 220x50) rather than per view
  at one size. Run under a build tag (`go test -tags=golden ./...`, `-update`
  to refresh)
- Fixture-based tests (`internal/app/testdata/fixtures/*.fix`) script command
  sequences and generate `docs/USAGE.md` (`mise run docs:usage`);
  `TestUsageDocsCurrent` fails CI when the docs go stale

M1 through M18 landed through 2026-08-04 and are not tracked here; `CHANGELOG.md`
and `git log` are the record. M19, M20, and the lists below are the whole
backlog; M19 ships first because M20 rebuilds the view M19's fixes land in.

## M19: correctness fixes from the 2026-08-04 review

A PTY review of `bbc6cdb` (220x50, 80x24, live resizes, a sandbox fleet of
hostile repos plus the real fleet) verified each of these in source. Ordered by
severity; the cleanup trio comes first because it is the only destructive one.

### Cleanup safety (do first)

- Batch cleanup runs without confirmation: `Car` deleted-branch-scanned all 8
  sandbox repos instantly. Single-repo writes (`c`, `p`, `M`) gate on
  `ViewModeConfirm`; the operator and `:cleanup` paths call `runBatchCommand`
  (`internal/app/command.go`) straight into the task. Add the same gate,
  naming the repo count
- `mergedBranchNames` (`internal/vcs/git.go:552`) strips `* ` from
  `git branch --merged` output, so the checked-out branch becomes a deletion
  candidate (observed: "Failed to delete … feat/…" on the current branch), and
  a detached HEAD emits the literal `(HEAD detached at 85d16a3)` line as a
  branch name passed to `git branch -d`. The `+ ` prefix (checked out in
  another worktree) is not stripped either. Parse with
  `for-each-ref refs/heads --merged` or filter the porcelain markers
- `CleanupMergedBranches` returns `true` even when every deletion failed, so
  batch rows render `✓` beside "Failed to delete" (`internal/vcs/git.go`,
  final return). Success must reflect the failed list

### Data correctness

- `strings.TrimSpace` on all git output (`internal/vcs/exec.go:34`) eats the
  leading space of the first `git status --porcelain -z` entry, so an
  unstaged ` M` file that sorts first is counted as staged. A repo with
  1 staged, 1 unstaged, and 1 untracked file renders `+2 ?1` and
  "2 staged · 1 untracked". Skews every status cell, `IsDirty`, and `-cli`
  JSON. Stop trimming porcelain output (or trim only trailing newlines)
- A failed CI fetch leaves the `…` placeholder forever:
  `handleWorkflowLoaded` (`internal/app/update.go:258`) ignores `msg.Error`,
  so archived repos (`dev-boards_archived`) and repos without a GitHub remote
  show a permanent spinner. Render `—` (or `?` with the error in a status
  line) on error
- `:prs` maps fork PRs to local branches by name alone: `locateBranch`
  (`internal/app/prmap.go:106`) matches `HeadRef` against local branch names,
  so mdit-py-plugins #63 and #124 (forks with head branch `master`) claim
  "here: master". Needs `headRepositoryOwner` from `gh pr list` in the join
- `:prs` double-counts a branch shared between a clone and its worktree
  (peer-beta and peer-beta-wt each contributed a `shared-branch` row); dedupe
  by repo identity, not checkout path

### Signal quality

- Same-branch conflict noise: `ConflictingBranches`
  (`internal/models/checkout.go:155`) has no default-branch special case, so
  backup clones sitting on `main` produce "⚠ 12 branch conflicts" on the real
  fleet and drown the lose-local-commits signal. Flag the default branch only
  when a conflicting checkout is dirty or ahead
- `:prs` local-only rows: 11 of "17 local-only branches" are `main` ahead of
  origin (`internal/app/prmap.go:88` skips only `Ahead == 0`). Exclude the
  default branch, or give it its own quieter section
- Header scope mixing: after a filter, "2/8 repos" and the conflict count
  follow the filter while "3 dirty" stays fleet-wide. Pick one scope for all
  header counts
- The compact two-line record drops the `⚠` that the wide PEERS cell renders
  after `⧉ 2`
- Peers KIND is viewpoint-relative: peer-beta-wt reads "worktree" from its
  parent and "clone" from peer-alpha; label from the checkout's own nature

### Copy and small UX

- "Sync in sync vs no upstream" (`internal/app/view_overview.go:198`) is
  contradictory; with no upstream say "no upstream", and for a repo with zero
  commits say "no commits"
- The breadcrumb labels an ahead-only clean repo "dirty" (`IsDirty` includes
  `Ahead > 0`); the badge should distinguish unpushed from uncommitted
- The maintain scene shows the jj-only stash hint ("Stashes are only
  available for git repositories") to git repos with zero stashes; jj's Files
  line says "unstaged" where jj has no staging area
- Detached HEAD is invisible inside the focused view: the list shows
  `(85d16a3)` but the branch tab marks no current branch and the header drops
  the commit
- Scenes `1`-`4` are missing from the `?` overlay; the review scene lacks the
  CHECKS column its design promised; the empty repo's compact record renders
  a bare `—` second line; PR descriptions clip mid-word; `⬆️` (emoji + VS16)
  still shifts a `:prs` row by one cell (width-measure edge case)

## M20: panel grid and universal find

The tabless focused view from
[focused-repo-view.md](docs/design/focused-repo-view.md), replacing both the
tab bar and the scene presets.

- lazygit-style grid: every data set is an always-visible panel showing real
  content when unfocused; the focused panel expands and a persistent detail
  pane renders the selected item (branch commits, PR description and latest
  comment, stash diffstat, note body)
- Relevance-driven sizing: panel height follows actionable state (dirty
  files, conflicted peers, active PRs) from cached data; clean sections
  compress to a line
- Universal find: `space` opens a typed-prefix palette (`#12` PR number,
  `b fix` branches, `s wip` stashes, `r dash` repos, bare text everything),
  scoped to the repo in the focused view and the fleet from the list. Result
  sets are actionable: `tab` marks, `!` opens type-appropriate verbs, and a
  repo set commits to the selected-repos text object so batch operators
  compose with it
- Tabs and scenes retire in the same milestone; number keys become panel
  jumps, scene fixtures are rewritten against panels, and jj drops the
  Stashes panel instead of hinting

Exit criteria: a busy repo arrives with its problems expanded and a quiet one
reads as one-liners plus detail; `space`, `#12`, `!` answers "which repos
have PR 12" and offers actions; no keypress is needed to see any section's
content; goldens cover the grid at all three breakpoints.

## Deferred features

Low priority; pick up when convenient.

- Full Catppuccin themes replacing the current textual themes
- Motion policy: the app has zero animation (static discovery text, static
  batch gauge). Decide deliberate restraint or add a spinner
- Notes preview shows the first line of the file, which is usually a date
  heading; skip headings and blanks so the peek carries content
- Deep-DRY items from the code-health survey, to do opportunistically when work
  next touches these files: a shared repo-enrichment path for `cli.loadRepo`,
  `app.newScriptModel`, and app's summary/detail loading, and guard/update
  helpers for the five `*LoadedMsg` handlers
- Surface deletable-branch counts in the repo list from cache-resident data and
  a `has_deletable` predicate (M9 leftovers), plus gh-poi-style branch pinning
- `internal/app`'s test files stay whitebox (`package app`,
  `//nolint:testpackage`); revisit only if tests are rewritten to drive `Model`
  through its exported surface
- Command-mode completion (a completion list under the `:` prompt); M13 ships
  discoverability hints only

## Parked ideas

Captured from earlier planning, not on the line:

- Macro registers (`:record @a` / `:replay @a`); persistence and
  record-while-recording edge cases cost more than scripts deliver
- Watch/auto-refresh mode
- Claude session awareness (badging repos with recent agent sessions);
  revisit if parallel-agent workflows make it earn a column
