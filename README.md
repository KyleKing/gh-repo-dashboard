# gh-repo-dashboard

![demo](https://raw.githubusercontent.com/kyleking/gh-repo-dashboard/main/.github/assets/demo.gif)

See the status of every git and jj repository under a directory in one dashboard,
with GitHub pull request and CI state beside the local status. Fetch, prune, and
delete merged branches across all of them at once.

## Install

```bash
# GitHub CLI extension
gh extension install kyleking/gh-repo-dashboard
# Homebrew
brew install --cask kyleking/tap/gh-repo-dashboard
# from source
go build -o gh-repo-dashboard ./cmd/gh-repo-dashboard
```

## Quick start

Pass the directories that hold your repositories:

```bash
gh repo-dashboard -depth 2 ~/Developer
```

Move with `j` and `k`, press `enter` to open a repo, and press `?` for the
keymap. Set `scan_paths` in the [config file](./docs/configuration.md) once and
`gh repo-dashboard` launches from anywhere with no arguments.

Run it inside a repository and it opens that repo's branches directly, listing
each branch with its pull request, CI checks, and which sibling checkout or
worktree currently holds it. From there `c` switches branch, `p` pushes with
`--follow-tags`, `N` opens a pull request, and `M` squash-merges one. Each of
those asks for confirmation first.

## What it does not do

- Commit, stage, rebase, or resolve conflicts. The writes it does make are
  `fetch`, remote prune, deleting branches already merged upstream, switching
  branch, pushing, and creating or squash-merging a pull request
- Replace a single-repo client. Use lazygit or gitui for staging, hunk-level
  work, and history surgery
- Replace `gh`. Pull request data comes from the `gh` CLI, and every non-GitHub
  column still works when `gh` is missing or logged out
- Edit your notes files. It reads the configured notes files and never writes them
- Run as a server or daemon. `--cli` and `--script` are one-shot headless runs

Full docs: [./docs](./docs)

## License

MIT
