## v1.3.1 (2026-07-27)

### Fix

- **release**: build each target into its own dist path

## v1.3.0 (2026-07-27)

### Feat

- surface every detected notes file, not just the first match
- read copier template metadata in panel

### Fix

- **lint**: extract usage and scan-path helpers out of main
- **lint**: accept Go's snake_case filenames in ls-lint
- clarify CLI help text

## v1.2.1 (2026-07-26)

### Fix

- **ci**: skip goreleaser when a push produces no version bump

## v1.2.0 (2026-07-26)

### Feat

- preflight missing-tool checks with git fallback for colocated repos

### Fix

- **lint**: make repeated command strings constants and annotate deliberate exec wrappers
- **release**: run goreleaser inside the bump workflow and publish a cask to the tap
- **ci**: repair golangci config for the v2 schema, pin the lint version, and gate on coverage
- resolve the jj default bookmark instead of assuming main
- scope PR cache keys by repo path

### Refactor

- **scripts**: require the tap repo to exist and fix the secret name

## v1.1.0 (2026-07-06)

### Feat

- command history, @: repeat, --script, and --cli --filter (M12)
- TOML config file at the XDG path (M11)
- detect and clean squash-merged branches (M9)
- surface per-repo notes files (M8)
- add --cli flag for non-interactive JSON output

### Fix

- compute real ahead/behind vs default branch in branch detail
- keep last upstream-less branch in GetBranchList
- stop caching nil PR/workflow results when gh fails
- assign contiguous sort priorities in CycleSortState

### Refactor

- split view.go by view mode and move Cmd constructors (M10 phase 2)
- code-health quick wins from survey (M10 phase 1)
- split vcs.Operations into composed sub-interfaces (M7)
- split renderPRDetail's loading/description/actions to reduce complexity
- flatten compareToDefaultBranch's nested loops to reduce complexity
- split renderBranchDetail into per-section writers
- extract sort-modal row building/rendering to reduce complexity
- extract table-row and branch-row rendering to reduce complexity
- extract breadcrumb/status-bar rendering to reduce complexity
- reduce nestif in completeCommand and copyToClipboardCmd
- extract adjacent-PR navigation to reduce handlePRDetailKey complexity
- extract handleDetailKey's tab/cursor/enter logic to reduce complexity
- extract handleKey's cursor/enter/back handling to reduce complexity
- extract Update message handlers to reduce cognitive complexity
- extract filter/select/sort commands to reduce DefaultRegistry complexity
- extract fixture parsing/assertion helpers to reduce complexity
- extract subtest bodies to reduce test cognitive complexity
- extract porcelain status classification to reduce complexity

## v1.0.1 (2026-07-04)

### Fix

- center modals and re-record demo

## v1.0.0 (2026-07-04)

### Feat

- add fixture-based tests with generated usage docs
- add vim-style text objects and operators
- add predicate expressions for filter and select
- add command mode with registry and tab completion
- upgrade to Bubble Tea v2
- begin replacing Python implementation with Go
- add PRs table
- improve info modal and test coverage
- continue golang migration
- start go refactor for gh-cli
- implement workflow caching
- add GitHub Actions workflow statuses (#1)
- inspired by gita, improve symbols and colors
- improve graceful error handling
- finish initial batch implementation
- implement batch maintenance tasks
- display jj repo information
- rename to 'reda'
- second level filter and sort, VHS demo, and other minor changes
- add chording for filters and sort; add esc <C-u>|<C-d> bindings
- add Search ('/')
- update colors and layout
- implement filters
- add copy modal
- begin major rewrite
- add --path and filtering
- improve overall usage and add tests
- init

### Fix

- parse real jj bookmark/workspace list output
- finish filter, sort, and progressive loading
- minor tweaks and add test snapshots

### Refactor

- flatten repository
- generalize vcs to add support for jj
