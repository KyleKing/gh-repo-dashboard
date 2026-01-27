# Repo Dashboard - Development Guide

K9s-inspired Bubble Tea TUI for managing multiple git and jj repositories with progressive loading, filtering, GitHub PR integration, and batch maintenance tasks.

## Project Overview

**Framework:** Bubble Tea (Go TUI framework)
**Theme:** Catppuccin Macchiato
**Design Philosophy:** Minimal color, single unified background, borders for hierarchy, vim-style keybindings

### Architecture

```
├── main.go                    # CLI entry point
├── go.mod / go.sum           # Dependencies
├── internal/
│   ├── app/                  # Bubble Tea app
│   │   ├── app.go           # Model definition, Init
│   │   ├── update.go        # Update function (message handling)
│   │   ├── view.go          # View rendering
│   │   ├── keymap.go        # Key bindings
│   │   ├── commands.go      # Tea commands
│   │   └── messages.go      # Message types
│   ├── models/               # Data structures
│   │   ├── repo.go          # RepoSummary, WorktreeInfo
│   │   ├── branch.go        # BranchInfo
│   │   ├── pr.go            # PRInfo, PRDetail, WorkflowSummary
│   │   ├── filter.go        # ActiveFilter, ActiveSort
│   │   └── enums.go         # FilterMode, SortMode, etc.
│   ├── vcs/                  # VCS abstraction
│   │   ├── operations.go    # VCSOperations interface
│   │   ├── git.go           # Git implementation
│   │   ├── jj.go            # JJ implementation
│   │   ├── factory.go       # Detection and factory
│   │   └── mock.go          # Test mock
│   ├── filters/              # Filter/sort logic
│   │   ├── filter.go        # FilterRepos, FilterReposMulti
│   │   ├── sort.go          # SortPaths, SortPathsMulti
│   │   └── search.go        # Fuzzy search
│   ├── discovery/            # Repo discovery
│   │   └── discovery.go     # DiscoverRepos
│   ├── batch/                # Batch operations
│   │   ├── runner.go        # Task runner
│   │   └── tasks.go         # Task definitions
│   ├── github/               # GitHub integration
│   │   ├── pr.go            # PR operations
│   │   └── workflow.go      # Workflow runs
│   ├── cache/                # Caching
│   │   └── ttl.go           # Generic TTL cache
│   └── ui/styles/            # Styling
│       └── styles.go        # Lipgloss styles
└── wip-test-improvements.md  # Testing patterns documentation
```

## Development Environment

### Prerequisites

- Go 1.23+
- git CLI (if managing git repos)
- jj CLI (if managing jj repos)
- gh (GitHub CLI, optional for PR features with both git and jj)

### Setup

```bash
# Build and run
go build -o gh-repo-dashboard .
./gh-repo-dashboard ~/Developer --depth 2

# Or as a GitHub CLI extension
gh extension install .
gh repo-dashboard ~/Developer
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package tests
go test -v ./internal/filters/...

# Run specific test
go test -v -run TestFilterRepos ./internal/filters/

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run with race detector
go test -race ./...
```

### Visual Testing

See `test-improvements.md` for comprehensive testing patterns including:

1. **teatest (Golden File Testing)** - Visual regression with snapshot comparison
2. **catwalk (Data-Driven Testing)** - Complex interaction sequence testing
3. **Direct Testing** - State transition and business logic testing

```bash
# Run golden file tests (if using build tag)
go test -tags=golden ./...

# Update golden files
go test -tags=golden -update ./...
```

## VCS Support

The dashboard uses an interface-based abstraction to support multiple version control systems.

### Architecture

**VCS Interface Pattern:**
- `VCSOperations` interface defines the contract for both read and write operations
- `GitOperations` and `JJOperations` implement the interface
- `DetectVCSType()` auto-detects VCS by directory presence (`.git` or `.jj`)
- `GetVCSOperations()` factory returns the appropriate implementation
- Colocated repos (both `.git` and `.jj`) prefer jj

