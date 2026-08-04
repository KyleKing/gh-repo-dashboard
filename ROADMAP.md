# Roadmap

Phased, shippable milestones for gh-repo-dashboard. Each milestone stands on its
own and can be released independently; the roadmap can stop at any point without
leaving the app half-migrated.

Architecture and domain context live in [DESIGN.md](DESIGN.md); Go and workflow
conventions live in [AGENTS.md](AGENTS.md). Design docs backing the milestones
below live in `docs/design/`:

- [fleet-navigation.md](docs/design/fleet-navigation.md), peers, the PR-to-local
  map, PR flows, CI, and the API budget (M16, M17)

The column engine lives in `internal/ui/table` and every table is sized by it.
`internal/app/view_overview.go` holds the repo overview pane, mounted both by
the wide layout's preview panel and by the focused repo view. Scenes live in
`internal/app/scene.go` and are derived from the active tab, so other views can
adopt the same pattern by defining their own scene list.

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

M1 through M15 landed through 2026-08-04 and are not tracked here. `CHANGELOG.md`
and `git log` are the record.

## Sequence at a glance

| Milestone | Theme | Depends on |
|-----------|-------|------------|
| M16 | Peers panel, same-branch conflicts, PR-to-local map | — |
| M17 | PR activity, PR flows, CI on the default branch | M16 |
| M18 | `--cli` fleet assessment (assess.sh replacement) | shares the CI fetch with M17 |

## M16: peers and the PR-to-local map

The local-data half of [fleet-navigation.md](docs/design/fleet-navigation.md).
Zero API cost.

- `:prs` fleet map: open PRs joined against cached local branch lists (which
  checkout holds the head ref), plus local branches with commits and no open
  PR

Exit criteria: two checkouts on one branch are flagged at list, panel, and
header level; the map answers "where is the branch for PR #N" and "which local
branches have no PR" for the real fleet without new gh calls beyond the
existing per-repo PR list.

## M17: PR flows and CI

The API-spending half of [fleet-navigation.md](docs/design/fleet-navigation.md),
governed by its budget rules (one call per repo, GraphQL batching, TTL caches,
lazy fetch, staleness shown as age suffixes).

- ACTIVITY column (latest comment or review event: age plus author) in the PR
  tab and fleet map, absorbing the mostly empty REVIEW width
- `g` checks a PR branch out locally, confirm-gated and peer-aware like `c`
- Batch PR refresh behind the `F+obj` operator pattern
- CI on the default branch as a repo-list column, wide-panel line, and
  focused-header badge

Exit criteria: a full-fleet refresh of PR and CI data stays within tens of
GraphQL points; cells render cache age when stale; no scroll or scene switch
triggers a fetch for repos not visible.

## M18: fleet assessment for the freshen workflow

Extend `--cli` so the freshen skill can retire `assess.sh`. Unchanged from the
earlier proposal; M17's CI fetch is the shared implementation.

- CI status per repo keyed by the default branch head: latest conclusion per
  workflow with timestamps, plus failing step names when red
- Copier awareness: `template_src` and `template_version` fields and a drift
  predicate (`template_version != latest`)
- Roster input: accept mani.yaml as a scan source alongside configured paths

Known gaps to close before retiring the script (from reading `assess.sh`
against this proposal): the script fetches before counting ahead/behind, so
`--cli` needs an opt-in fetch or a documented difference; and the script emits
`dependabot_alerts` (open counts by severity, `{}` on denied access), which the
three additions above do not cover. `NEXT_STEPS.md` records further
verification notes: `is_template` and `has_freshen_txt`, the
`WorkflowSummary` shape mismatch, and the undefined meaning of `--fresh` for
non-PR data.

Out of scope: waiting or polling. The snapshot stays one-shot; the caller owns
retry cadence.

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
