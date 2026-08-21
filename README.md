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

Move with `j` and `k`, press `v` to expand the repo under the cursor without
leaving the list, `x` to mark it, `enter` to open it, and `?` for the keymap. Set `scan_paths` in the [config file](./docs/configuration.md) once and
`gh repo-dashboard` launches from anywhere with no arguments.

Run it inside a repository and it opens that repo directly, listing each branch
with its pull request, CI checks, and which sibling checkout or worktree
currently holds it. `x` marks rows, `V` opens a range, and `a` offers the verbs
for whatever is marked: switch branch, push with `--follow-tags`, open a pull
request, squash-merge one, apply a stash, or open a notes file in `$EDITOR`.
Anything that reaches the remote or destroys work asks for confirmation first.

## What it does not do

- Commit, stage, rebase, or resolve conflicts. The writes it does make are
  `fetch`, remote prune, deleting a branch, applying or dropping a stash,
  switching branch, pushing, and creating or squash-merging a pull request
- Replace a single-repo client. Use lazygit or gitui for staging, hunk-level
  work, and history surgery
- Replace `gh`. Pull request data comes from the `gh` CLI, and every non-GitHub
  column still works when `gh` is missing or logged out
- Edit your notes files itself. It reads them, and hands one to `$EDITOR` on
  request so the writing stays yours
- Run as a server or daemon. `--cli` and `--script` are one-shot headless runs

Full docs: [./docs](./docs)

## License

MIT