**Key Files:**
- `vcs/operations.go` - Interface defining VCS operations
- `vcs/git.go` - Git implementation with full interface support
- `vcs/jj.go` - Jujutsu implementation with full interface support
- `vcs/factory.go` - VCS detection and factory function
- `batch/tasks.go` - Batch operations using VCS abstraction

### Git vs JJ Concept Mapping

| Concept | Git | JJ (Jujutsu) | Notes |
|---------|-----|--------------|-------|
| Current location | HEAD | @ (working copy) | jj always has a working copy change |
| Branch | branch | bookmark | jj bookmarks are similar to git branches |
| Staged changes | index/staging | N/A | jj automatically tracks all changes |
| Uncommitted | unstaged + staged | working copy | Different mental model |
| Commits ahead/behind | ahead/behind | ahead/behind | Similar concept |
| Remote tracking | upstream branch | tracking bookmark | Similar |
| Stash | stash | N/A | jj doesn't need stashing (can create changes) |
| Worktree | worktree | workspace | Similar but jj workspaces are more powerful |

### VCS Operations

**Read Operations:**
- `GetRepoSummary()` - Get repository status and metadata
- `GetCurrentBranch()` - Get current branch/bookmark name
- `GetBranchList()` - List all branches/bookmarks
- `GetStashList()` - List stashes (git only, jj returns empty)
- `GetWorktreeList()` - List worktrees/workspaces
- `GetCommitLog()` - Get commit/change history
- `GetAheadBehind()` - Get commits ahead/behind tracking branch
- `GetStagedCount()` / `GetUnstagedCount()` / `GetUntrackedCount()` - File status counts
- `GetConflictedCount()` - Count of conflicted files

**Write Operations (batch tasks):**
- `FetchAll()` - Fetch from all remotes
  - Git: `git fetch --all --prune`
  - JJ: `jj git fetch --all-remotes`
- `PruneRemote()` - Prune stale remote branches
  - Git: `git remote prune origin`
  - JJ: No-op (jj handles this automatically)
- `CleanupMergedBranches()` - Delete merged local branches/bookmarks
  - Git: Deletes local branches merged into main
  - JJ: Deletes bookmarks that are ancestors of main

All write operations return `(success bool, message string)` for UI feedback.

### GitHub CLI Integration

GitHub integration works for both git and jj repositories via the `gh` CLI:

- For git repos: Uses standard git directory
- For jj repos (non-colocated): Sets `GIT_DIR` environment variable to `.jj/repo/store/git`
- For jj repos (colocated): Uses `.git` directory like standard git repos

The `GetGitHubEnv()` helper in `vcs/factory.go` handles this transparently.

## Batch Tasks

Batch operations execute maintenance tasks across multiple repositories simultaneously.

### Architecture

**BatchTaskRunner:**
- Runs tasks sequentially across filtered repositories
- Uses VCS factory to get appropriate operations for each repo
- Tracks progress for each operation
- Handles errors gracefully (continues on failure)
- Sends progress messages via Tea commands

### Adding a New Batch Task

1. Add method to `VCSOperations` interface (`vcs/operations.go`)
   ```go
   type VCSOperations interface {
       // ... existing methods
       NewOperation(ctx context.Context, repoPath string) (bool, string)
   }
   ```

2. Implement in both `GitOperations` and `JJOperations`
   ```go
   // vcs/git.go
   func (g *GitOperations) NewOperation(ctx context.Context, repoPath string) (bool, string) {
       // Git-specific implementation
       return true, "Success message"
   }

   // vcs/jj.go
   func (j *JJOperations) NewOperation(ctx context.Context, repoPath string) (bool, string) {
       // JJ-specific implementation
       return true, "Success message"
   }
   ```

3. Create task function in `batch/tasks.go`
   ```go
   func TaskNewOperation(vcsOps vcs.VCSOperations, repoPath string) (bool, string) {
       return vcsOps.NewOperation(context.Background(), repoPath)
   }
   ```

4. Add handler in `app/update.go`
   ```go
   case "N":
       if m.viewMode == ViewModeRepoList {
           return m, m.startBatchTask("New Operation", batch.TaskNewOperation)
       }
   ```

5. Add keybinding to `app/keymap.go`
   ```go
   key.NewBinding(
       key.WithKeys("N"),
       key.WithHelp("N", "new operation"),
   ),
   ```

