# Contributing to Repo Dashboard

## Development Setup

### Prerequisites

- Go 1.23+
- git (required for core functionality)
- jj (optional, for Jujutsu repository support)
- gh (GitHub CLI, optional for PR features)

### Installation

```bash
# Build and run
go build -o gh-repo-dashboard .
./gh-repo-dashboard ~/Developer

# Or install as gh extension
gh extension install .
gh repo-dashboard ~/Developer
```

## Testing

### Unit Tests

Run all tests:
```bash
go test ./...
```

Run with verbose output:
```bash
go test -v ./...
```

Run specific package:
```bash
go test -v ./internal/filters/...
```

Run specific test:
```bash
go test -v -run TestFilterRepos ./internal/filters/
```

Run with coverage:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

Run with race detector:
```bash
go test -race ./...
```

### Visual Testing

See `test-improvements.md` for comprehensive testing patterns:

1. **teatest (Golden File Testing)** - Visual regression with snapshot comparison
2. **catwalk (Data-Driven Testing)** - Complex interaction sequence testing
3. **Direct Testing** - State transition and business logic testing

```bash
# Run golden file tests (if using build tag)
go test -tags=golden ./...

# Update golden files
go test -tags=golden -update ./...
```

## Recording Demo

Generate demo GIF using VHS:

```bash
# Install VHS (if not already installed)
# macOS:
brew install vhs

# Other platforms:
# https://github.com/charmbracelet/vhs#installation

# Record the demo
vhs < .github/assets/demo.tape
```

The demo will be saved as `.github/assets/demo.gif`.

**Editing the demo:**
1. Edit `.github/assets/demo.tape` to change the recording script
2. Run `vhs < .github/assets/demo.tape` to regenerate
3. Commit both the tape file and generated GIF

**VHS tips:**
- Use `Set PlaybackSpeed` to control animation speed
- Use `Sleep` between actions to let UI settle
- Use `Hide`/`Show` to hide setup commands
- Use Catppuccin Macchiato theme to match app theme

## Code Style

See [CLAUDE.md](./CLAUDE.md) for detailed code style guidelines.

**Key principles:**
- Use interfaces for abstraction
- Write small, composable functions with single responsibility
- Return errors explicitly with context
- Use `context.Context` for cancellation and timeouts
- No inline comments explaining what code does
- Add doc comments for exported functions/types

## Architecture

See [CLAUDE.md](./CLAUDE.md) for detailed architecture documentation.

**Key packages:**
- `internal/app/` - Bubble Tea app (Model, Update, View)
- `internal/models/` - Data structures (RepoSummary, BranchInfo, PRInfo)
- `internal/filters/` - Filter and sort logic with fuzzy search
- `internal/vcs/` - VCS abstraction (git and jj implementations)
- `internal/github/` - GitHub CLI integration
- `internal/discovery/` - Repository discovery
- `internal/cache/` - Generic TTL-based caching
- `internal/batch/` - Batch task runner

## Common Tasks

### Adding a new filter mode

1. Add const to `FilterMode` in `internal/models/enums.go`
2. Add filter function in `internal/filters/filter.go`
3. Add case to `FilterRepos()` in `internal/filters/filter.go`
4. Add tests in `internal/filters/filter_test.go`

### Adding a new keybinding

1. Add key binding to `internal/app/keymap.go`
2. Add case to key handling in `internal/app/update.go`
3. Update help text in `internal/app/view.go`
4. Add test in `internal/app/app_test.go`

### Adding a new view mode

1. Add const to `ViewMode` in `internal/app/app.go`
2. Add view rendering in `internal/app/view.go`
3. Add update handling in `internal/app/update.go`
4. Add navigation logic (enter/exit)

### Adding a new batch task

1. Add method to `VCSOperations` interface in `internal/vcs/operations.go`
2. Implement in both `GitOperations` and `JJOperations`
3. Create task function in `internal/batch/tasks.go`
4. Add handler in `internal/app/update.go`
5. Add keybinding to `internal/app/keymap.go`
6. Add tests to `internal/batch/batch_test.go`

## External Dependencies

### Required

- **git** - Core functionality depends on git CLI

### Optional

- **jj** (Jujutsu) - For jj repository support
  - Install: See https://github.com/martinvonz/jj

- **gh** (GitHub CLI) - PR features require this
  - Install: `brew install gh` (macOS) or see https://cli.github.com/

## Debugging

### Logging

```bash
# Run with debug output to stderr
./gh-repo-dashboard ~/Developer 2>debug.log
```

### Common Issues

**Terminal size issues:**
- Model receives `tea.WindowSizeMsg` on startup and resize
- Ensure width/height are updated in Update()

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
