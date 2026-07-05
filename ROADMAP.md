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

## Known issues (found during the M2 test audit)

Real bugs surfaced while writing tests; tests currently assert the existing
behavior so fixes are visible diffs. Fix opportunistically between milestones.

- git `GetBranchList` silently drops the last branch when it has no upstream
  (output trimming eats the trailing tab, leaving too few fields)
- `Model.compareToDefaultBranch` (view.go) diffs `m.branchDetail.Commits` against
  itself instead of the default branch's commit log, so `ahead` is always 0 and
  `behind` is hardcoded to 0; the branch detail's ahead/behind-of-default display
  is meaningless. Found during the 2026-07 lint cleanup's complexity refactor;
  preserved as-is since fixing it was out of scope for that pass

## Deferred features

Low priority; slot between milestones when convenient rather than blocking the line
above.

- Full Catppuccin themes replacing the current textual themes
- `--cli` flag for non-interactive JSON output, cache-only by default (fresh retrieval
  only on request) for performance
- gh-poi integration to identify safe-to-delete branches
- Split `vcs.Operations` (18 methods, `//nolint:interfacebloat` suppressed for now)
  into narrower interfaces, e.g. a query-only reader (`GetRepoSummary`,
  `GetBranchList`, `GetStashList`, `GetWorktreeList`, `GetCommitLog`, etc.) and a
  mutator (`FetchAll`, `PruneRemote`, `CleanupMergedBranches`). Deferred because it
  would ripple through every caller (`app`, `vcs/git.go`, `vcs/jj.go`, tests) for a
  cosmetic lint fix rather than a real consumer boundary need
- `internal/app`'s 11 test files stay whitebox (`package app`, `//nolint:testpackage`)
  rather than converting to `app_test`. `Model` has 35+ unexported fields that tests
  construct and inspect directly across hundreds of call sites (`app_test.go`,
  `command_test.go`, `update_test.go`, etc.); a blackbox conversion would mean
  exporting most of `Model`'s internals via `export_test.go`, eroding encapsulation
  for state that's intentionally private. Revisit only if these tests are rewritten
  to drive `Model` through its exported `Update`/command-mode surface instead of
  direct field access

## Parked ideas

Captured from earlier planning, not yet on the line:

- Command recording and replay (`:record` / `:replay`), macro registers (`@a`)
- Script execution (`gh-repo-dashboard --script commands.txt`)
- gh CLI subcommands backed by text objects (`gh repo fetch-dirty ~/Developer`)