6. Add tests to `internal/batch/batch_test.go`

### Safety Considerations

**Read-Only by Default:**
- All existing functionality remains read-only
- Write operations require explicit user action (keybinding)

**Batch Task Safety:**
- Only operate on currently filtered repos (explicit scope)
- Progress feedback shows results incrementally
- Failures highlighted but don't stop batch execution

**JJ-Specific Considerations:**
- Non-colocated repos require GIT_DIR for gh CLI (handled automatically)
- jj operations are generally safer (immutable history)
- Some git concepts don't map to jj (stash, staged changes)
- jj has more powerful undo capabilities

## Code Style

### Go Conventions

**Structure:**
- Use lowercase unexported functions for internal helpers
- Place related code in the same package
- Use interfaces for abstraction
- Write small, composable functions with single responsibility

**Error handling:**
- Return errors explicitly
- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- Use `context.Context` for cancellation and timeouts
- Validate at system boundaries, trust internal code

**Naming:**
- Use MixedCaps (PascalCase for exported, camelCase for unexported)
- Acronyms should be all caps: `GetPRInfo`, `HTTPClient`
- Interface names should describe behavior: `VCSOperations`, `Fetcher`

**Comments:**
- Add package comments in a single file per package
- Add doc comments for exported functions/types
- Do not add comments explaining what code does (code should be self-explanatory)

### Bubble Tea Patterns

**Model structure:**
```go
type Model struct {
    // State
    viewMode    ViewMode
    loading     bool
    cursor      int

    // Data
    repoPaths     []string
    filteredPaths []string
    summaries     map[string]models.RepoSummary

    // UI components
    width  int
    height int
}
```

**Update function:**
```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            return m, tea.Quit
        case "j", "down":
            m.cursor++
        }
    case RepoSummaryLoadedMsg:
        m.summaries[msg.Path] = msg.Summary
    }
    return m, nil
}
```

**Commands:**
```go
func loadRepoSummary(path string) tea.Cmd {
    return func() tea.Msg {
        summary, err := vcs.GetRepoSummary(context.Background(), path)
        if err != nil {
            return RepoSummaryErrorMsg{Path: path, Err: err}
        }
        return RepoSummaryLoadedMsg{Path: path, Summary: summary}
    }
}
```

**View rendering with Lipgloss:**
```go
func (m Model) View() string {
    var b strings.Builder

    // Header
    header := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#8aadf4")).
        Render("Repository Dashboard")
    b.WriteString(header + "\n")

    // Content
    for i, path := range m.filteredPaths {
        style := lipgloss.NewStyle()
        if i == m.cursor {
            style = style.Background(lipgloss.Color("#363a4f"))
        }
        b.WriteString(style.Render(path) + "\n")
    }

    return b.String()
}
```

## Design Principles

### UI Design

**Catppuccin Macchiato Colors:**
- Base: `#24273a` (background)
- Surface0: `#363a4f` (elevated surfaces, cursor)
- Text: `#cad3f5` (primary text)
- Subtext0: `#a5adcb` (secondary text)
- Blue: `#8aadf4` (primary accent, borders)
- Mauve: `#c6a0f6` (search accent)
- Yellow: `#eed49f` (filter accent)
- Green: `#a6da95` (success, PRs)
- Peach: `#f5a97f` (dirty repos)

**Visual hierarchy:**
- Borders provide visual separation
- Color is reserved for actionable elements (badges, accents)
- Minimal color usage overall
- Single unified background
- Focus states use Surface0 for cursor

### Filtering Architecture

**Compositional filtering:**
```
FilterMode -> SearchText -> SortMode -> Display
```

Example: "DIRTY" filter + "api" search = dirty repos containing "api"

**Filter modes:**
- ALL - Show all repositories
- DIRTY - Uncommitted changes or unpushed commits
- AHEAD - Commits ahead of tracking branch
- BEHIND - Commits behind tracking branch
- HAS_PR - Has associated GitHub PR
- HAS_STASH - Has stashed changes

