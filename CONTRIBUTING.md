# Contributing to gh-repo-dashboard

## Setup

Prerequisites: Go (see `go.mod`), [mise](https://mise.jdx.dev/), [hk](https://hk.jdx.dev/)

```bash
mise install
hk install --mise
mise run ci
```

## Tasks

Shared tasks live in `.config/mise/conf.d/template.toml` (managed by the copier template).
Project-specific tasks go in additional `.config/mise/conf.d/*.toml` files.

| Command | Description |
|---------|-------------|
| `mise run bench` | Run benchmarks |
| `mise run build` | Build binary |
| `mise run ci` | Full CI check (tests + build) |
| `mise run clean` | Clean build artifacts |
| `mise run demo` | Generate VHS demo recordings |
| `mise run format` | Auto-fix lint and formatting |
| `mise run hooks` | Run git hooks |
| `mise run lint` | Run linter |
| `mise run run` | Run from source (`go run`, always reflects current code) |
| `mise run test` | Run tests with coverage |
| `mise run test:live` | [Live integration test](#live-integration-test) of `:cleanup` against a scratch GitHub repo |
| `mise tasks` | List all available tasks |

## Live Integration Test

`scripts/live-test.sh` (`mise run test:live`) is the only test that exercises
`:cleanup` against real git and real GitHub. Everything else mocks the VCS layer,
which means the unit suite cannot catch a change that deletes the wrong branch.

### What it covers

The script creates four branch classes in a scratch repo and asserts how cleanup
treats each one:

| Branch | Setup | Expected |
|--------|-------|----------|
| `merged-branch` | merged into the default branch with `--no-ff` | deleted via `git branch -d` |
| `squashed-branch` | opened as a PR, squash-merged with `gh pr merge --squash` | deleted via `git branch -D` after OID verification |
| `unmerged-branch` | one unique commit, no PR | survives |
| `current-branch` | checked out, sitting at the default branch tip | survives |

`squashed-branch` is the reason the script exists. `git branch --merged` cannot
see a squash-merge, so cleanup falls back to matching the branch tip against
merged PR head OIDs from `gh` and then force-deletes. That path only runs against
a real GitHub PR.

`unmerged-branch` carries the assertion that matters most. Deleting it would be
data loss, so a failure there should block any release.

The script runs `:cleanup --dry-run` first, then the real `:cleanup`, and
compares the two. Every branch actually deleted must have been named in the dry
run. A dry run that disagrees with the real run fails the test.

### Known issue it pins

`:cleanup --dry-run` over-reports the checked-out branch: `batch.PreviewCleanup`
lists everything `git branch --merged` returns, but the real run's
`git branch -d` refuses to delete a branch that is checked out. The branch is
announced and then survives. The dry run over-reports and never under-reports, so
no branch disappears without warning.

The script asserts the current behavior on purpose. If someone fixes the source,
that check flips to a failure telling you to update the script.

### Safety design

- It refuses to run without an explicit opt-in: `--yes` or `GH_RD_LIVE_TEST=1`.
  Without one it prints exactly what it would create and destroy, then exits 1
- It creates its own private repo named `gh-rd-livetest-<epoch>-<rand>` and only
  ever operates on that. Every delete passes through a guard that rejects any
  name missing the `gh-rd-livetest-` prefix
- It clones into a `mktemp -d` directory. Every git command passes through a
  guard asserting the working directory is under that temp root, so it can never
  reach your `~/Developer` tree
- An `EXIT` trap deletes the scratch repo and the temp directory on success,
  failure, or interrupt. Set `KEEP=1` (or pass `--keep`) to leave both in place
  for debugging
- It verifies up front that the token can both create *and* delete repositories.
  A classic token is checked for the `delete_repo` scope; a fine-grained token
  does not advertise scopes, so the script probes with a disposable
  `gh-rd-livetest-probe-*` repo instead. Without this check a missing scope would
  leak the scratch repo on every run. A classic token needs `repo` and
  `delete_repo`; a fine-grained token needs `Administration: Read and write`
  alongside the usual contents and pull-request permissions
- It builds the binary under test with `go build` into the temp directory rather
  than using whatever `gh extension` version happens to be installed

### Token setup

The script aborts in preflight if the token cannot create a repository:

```
POST /user/repos -> 403 Resource not accessible by personal access token
```

What to do depends on which credential `gh` has stored. Check the prefix in
`gh auth status`: `gho_` is an OAuth token, `ghp_` a classic PAT, `github_pat_`
a fine-grained PAT.

- **OAuth token** (`gho_`): `gh auth refresh --hostname github.com --scopes delete_repo`
- **Fine-grained PAT** (`github_pat_`): `gh auth refresh` cannot help, because the
  permissions live on GitHub's side rather than under gh's OAuth app. Either
  switch to the OAuth flow with `gh auth login --web`, or edit the token under
  Settings → Developer settings → Fine-grained tokens and grant
  **Administration: Read and write** with **Repository access: All repositories**.
  All repositories is required because the script creates a repo that does not
  exist yet, which a token scoped to selected repositories can never reach
- **Classic PAT** (`ghp_`): add the `delete_repo` scope on the web, then re-store
  the token with `gh auth login --with-token`

Verify with the same probe the script uses:

```bash
gh api -X POST /user/repos -f name=zz-token-probe -F private=true --jq .full_name \
  && gh repo delete zz-token-probe --yes
```

### Cost

One private repo and one pull request created and deleted, a handful of `gh` API
calls, and roughly 30 seconds. Nothing persists after a successful run.

### Running it

```bash
mise run test:live -- --yes
# or
GH_RD_LIVE_TEST=1 ./scripts/live-test.sh
# keep the scratch repo and temp clone for inspection
KEEP=1 ./scripts/live-test.sh --yes
```

It is deliberately excluded from `mise run ci`, because CI has no business
creating repositories under a contributor's account.

## Code Guidelines

Follow [AGENTS.md](AGENTS.md) for code organization, testing patterns, and error handling.

Linting is configured in `.golangci.toml` with 40+ rules. Run `mise run format` to auto-fix.

## Git Workflow

Conventional commits enforced via [commitizen](https://commitizen-tools.github.io/commitizen/):

```
<type>(<scope>): <subject>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Git hooks run automatically via hk on commit and push.


## Development Install

Run straight from source with `go run`, which always reflects the current code, so there's no built binary or installed extension to go stale between edits:

```bash
go run ./cmd/gh-repo-dashboard [args]
```

To test the actual `gh gh-repo-dashboard ...` extension invocation or a Homebrew install, use the released version rather than installing from this checkout:

```bash
gh extension install kyleking/gh-repo-dashboard
# or
brew install --formula https://github.com/kyleking/gh-repo-dashboard/raw/main/Formula/gh-repo-dashboard.rb
```


## Releases

Releases are fully automated by the `Bump Version` workflow: every push to `main` with a releasable Conventional Commit triggers commitizen to bump the version, tag, and update `CHANGELOG.md`, then goreleaser builds the binaries, publishes the GitHub release, and pushes an updated cask to [kyleking/homebrew-tap](https://github.com/kyleking/homebrew-tap). Tags pushed with the default `GITHUB_TOKEN` cannot trigger a second workflow, which is why goreleaser runs inside the same workflow rather than on tag push.

The tap push happens over SSH with a write deploy key on `kyleking/homebrew-tap`, stored as the `TAP_DEPLOY_KEY` repository secret (GitHub offers no API to create PATs, so a deploy key keeps provisioning scriptable and scoped to one repo). Run `scripts/provision-tap-deploy-key.sh` to create or rotate it once the tap repo exists; the script archives the private key in 1Password. If the secret is missing the release still publishes; only the cask upload fails.

After a release, verify the properly named binaries are attached (`gh-repo-dashboard-linux-amd64`, `gh-repo-dashboard-darwin-arm64`, `gh-repo-dashboard-windows-amd64.exe`, etc.), since `gh extension install kyleking/gh-repo-dashboard` and the cask both download them by that exact naming.


## Troubleshooting

```bash
mise install --force   # Reinstall tools
hk install --mise --force  # Reinstall hooks
go test -v -run TestName ./package  # Debug specific test
```
