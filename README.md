# gh-repo-dashboard

![demo](https://raw.githubusercontent.com/kyleking/gh-repo-dashboard/main/.github/assets/demo.gif)

See the status of every git and jj repository under a directory in one dashboard,
with GitHub pull request and CI state beside the local status. Fetch, prune, and
delete merged branches across all of them at once, but you probably want to use one
of these instead:

| Name | Notes |
| --- | --- |
| [gh-dash](https://github.com/dlvhdr/gh-dash) | "A rich terminal UI for GitHub that doesn't break your flow" |
| [ghorg](https://github.com/gabrie30/ghorg) | "Quickly clone or backup massive amounts of org/users repositories into one directory" |
| [git-xargs](https://github.com/gruntwork-io/git-xargs) | "git-xargs is a command-line tool (CLI) for making updates across multiple Github repositories with a single command" |
| [gita](https://github.com/nosarthur/gita) | "Manage many git repos with sanity" |
| [gr](https://github.com/mixu/gr) | "Multiple git repository management tool" |
| [lazygit](https://github.com/jesseduffield/lazygit) | "simple terminal UI for git commands" (single repo, not a multi-repo dashboard) |
| [mani](https://github.com/alajmo/mani) | "CLI tool to help you manage repositories" |
| [meta](https://github.com/mateodelnorte/meta) | "tool for turning many repos into a meta repo" |
| [mu-repo](https://github.com/fabioz/mu-repo) | "Tool to help in dealing with multiple git repositories" |
| [myrepos](https://myrepos.branchable.com/) | "a tool to manage all your version control repositories" |
| [repo](https://android.googlesource.com/tools/repo) | Google's tool for checking out and syncing the many Git repositories behind a single Android build |
| [Sourcegraph](https://sourcegraph.com) | "Universal code search" (Closed source) |
| [tsrc](https://github.com/your-tools/tsrc) | "Manage groups of git repositories" |
| [vcstool](https://github.com/dirk-thomas/vcstool) | "Vcstool is a command line tool designed to make working with multiple repositories easier" |

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
