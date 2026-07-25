# Next steps

Findings from a review pass on 2026-07-21, exercising the tool against 62 real repos at depth 2 and reading the cleanup and GitHub-integration paths. Each item names the symbol to change and a suggested approach. Line numbers are omitted because they drift; grep the symbol.

## Already fixed

- PR caches were keyed on `upstream` alone (`origin/main`), which nearly every repo shares, so most repos read another repo's PR data. Keys are now scoped by repo path through `PRCacheKey`/`PRListCacheKey` in `internal/github/pr.go`
- jj cleanup assumed the default branch was `main`. `resolveDefaultBookmark` in `internal/vcs/jj.go` now resolves the real trunk via jj's `trunk()` revset
- `scripts/live-test.sh` (`mise run test:live`) exercises the destructive `:cleanup` path against a scratch repo, covering the merged, squash-merged, unmerged, and checked-out branch classes

## Correctness

`--cli` without `--fresh` can never return PR data. The cache is in-memory only, so every process starts cold and `pr`/`pr_count` are always omitted. All 62 repos returned null for both in the review run. Either persist the cache under `~/.cache/gh-repo-dashboard/` with the existing TTL so the flag pair means what it reads like, or always fetch in `--cli` mode and drop `--fresh`. A JSON consumer that silently omits PR data is worse than a slow one. If both flags stay, `-h` should state that `--cli` alone returns no PR data.

`:cleanup --dry-run` over-reports the checked-out branch. `mergedBranchNames` in `internal/vcs/git.go` strips the `* ` marker and then keeps the branch, so the preview promises a delete that `git branch -d` refuses. It over-reports and never under-reports, so nothing vanishes unannounced, but the preview is not truthful. Capture the `* ` prefix before stripping and skip that branch, matching the guard `deleteSquashMerged` already applies. The live-test harness pins the current behavior and will flip to a failure telling you to update it once this is fixed.

jj repos never get PR data at all. They report `branch: "@"` with no upstream, so `lookupPR` and `lookupPRCount` in `internal/cli/cli.go` short-circuit on the empty upstream. The README claims PR features work with both git and jj. Resolve the working-copy bookmark name and use that instead of the empty upstream, reusing the `trunk()` approach from the cleanup fix.

## Robustness

A typo'd scan path is silently ignored, exit 0 with an empty repo list, indistinguishable from a directory that genuinely holds no repos. `os.Stat` each positional path up front and fail with a clear message, since this is a user-input boundary.

`--script` always exits 0, even on an unknown command or a bad predicate, so automation cannot tell a broken script from a clean run. A missing script file does exit nonzero, so the two failure classes are inconsistent. Track whether any line errored and exit 1 at the end.

## Discoverability

`-h` is a bare Go `flag` dump. It never mentions that positional arguments are scan paths, that no arguments falls back to the config file and then the enclosing repo root, or that `~/.config/gh-repo-dashboard/config.toml` exists. There is also no headless way to list the `:commands`; you have to read `DefaultRegistry()`. Add a custom `flag.Usage` with a short synopsis and a README pointer, and make `:help` print the registry in `--script` mode.

## Release and distribution

The extension is not installable. Every release carries zero binary assets, because `bump_version.yml` pushes the tag with `secrets.GITHUB_TOKEN`, and GitHub suppresses workflow triggers for `GITHUB_TOKEN`-authored pushes, so `release.yml` (goreleaser) has never run. Either move the goreleaser job into `bump_version.yml` so it runs in the same workflow run, or push the tag with a PAT stored as a repo secret so the tag push is attributed to a user and triggers `release.yml`. The first needs no new secret and keeps the tag and the build from disagreeing.

`Formula/gh-repo-dashboard.rb` ships `version "0.1.0"` with `REPLACE_WITH_SHA256_FOR_*` placeholders, and `kyleking/homebrew-tap` returns 404. Either finish the tap or drop the `brew install` line from the README, which promises something that fails today.

## Template and dependencies

On copier `my_go_template` v0.3.0, two patch versions behind v0.3.2. Bubbletea is already on `charm.land/*/v2`, so no framework migration is needed. Run the copier update when convenient.

## Running the live test

The harness needs a token that can create and delete repositories, which the default fine-grained PAT usually cannot. See the "Token setup" section in `CONTRIBUTING.md`.
