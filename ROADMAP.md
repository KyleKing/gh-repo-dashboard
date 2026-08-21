# Roadmap

Where gh-repo-dashboard is going, and the shape of what it already has.
Architecture and domain context live in [DESIGN.md](DESIGN.md); Go and workflow
conventions live in [AGENTS.md](AGENTS.md); the design docs the shipped work was
built from live in `docs/design/`, alongside
[selection-and-verbs.md](docs/design/selection-and-verbs.md), which is proposed
rather than shipped.

The column engine lives in `internal/ui/table` and every table is sized by it.
The repo list is one full-width table with an expanded region `v` opens beneath
it (`internal/app/view_repolist.go`). The focused repo view is a panel grid
(`panels.go`, `view_panels.go`) and the universal find is `palette.go` and
`view_palette.go`. The `:prs` fleet map lives in `internal/app/prmap.go`.
Default-branch CI is fetched lazily for visible rows only, tracked by
`Model.ciRequested`.

## Vision

Grow the dashboard into a composable, vim-paradigm TUI: text objects (dirty repos,
repos with PRs), operators (fetch, prune, cleanup), and composition (`Fdr` fetches
dirty repos) driven either by keys or a `:command` mode with predicates
(`:filter dirty and has_pr`). The composable command layer is also the seam that
makes behavior testable without keyboard simulation, documentable from fixtures,
and scriptable headlessly.

The 2026-08 direction adds a second axis: the same data at two densities. A
compact narrow layout with its own UX, and the standard table, which keeps
growing into whatever width the terminal has, with a focused single-repo view
that answers "what is the state of the repo I am standing in" without any
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

M1 through M20 landed through 2026-08-05 and are not tracked here; `CHANGELOG.md`
and `git log` are the record. The lists below are the whole backlog.

## Migrating `internal/ui` to aragonite

`cache` and `forge` already moved. The rest of the shared rendering follows in
dependency order, one package per change, each landing in this repo first and
moving only once it has no import back into `internal/`.

| Phase | Package | Becomes | Blocked on |
| --- | --- | --- | --- |
| 1 | `internal/ui/table` | `tui/table` | nothing; depends only on lipgloss and uniseg |
| 2 | `internal/ui/region` | `tui/region` | phase 1, since it renders through `table` |
| 3 | `internal/ui/styles` | `tui/theme` | splitting the palette from the app's named styles |
| 4 | `internal/ui/markdown` | `tui/markdown` | phase 3, plus taking its styles as an argument |
| 5 | `internal/ui` (display.go) | `forge` | nothing; it renders `forge` types and imports no lipgloss |

Phase 3 is the one with a real decision in it. `styles.go` is a Catppuccin
Macchiato palette and roughly thirty-five named styles. The palette is generic and
the names are this app's vocabulary (`PROpenStyle`, `NotesBadgeStyle`), so only the
palette and the handful of structural styles move, and the app keeps the rest.

Phase 5 is a rename more than a move: `display.go` emits plain strings for `forge`
types and pulls in no rendering library at all, so it is `forge` presentation
sitting in a `ui` package rather than anything terminal-specific.

Two conventions carry over from `docs/extraction.md`: split on consumer coupling
rather than on file boundaries, and promote whatever the tests need into real API
instead of leaving it in `export_test.go`. Test cleanup lands with each phase, in
aragonite, rather than as a sweep afterward.

## Deferred features

Low priority; pick up when convenient.

- The copier template update from my_go_template v0.9.1 to v0.11.3. The `ci`
  job's GOROOT fix (`d41265c`) lives in a template-managed file, and mise and
  `actions/setup-go` disagree about `GOROOT` when both run in one job, which
  goes red the moment mise's `go = "latest"` resolves past `go.mod`. Confirm the
  newer template carries the fix before accepting its `ci.yml`
- Full Catppuccin themes replacing the current textual themes
- Motion policy: the app has zero animation (static discovery text, static
  batch gauge). Decide deliberate restraint or add a spinner
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
- Shell into a repo's directory from the dashboard, keyed off the row under
  the cursor. The interaction model is still open: suspend the TUI, spawn a
  one-shot subshell in the repo path, and restore the dashboard on exit
  (simplest, matches how `git log`/`less` already hand off the terminal), or
  hand off to a persistent per-repo session (a tmux pane or window) that
  outlives the dashboard and that repeated shells jump back into. Decide once
  the one-shot path ships and the persistent case actually comes up
- Absorb mani: gh-repo-dashboard already owns the repo list, so let it also
  define and run the batch operations mani exists for (fetch, tag, and
  whatever else mani's config currently drives across these repos), removing
  the need to keep the two tools' repo lists in sync. Fits the vim-paradigm
  operator model in the Vision section above: `fetch` becomes an operator
  over the repos a predicate selects, the same shape as the planned `Fdr`.
  Scope it against what mani's config for this repo set actually uses today
  before generalizing past that
- A vcs-doctor view surfacing local git/jj config that has drifted from
  global defaults, so a repo behaving oddly is diagnosable from the
  dashboard instead of by hand-checking config files. Candidates:
  `.git/info/exclude` entries not in the global excludes file, `rerere`
  disabled locally when global has it on (already surfaced in today's
  summary), and any other per-repo override worth flagging (hooks,
  divergent remote settings). Needs a decision on scope: read-only
  diagnostic panel first, with a "fix" operator only if the diagnostics
  prove useful enough to act on

## Parked ideas

Captured from earlier planning, not on the line:

- Macro registers (`:record @a` / `:replay @a`); persistence and
  record-while-recording edge cases cost more than scripts deliver
- Watch/auto-refresh mode
- Claude session awareness (badging repos with recent agent sessions);
  revisit if parallel-agent workflows make it earn a column
