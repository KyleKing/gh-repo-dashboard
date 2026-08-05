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

M1 through M19 landed through 2026-08-05 and are not tracked here; `CHANGELOG.md`
and `git log` are the record. M20 and the lists below are the whole backlog.

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
