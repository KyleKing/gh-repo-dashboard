# Next steps

Immediate follow-ups. Feature work and the TUI critique backlog live in
`ROADMAP.md`. Every item here was re-verified against the source on 2026-07-27;
each names a symbol rather than a line number, so grep for it.

## Correctness

`--cli` without `--fresh` can never return PR data. The cache is in-memory only,
so every process starts cold and `pr`/`pr_count` are always omitted; all 62 repos
returned null for both in the original review run, and nothing in the tree calls
`os.UserCacheDir`. Either persist the cache under `~/.cache/gh-repo-dashboard/`
with the existing TTL so the flag pair means what it reads like, or always fetch
in `--cli` mode and drop `--fresh`. A JSON consumer that silently omits PR data
is worse than a slow one. If both flags stay, `-h` should say that `--cli` alone
returns no PR data.

`:cleanup --dry-run` over-reports the checked-out branch. `mergedBranchNames` in
`internal/vcs/git.go` strips the `* ` marker and then keeps the branch, so the
preview promises a delete that `git branch -d` refuses. It over-reports and never
under-reports, so nothing vanishes unannounced, but the preview is not truthful.
Capture the `* ` prefix before stripping and skip that branch, matching the guard
`deleteSquashMerged` already applies. The live-test harness pins the current
behavior and will flip to a failure telling you to update it once this is fixed.

jj repos never get PR data at all. They report `branch: "@"` with no upstream, so
`lookupPR` and `lookupPRCount` in `internal/cli/cli.go` return nil on the empty
upstream. The README claims PR features work with both git and jj. Resolve the
working-copy bookmark name and use that instead, reusing the `trunk()` approach
`resolveDefaultBookmark` already uses in `internal/vcs/jj.go`.

## Robustness

A typo'd scan path is silently ignored: exit 0 with an empty repo list,
indistinguishable from a directory that genuinely holds no repos.
`resolveScanPaths` in `cmd/gh-repo-dashboard/main.go` never stats its positional
arguments. `os.Stat` each one up front and fail with a clear message, since this
is a user-input boundary.

`--script` still exits 0 on an unknown command or a bad predicate. `RunScript` in
`internal/app/script.go` only returns an error for an unreadable script or a
scanner failure; `runScriptLine` swallows per-line failures. A missing script file
does exit nonzero, so the two failure classes are inconsistent. Track whether any
line errored and return an error at the end.

## Discoverability

`printUsage` now carries a synopsis explaining that positional arguments are scan
paths and how config precedence works, so the bare `flag` dump is fixed. What is
left: `-h` never mentions that `~/.config/gh-repo-dashboard/config.toml` exists,
and there is still no headless way to list the `:commands` (you have to read
`DefaultRegistry()`). Make `:help` print the registry in `--script` mode.

## Coverage

Coverage is 70.0% against the 70% floor `test:coverage-min` enforces, so the next
uncovered branch fails the gate. Verified by running the task on 2026-07-27.

## Template

This repo is pinned at my_go_template v0.9.1, which is the template's current tag.
The `brew:sha` and CI-`hooks` items that sat here are resolved: v0.9.1 drops the
task, and `hk check --all` passes on this tree.

- `.pre-commit-config.yaml` is dead config here and running `prek run --all-files`
  actively corrupts the tree: no testdata exclude, so end-of-file-fixer and
  trailing-whitespace rewrite all five `.golden` fixtures, and mdformat plus
  prettier reformat every markdown file and workflow against the committed style.
  prek runs in neither CI nor the git hooks. Either delete the file or give it the
  same testdata excludes `hk.pkl` has. The template carries the same problem and
  is the better place to fix it
- `AGENTS.md` was replaced wholesale with the v0.9.1 render on 2026-08-01. It is a
  `_skip_if_exists` file, so copier had left it at the v0.7.0 boilerplate; the local
  copy carried no project-specific content, only a reformatted table-test snippet
  and a deleted TUI Testing section. Confirm you did not want that section gone,
  since the new render restores it

## Extending `--cli` to replace assess.sh

Re-verified against the source on 2026-08-01, before implementing anything from the
ROADMAP proposal.

- The proposal's premise holds: `--cli` emits the git fields and none of `ci[]`,
  `archived`, `default_branch`, `dependabot_alerts`, `exists_locally`, or the copier
  fields. `internal/copier.GetTemplateInfo` feeds only the TUI through
  `RepoSummary.TemplateInfo`; `internal/cli` never imports it, and
  `models.CopierTemplateInfo` has no JSON tags
- `is_template` and `has_freshen_txt` are a third gap the ROADMAP does not list.
  assess.sh emits both, and `freshen.txt` does not necessarily overlap with what
  `models.DetectNotes` reports as `notes_files`
- `ci[]` is more than a passthrough. `internal/github/workflow.go` produces
  `models.WorkflowSummary`, keyed differently from assess.sh's per-workflow
  `{workflow, status, conclusion, sha}` array, and the proposal also wants
  `failed_steps` and `completed_at`, which assess.sh does not emit
- Decide what `--fresh` means before adding anything. It is PR-only today, and
  `lookupPR`/`lookupPRCount` are cache-first and swallow errors, so copying that
  pattern would make CI and alert failures silently invisible
- `exists_locally` is inapplicable until the roster lands, since `--cli` discovers
  from the filesystem and a missing repo simply never appears
- ROADMAP's line references have drifted: it cites `assess.sh:23` for the fetch
  (now 24) and `assess.sh:42` for dependabot (now 41-44)

## Running the live test

`scripts/live-test.sh` (`mise run test:live`) exercises the destructive
`:cleanup` path against a scratch repo, covering the merged, squash-merged,
unmerged, and checked-out branch classes. It needs a token that can create and
delete repositories, which the default fine-grained PAT usually cannot. See the
"Token setup" section in `CONTRIBUTING.md`.

## Environment note

`hk install --mise` on git >= 2.55 writes `hook.hk-*.command` entries into
`.git/config` rather than files under `.git/hooks`, so checking for
`.git/hooks/pre-commit` is the wrong way to verify hooks are installed here.
