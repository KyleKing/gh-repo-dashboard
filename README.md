# Repo Dashboard

![.github/assets/demo.gif](https://raw.githubusercontent.com/kyleking/gh-repo-dashboard/main/.github/assets/demo.gif)

K9s-inspired TUI for managing multiple git and jj repositories with GitHub PR integration.

## Installation

As a GitHub CLI extension:
```bash
gh extension install kyleking/gh-repo-dashboard
```

Or build from source:
```bash
go build -o gh-repo-dashboard .
```

## Usage

```bash
# Scan default directory (~/Developer)
gh repo-dashboard

# Scan specific directories
gh repo-dashboard ~/projects ~/work

# Limit scan depth
gh repo-dashboard -depth 2 ~/Developer

# Print repo summaries as JSON instead of launching the TUI
# (uses only cached GitHub data, so PR fields may be omitted)
gh repo-dashboard --cli ~/projects

# JSON output with fresh GitHub PR data (invokes gh per repo)
gh repo-dashboard --cli --fresh ~/projects
```

## Supported Version Control Systems

- **Git**: Full support for git repositories
- **Jujutsu (jj)**: Full support for jj repositories (both colocated and non-colocated)

The dashboard automatically detects the VCS type and uses appropriate operations. Colocated repositories (having both `.git` and `.jj`) are treated as jj repositories.

**Requirements:**
- Go 1.23+ (for building)
- git CLI (if managing git repos)
- jj CLI (if managing jj repos)
- gh CLI (GitHub CLI) - optional, for PR features with both git and jj repos

## Keybindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `down` | Move down |
| `k` / `up` | Move up |
| `g` | Go to top |
| `G` | Go to bottom |
| `enter` / `space` | Select / drill down |
| `esc` / `backspace` | Go back |
| `q` | Quit |

### Views

| Key | Action |
|-----|--------|
| `?` | Help |
| `/` | Search |
| `f` | Filter modal |
| `s` | Sort modal |
| `R` | Reverse sort |
| `r` | Refresh |

### Detail View

| Key | Action |
|-----|--------|
| `tab` | Next tab |
| `h` / `left` | Previous tab |
| `l` / `right` | Next tab |

### Actions

| Key | Action |
|-----|--------|
| `o` | Open PR in browser |
| `c` | Copy (branch/PR/path) |

### Batch Operations

| Key | Action |
|-----|--------|
| `F` | Fetch all (filtered repos) |
| `P` | Prune remote branches (filtered repos, git only) |
| `C` | Cleanup merged branches (filtered repos) |

## Status Symbols

### Repository Status
- `+N` - N staged changes
- `*N` - N unstaged changes
- `?N` - N untracked files
- `!N` - N conflicted files
- `$N` - N stashed changes
- `WN` - N worktrees/workspaces

### Ahead/Behind
- `^N` - N commits ahead of tracking branch
- `vN` - N commits behind tracking branch

### Workflow Status (GitHub Actions)
- `oN` - N successful workflow runs
- `xN` - N failed workflow runs
- `-N` - N skipped workflow runs
- `~N` - N pending/in-progress workflow runs

## Features

### Core Functionality
- **Multi-VCS Support**: Works with both git and jj repositories
- **Progressive Loading**: Data loads concurrently as it becomes available
- **TTL Caching**: Intelligent caching for PR information, workflow status, and VCS operations
- **GitHub Integration**: Pull request info, status checks, and workflow runs via gh CLI

### Filtering & Sorting
- **Multi-Filter Support**: Combine multiple filters with AND logic
- **Filter Modes**: all, dirty, ahead, behind, has_pr, has_stash
- **Sort Modes**: name, modified, status, branch (all reversible)
- **Fuzzy Search**: Real-time search with similarity matching

### Repository Management
- **Batch Operations**: Fetch, prune, and cleanup across filtered repositories
- **Worktree Detection**: Git worktrees and jj workspaces
- **Stash Tracking**: Git stash monitoring (jj doesn't use stashes)
- **Branch Details**: View branches, PRs, commits, workflow runs, and modified files

### User Experience
- **Vim-Style Keybindings**: Familiar navigation patterns
- **Help Modal**: Complete keybinding reference
- **Catppuccin Theme**: Dark theme with minimal color usage

## Batch Operations

Perform maintenance tasks across multiple repositories simultaneously:

### Fetch All (`F`)
Updates remote refs for all filtered repositories.
- **Git**: `git fetch --all --prune`
- **JJ**: `jj git fetch --all-remotes`

### Prune Remote (`P`)
Cleans up stale remote branch references.
- **Git**: `git remote prune origin`
- **JJ**: No-op (jj handles this automatically during fetch)

### Cleanup Merged Branches (`C`, `:cleanup`)
Deletes local branches/bookmarks that have been merged into main/master, plus
branches squash-merged via a pull request (detected by comparing the branch tip
against merged PR head OIDs from `gh`).
- **Git**: Deletes true-merges with `git branch -d` and verified squash-merges
  with `git branch -D`, skipping the current branch and any branch checked out
  in a worktree
- **JJ**: Deletes bookmarks that are ancestors of main, plus verified
  squash-merged bookmarks, via `jj bookmark delete`
- `:cleanup --dry-run [predicate]` previews what would be deleted without
  deleting anything

**Usage:**
1. Apply filters to select repositories (e.g., filter by "dirty" or search for specific repos)
2. Press `F`, `P`, or `C` to run the batch operation, or run `:cleanup --dry-run` to preview first
3. View real-time progress and results in the modal
4. Operations run sequentially across all filtered repositories

**Safety:**
- Batch operations only work in the repository list view
- Only operate on currently filtered/visible repositories
- Each operation shows success/failure status with detailed messages
- Failed operations don't stop the batch (continues to next repo)

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./internal/filters/...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run with race detector
go test -race ./...
```

### Recording the Demo

Generate demo GIF using VHS:

```bash
vhs < .github/assets/demo.tape
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) for more details on VHS setup and recording.

## Alternatives

### Multi-Repository TUIs

**[Git-Scope](https://github.com/Bharath-code/git-scope)** - Similar tool built with Bubble Tea.

| Feature | repo-dashboard | Git-Scope |
|---------|---------------|-----------|
| **VCS Support** | Git + Jujutsu (jj) | Git only |
| **GitHub Integration** | PR details, checks, status via gh CLI | Contribution graphs |
| **Filtering** | 6 modes (dirty, ahead, behind, has_pr, has_stash, all) | Dirty filter + pagination |
| **Batch Operations** | Fetch all, prune remote, cleanup merged branches | None |
| **Search** | Fuzzy search | Fuzzy search by name/path/branch |
| **Additional Features** | Worktrees/workspaces, stash tracking, PR opening | Editor launch, disk usage, timeline view |

### Other Multi-Repository Tools

- **[Gita](https://github.com/nosarthur/gita)** - CLI tool to manage multiple git repositories with custom groups and batch operations
- **[gitbatch](https://github.com/isacikgoz/gitbatch)** - Manage your git repositories in one place with interactive TUI
- **[mgitstatus](https://github.com/fboender/multi-git-status)** - Show uncommitted, untracked, and unpushed changes for multiple repos
- **[Mani](https://github.com/alajmo/mani)** - Go-based CLI with YAML configuration, built-in TUI, batch operations, and parallel command execution

### Single-Repository TUIs

- **[lazygit](https://github.com/jesseduffield/lazygit)** - Simple terminal UI for git commands
- **[GitUI](https://github.com/extrawurst/gitui)** - Blazing fast terminal UI for git written in Rust
- **[Gitu](https://github.com/altsem/gitu)** - TUI Git client inspired by Magit

## License

MIT
