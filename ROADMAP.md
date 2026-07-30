# Roadmap

Phased, shippable milestones for gh-repo-dashboard. Each milestone stands on its
own and can be released independently; the roadmap can stop at any point without
leaving the app half-migrated.

Architecture and domain context live in [DESIGN.md](DESIGN.md); Go and workflow
conventions live in [AGENTS.md](AGENTS.md).

## Vision

Grow the dashboard into a composable, vim-paradigm TUI: text objects (dirty repos,
repos with PRs), operators (fetch, prune, cleanup), and composition (`Fdr` fetches
dirty repos) driven either by keys or a `:command` mode with predicates
(`:filter dirty and has_pr`). The composable command layer is also the seam that
makes behavior testable without keyboard simulation, documentable from fixtures,
and scriptable headlessly.

The external `tui-commander` package extraction is deferred until a second TUI
actually shares the code (extract on second use).

## Testing strategy

A layered pyramid:

- Direct state-transition tests are the base layer: fast, dependency-free, and
  where most new tests land
- teatest golden files are a thin regression layer over a few stable screens (repo
  list, filter modal, detail, batch progress). Kept deliberately small so snapshots
  do not go brittle. Run under a build tag (`go test -tags=golden ./...`, `-update`
  to refresh)
- Fixture-based tests (`internal/app/testdata/fixtures/*.fix`) script command
  sequences and generate `docs/USAGE.md` (`mise run docs:usage`);
  `TestUsageDocsCurrent` fails CI when the docs go stale

M1 through M12 landed through 2026-07-05 and are not tracked here. `CHANGELOG.md`
and `git log` are the record.

## Proposed: fleet assessment for the freshen workflow

The freshen skill (multi-repo maintenance in `~/.claude/skills/freshen/`) needs a
per-repo snapshot before it dispatches work: local git state, CI conclusions on
the default branch, and where the repo sits relative to its copier template. Today
that lives in a bundled shell script (`assess.sh`) that shells out to git, gh, yq,
and jq per repo. `--cli` already emits the local half of that snapshot as JSON, so
the script duplicates repo discovery and status logic this codebase already has.

Options considered:

1. Extend `--cli`. The gaps are additive fields on the existing `Repo` struct, and
   the CI fetch already exists in `internal/github/workflow.go` (TUI-only today)
2. A dedicated fleet tool. Rejected: it would re-implement discovery, status, and
   PR enrichment for one consumer
3. Keep the shell script. Works, but every field it computes outside `--cli` is
   untested and unfilterable

Decision: extend `--cli` with three additions, then retire the script:

- CI status per repo, keyed by the default branch head rather than a commit SHA.
  Latest conclusion per workflow with timestamps, plus failing step names when
  red, so the caller can triage without a second round-trip
- Copier awareness: parse `.copier-answers.yml` into `template_src` and
  `template_version`, and let a predicate express drift
  (`template_version != latest`)
- Roster input: accept mani.yaml as a scan source alongside the configured scan
  paths, so the fleet definition stays in one file

Sketch of the added JSON, on the existing `Repo` shape:

```json
{
  "ci": [{"workflow": "CI", "conclusion": "failure", "failed_steps": ["lint"],
          "completed_at": "..."}],
  "template_src": "gh:KyleKing/my_go_template",
  "template_version": "v0.4.3"
}
```

Out of scope: waiting or polling. The snapshot stays one-shot; the caller owns
retry cadence. Batch mutations (fetch, prune, cleanup) already exist behind the
TUI and are not part of assessment.

Two gaps the proposal above does not cover, both found by reading
`~/.claude/skills/freshen/scripts/assess.sh` against it. Close them before
retiring the script:

- `assess.sh:23` runs `git -C "$dir" fetch --quiet` before it counts
  ahead/behind, so its counts are against freshly updated remote refs. `--cli` is
  read-only and never fetches, so a straight swap silently changes the meaning of
  `ahead`/`behind` to "against whatever `origin/<branch>` was last time you
  fetched". Either add an opt-in fetch to `--cli` or state the difference where
  callers will see it
