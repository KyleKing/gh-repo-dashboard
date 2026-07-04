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
