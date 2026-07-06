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

## Shipped

Twelve milestones landed through 2026-07-05; full detail lives in git history and
the commit messages referenced below.

- M1 Bubble Tea v2 upgrade; M2 test foundation (state-transition tests, golden
  layer); M3 `:command` mode with registry and completion; M4 predicate parser
  (`:filter dirty and has_pr`); M5 text objects and operators (`Fdr`, `Cpr`);
  M6 fixture-based tests generating `docs/USAGE.md`
- M7 `vcs.Operations` split into `StatusReader`/`DetailReader`/`Mutator`
  composites (6959fd3)
- M8 per-repo notes surfacing: `.doing`/`doing.md`/`doing.txt`/`TODO.md` badge,
  detail tab, `has_notes` filter and `nr` text object (537d79f)
- M9 safe-to-delete branch detection: merged PR heads via `gh pr list --json`
  compared against branch tip OIDs, `:cleanup --dry-run`, squash-merge-aware
  cleanup with per-branch failure reporting, `origin/HEAD` default-branch
  resolution (75e2d6d)
- M10 code-health quick wins (`withSelection`, `vcs.IsDefaultBranchName`, cache
  registry, `CycleSortState`) and the view.go split by view mode (1e27ac3,
  0207985)
- M11 TOML config at `$XDG_CONFIG_HOME/gh-repo-dashboard/config.toml`: saved
  scan paths, depth, notes filenames, cache TTLs; flags win (d4a0f51)
- M12 `:history` and `@:` repeat, `--script` headless command runner,
  `--cli --filter <predicate>` (79ba9cc)

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
  `has_deletable` predicate (M9 leftovers), plus gh-poi-style branch pinning; a
  jj-specific default-branch resolver (jj cleanup still assumes
  main/master/trunk)
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
