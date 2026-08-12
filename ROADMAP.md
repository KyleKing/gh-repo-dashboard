# Roadmap

Where gh-repo-dashboard is going, and the shape of what it already has.
Architecture and domain context live in [DESIGN.md](DESIGN.md); Go and workflow
conventions live in [AGENTS.md](AGENTS.md); the design docs the shipped work was
built from live in `docs/design/`.

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

The 2026-08 direction adds a second axis: the same data at three densities.
A compact narrow layout with its own UX, the standard table, and a wide layout
that spends surplus width on the table's own columns, with a focused single-repo
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

M1 through M20 landed through 2026-08-05 and are not tracked here; `CHANGELOG.md`
and `git log` are the record. The lists below are the whole backlog.

## Deferred features

Low priority; pick up when convenient.

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

## Parked ideas

Captured from earlier planning, not on the line:

- Macro registers (`:record @a` / `:replay @a`); persistence and
  record-while-recording edge cases cost more than scripts deliver
- Watch/auto-refresh mode
- Claude session awareness (badging repos with recent agent sessions);
  revisit if parallel-agent workflows make it earn a column