- `assess.sh:42` also emits `dependabot_alerts`, open counts grouped by
  `security_advisory.severity` from `repos/{slug}/dependabot/alerts`, with a
  fallback to `{}` when the endpoint denies access (archived repos). The three
  additions listed above do not mention it, so a `--cli` that shipped exactly
  them would still leave the caller making a second round-trip

## Proposed: TUI polish from the 2026-07 critique

From a `tui-critique` pass over 63 real repos. Every item below was re-verified
against the source on 2026-07-27 and is still unfixed. Ordered by severity.

- P1 `statusColWidth` is a hardcoded `12` (`internal/app/view.go:25`) and
  `renderTableRow` pads with `fmt.Sprintf("%-*s", statusTextWidth, status)`
  (`view_repolist.go:369`), which pads short statuses but never truncates long
  ones. Name and branch cells do call `truncate()`; status is the one column
  without that safety net, so any status over 12 characters (`+1 ~10 ?2 ↑1N` is
  14) desyncs every column to its right for that row. Call `truncate()` on
  `status`, or size the column from the real max width
- P1 `NO_COLOR` is not implemented: no reference to it anywhere in `internal/`,
  and `NO_COLOR=1` output is identical to a full-color run. Status meaning is
  color-only in practice. Likely a lipgloss/termenv color-profile configuration
  fix rather than a redesign, since the `+`/`↑`/`↓`/`?` glyphs already carry the
  meaning
- P2 Command mode has no discoverability. A bare `:` prompt shows no completion
  list, no argument hint, and no footer change, and the `?` help screen never
  mentions `:`, `@:` repeat, or text objects despite `command.go` and
  `textobject.go` being first-class systems
- P2 The empty state is a bare `"No repositories found"`
  (`view_repolist.go:138`) with no next step. Name the path and depth that were
  scanned and point at `--depth`
- P3 Narrow terminals silently clip the PR/PRs/Template/Modified columns and part
  of the footer's own `? help q quit` hints, with no truncation indicator and no
  minimum-size gate. Priority-collapse the data columns before the footer hints
- P3 optional: the app has zero motion anywhere (static
  `"Discovering repositories..."`, a static-fill batch gauge). Deliberate
  restraint or unaddressed is undecided; settle that before treating a spinner as
  a backlog item

Open questions the critique raised that Kyle has not answered: whether both P1s
are wanted now or one can wait, whether command-mode discoverability should be a
footer hint or a help-screen line or both, and whether 80x24 is a support target
at all (DESIGN.md states no minimum either way).

## Deferred features

Low priority; pick up when convenient.

- Full Catppuccin themes replacing the current textual themes
- Deep-DRY items from the code-health survey, to do opportunistically when work
  next touches these files: a shared repo-enrichment path for `cli.loadRepo`,
  `app.newScriptModel`, and app's summary/detail loading (same GetRepoSummary →
  worktrees → DetectNotes → PR-lookup sequence, differing only in cache policy),
  and guard/update helpers for the five `*LoadedMsg` handlers repeating the
  selected-repo check and summary read-modify-write
- Surface deletable-branch counts in the repo list from cache-resident data and a
  `has_deletable` predicate (M9 leftovers), plus gh-poi-style branch pinning
- `internal/app`'s test files stay whitebox (`package app`,
  `//nolint:testpackage`) rather than converting to `app_test`. `Model` has 35+
  unexported fields that tests construct and inspect directly across hundreds of
  call sites; a blackbox conversion would mean exporting most of `Model`'s
  internals via `export_test.go`, eroding encapsulation for state that's
  intentionally private. Revisit only if these tests are rewritten to drive
  `Model` through its exported `Update`/command-mode surface instead of direct
  field access

## Parked ideas

Captured from earlier planning, not on the line:

- Macro registers (`:record @a` / `:replay @a`); persistence and
  record-while-recording edge cases cost more than scripts deliver
- Watch/auto-refresh mode