**Sort modes:**
- NAME - Alphabetical by repo name
- MODIFIED - Most recently modified first
- STATUS - Dirty repos first, then by uncommitted count
- BRANCH - By branch name, then repo name

**Multi-sort support:**
- Go implementation supports multi-field sorting with priority
- Each sort can have ASC/DESC direction

**Search:**
- Fuzzy matching using sahilm/fuzzy library
- Case-insensitive
- Applied after filter mode, before sort
- Real-time updates as you type

## Key Features

### Progressive Loading

- Repo list appears immediately with placeholder data
- Goroutines load `RepoSummary` for each repo concurrently
- Table updates incrementally as data becomes available via Tea messages
- No blocking on slow git operations

### Caching Strategy

Generic TTL cache with mutex protection:
- `prCache` - GitHub PR information
- `branchCache` - Branch lists
- `summaryCache` - Repository summaries

Refresh clears all caches.

### View Hierarchy

**ViewModeRepoList** (initial)
- Shows all discovered repositories
- Columns: Name, Branch, Status, PR, Modified

**ViewModeRepoDetail** (drill-down with Enter)
- Shows branches, stashes, worktrees, PRs
- Tab switching between detail views

**ViewModeFilter** (f key)
- Filter selection modal
- Multi-filter with AND logic

**ViewModeSort** (s key)
- Sort selection modal
- Multi-sort with priority

**ViewModeHelp** (? key)
- Complete keybinding reference

**ViewModeBatchProgress**
- Progress bar and results during batch operations

## Common Tasks

### Adding a new filter mode

1. Add const to `FilterMode` in `models/enums.go`
2. Add filter function in `filters/filter.go`
3. Add case to `FilterRepos()` in `filters/filter.go`
4. Add tests in `filters/filter_test.go`

### Adding a new keybinding

1. Add key binding to `keymap.go`
2. Add case to `handleKey()` in `update.go`
3. Update help text in `view.go`
4. Add test in `app_test.go`

### Adding a new view mode

1. Add const to `ViewMode` in `app/app.go`
2. Add view rendering in `view.go`
3. Add update handling in `update.go`
4. Add navigation logic (enter/exit)

## External Dependencies

### Required (VCS-specific)

- **git** - For managing git repositories
  - Used for: status, branch list, commits, stashes, worktrees
  - Assumes git is in PATH
  - Not needed if only managing jj repos

- **jj** - For managing jujutsu repositories
  - Used for: status, bookmark list, changes, workspaces
  - Assumes jj is in PATH
  - Not needed if only managing git repos
  - Install: See https://github.com/martinvonz/jj

### Optional

- **gh** (GitHub CLI) - PR features for both git and jj repos
  - Used for: fetching PR info, check status, PR details
  - Works with both git and jj repositories
  - For non-colocated jj repos: automatically sets GIT_DIR
  - If missing: PR columns show dash instead of failing
  - Install: `brew install gh` (macOS) or see https://cli.github.com/

## Debugging

### Bubble Tea Debugging

```bash
# Run with debug logging
DEBUG=1 ./gh-repo-dashboard ~/Developer

# Log to file
./gh-repo-dashboard ~/Developer 2>debug.log
```

### Common Issues

**Terminal size issues:**
- Model receives `tea.WindowSizeMsg` on startup and resize
- Ensure `m.width` and `m.height` are updated

**Message ordering:**
- Commands execute asynchronously
- Don't assume message arrival order
- Use state flags to track loading/completion

**Goroutine leaks:**
- Use `context.Context` for cancellation
- Cancel contexts when leaving views or quitting

## Performance Considerations

- Fuzzy search uses sahilm/fuzzy for efficient matching
- Progressive loading prevents blocking on initial scan
- TTL caching with mutex protection for thread safety
- Goroutines with Tea commands for parallel data loading
- Lipgloss style caching (reuse style objects)

## Release Checklist

1. Run full test suite: `go test ./...`
2. Run with race detector: `go test -race ./...`
3. Test manually with real repositories (both git and jj if available)
4. Test batch operations (fetch, prune, cleanup)
5. Update version in `main.go`
6. Update `README.md` if features changed
7. Build for release: `go build -o gh-repo-dashboard .`
