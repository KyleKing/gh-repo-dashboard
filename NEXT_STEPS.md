# Next steps

Immediate follow-ups. Feature work lives in `ROADMAP.md`. Every item here was
re-verified against the source on 2026-08-11 and names a symbol rather than a
line number, so grep for it.

## Correctness

A typo'd scan path is silently ignored, exiting 0 with an empty repo list,
indistinguishable from a directory that genuinely holds no repos.
`resolveScanPaths` in `cmd/gh-repo-dashboard/main.go` returns its positional
arguments without stat'ing them. This is a user-input boundary, so it should
fail with a clear message.

`--script` exits 0 on an unknown command or a bad predicate. `RunScript` in
`internal/app/script.go` returns an error only for an unreadable script or a
scanner failure, and `runScriptLine` swallows per-line failures. A missing
script file does exit nonzero, so the two failure classes disagree. Track
whether any line errored and return at the end.

`--cli` reads PR data unevenly across processes. `pr_count` survives a cold
start through the disk cache (`cache.Persisted` in `github.CachedPRs`), while
`pr` and `ci` are memory-only and are always absent without `--fresh`. Either
persist them the same way or say so in `-h`, since a JSON consumer cannot tell
an absent field from an empty one.

## Discoverability

`-h` never mentions that `~/.config/gh-repo-dashboard/config.toml` exists, and
there is no headless way to list the `:commands` (you have to read
`DefaultRegistry()`). Making `:help` print the registry in `--script` mode
covers both.

## Template

This repo is pinned at my_go_template v0.9.1 and the template is at v0.11.3, so
an update is three minor versions of drift away. Two things to check against the
newer template when it happens:

- `.pre-commit-config.yaml` is dead config here, and running `prek run
  --all-files` actively corrupts the tree: with no testdata exclude,
  end-of-file-fixer and trailing-whitespace rewrite the `.golden` fixtures, and
  mdformat plus prettier reformat every markdown file and workflow against the
  committed style. prek runs in neither CI nor the git hooks. Either delete the
  file or give it the excludes `hk.pkl` has
- The `ci` job's GOROOT fix (`d41265c`) lives in a template-managed file. mise
  and `actions/setup-go` disagree about `GOROOT` when both run in one job, which
  goes red the moment mise's `go = "latest"` resolves past `go.mod`. Confirm
  v0.11.3 carries the fix before accepting its `ci.yml`
