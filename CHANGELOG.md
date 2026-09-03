## v1.11.1 (2026-09-03)

### Fix

- **ci**: force bash for test:coverage-min to support pipefail on dash runners

## v1.11.0 (2026-09-02)

### Feat

- **cli**: emit remote, remote_id, and worktrees in JSON output

### Refactor

- **app**: center modals through aragonite

## v1.10.2 (2026-08-31)

### Fix

- **scripts**: generate the tap deploy key inside 1Password

## v1.10.1 (2026-08-29)

### Refactor

- **ui**: source the model renderers from aragonite
- **ui**: source the markdown renderer from aragonite
- **styles**: build the named styles from aragonite's detected palette
- **ui**: source the region renderer from aragonite
- **ui**: source the table engine from aragonite

## v1.10.0 (2026-08-27)

### Feat

- **app**: mark rows with x, open the verb menu with a
- **app**: bracket a footer key inside the word it triggers, open PRs with o

### Fix

- give TOML arrays the trailing comma tombi needs
- **app**: make the PRs tab preview light, debounced, and fleet-wide

### Refactor

- **forge**: read the gh CLI through aragonite
- **vcs**: read git and jj through aragonite
- **ui**: render both expandable regions from one component
- **ui**: move pull request rendering out of the data package
- rename models.PRInfo to forge.PullRequest
- **cache**: source the TTL and disk cache from aragonite

## v1.9.0 (2026-08-15)

### Feat

- **app**: show a one-line explanation for every command-bar completion
- **app**: flag PRs missing a requested reviewer, preview a row in place
- **app**: show live command completion and a search scope legend
- **app**: support * and ? glob wildcards in search, with ^/$ anchors
- **app**: add p:, t:, c:, and n: search prefixes
- **app**: add :pr-query to override a PR view's GitHub search this session
- **app**: let :filter narrow the PRs tab like it does the repo list
- **app**: preview combined filter counts, surface the predicate and Git mode
- **app**: dock the filter and sort editors instead of a full-screen takeover
- **app**: surface find grammar as placeholder hints instead of help-only
- **app**: show the newest edited file, flag stale and off-default branches
- **app**: search matches branch name too, palette search goes fuzzy
- **app**: merge BRANCH/PR count columns, break CI down by outcome
- **app**: call out peers holding a real branch, not just main
- **app**: show local branch count in Repo List, trim branch names from the right

### Fix

- **demo**: correct the build and flag order, add the PRs tab
- **app**: cap the repo detail Notes panel preview and highlight ! lines
- **app**: make PR and PEERS load for real instead of a permanent dash
- **app**: give BRs its own fetch, spin while BRs/PRs load, stop blocking startup on gh auth

### Refactor

- **app**: move repo identity badges from breadcrumb into Status's preview

### Perf

- **app**: fix quadratic BranchConflictCount, add a scroll-latency regression test

## v1.8.0 (2026-08-13)

### Feat

- **app**: show the working-tree diff in Status and Peers, jump between files
- **vcs**: add uncommitted diff and diffstat reads
- **app**: scroll the PR detail page and move its actions to the footer
- **tui**: highlight note lines flagged with a leading !
- **tui**: divide the notes section from its metadata row
- **tui**: show a notes count badge in the repo list header

### Fix

- **app**: regenerate stale TestGoldenRepoListExpanded fixtures
- **app**: only highlight failing or skipped checks in PR detail
- **app**: stop the PRs tab's status message from sticking
- **app**: match the fleet map's frame width to the repo list
- **app**: default the PRs tab's scope to where it was opened from
- **github**: surface gh's stderr instead of swallowing it
- **app**: match the PRs tab's frame width to the repo list

## v1.7.0 (2026-08-12)

### Feat

- **app**: filter repo-list peers to those sharing an open PR
- **prs**: add a PRs tab with saved GitHub searches and local checkout
- **stash**: render the full patch through the configured diff viewer
- **tui**: narrow the panel column and widen the detail pane on focus
- **tui**: render pull request bodies and open the verb menu as a modal
- **script**: fail on rejected commands and list :commands headlessly
- **list**: drop the side panel and put one expanded region under the table
- **list**: tell a pending cell from an empty one while the fleet loads
- **list**: dismiss the notes panel, then the search, then the filters on esc
- **cache**: persist pull request data per upstream between runs
- **vcs**: serve branch lists and commit logs while the stamp holds
- **cache**: expire entries by checkout stamp as well as by clock
- **vcs**: derive a host-qualified upstream identity for each remote
- **panels**: name the failing checks in the PR detail pane
- **panels**: let a stash verb swap the diffstat for its full diff
- **vcs**: add branch delete and stash apply/drop, wired to the panel verbs
- **panels**: give every panel its own verbs behind the leader key
- **panels**: put every panel verb behind the ! leader, freeing the letter keys
- **panels**: drop empty panels and move each jump key onto its own border
- **list**: close the notes preview with a divider that captions it
- **notes**: spell the note out in the focused view and the list preview
- **repo**: let the focused view use the wide terminal it is given
- **repo**: focus the detail pane with enter, lazygit-style

### Fix

- **cli**: persist the per-branch PR and default-branch CI like the PR list
- **cli**: reject a scan path that is not an existing directory
- **list**: stop the fleet map from discarding the expanded region's cache
- **cache**: rename totalBytes so the coverage gate can find its total
- **cache**: key branch lists and commit logs by checkout, not by object store
- **github**: read the PR list as two filtered pages so a busy repo stops timing out
- **panels**: give every panel an equal share before relevance, and stop padding boxes
- **panels**: pool the unclaimed height in the focused panel
- **panels**: carry j/k across panel boundaries so a one-row panel can't swallow them
- **list**: keep the notes key on offer and say when the terminal is too short
- **list**: keep a discovered worktree out of the fleet list
- **list**: lead the body with the notes preview so the table never moves
- **list**: give the notes preview a fixed region instead of pushing the frame

### Refactor

- give the summary read and the message handlers one path each
- **list**: collapse the breakpoint enum now that wide renders like standard
- **cache**: key remote-derived entries by upstream identity, object-store entries by checkout

## v1.6.0 (2026-08-05)

### Feat

- **app**: add universal find across the fleet
- **app**: replace the focused view's tabs with a panel grid

### Fix

- say what the state actually is
- keep fleet signals scoped and viewpoint-independent
- correct CI, fork, and shared-checkout signals
- make branch cleanup safe and status counts faithful

## v1.5.0 (2026-08-04)

### Feat

- **cli**: report CI, template drift, and alerts for a fleet assessment
- **app**: surface PR activity, PR checkout, and default-branch CI
- **app**: map open PRs onto the branches and checkouts holding them
- **app**: show parallel checkouts and flag shared branches
- **app**: answer the focused repo view on arrival with an overview pane
- **app**: mount a live preview panel beside the wide repo list
- **app**: add breakpoint layouts and collapse the footer by priority
- **app**: size every detail table with the column engine
- **app**: surface command mode in the footer and help
- **ui**: add the shared column engine and put the repo list on it
- expand and center tables

### Fix

- **app**: wrap notes content to the pane width
- **app**: space the peers glyph from its count
- **app**: start search from an empty query
- **app**: pad every table cell by display width
- **vcs**: read stash list with log-format atoms

## v1.4.0 (2026-08-03)

### Feat

- **pr**: show per-check CI rows and the latest comment in PR detail
- **app**: switch, push, open, and squash-merge from the focused repo view
- **vcs**: add branch switch, push --follow-tags, and gh PR create/squash-merge
- **detail**: add PR, checks, and parallel-checkout columns to the branch list
- **app**: open the focused repo view when the scan finds a single repo
- **repos**: surface parallel checkouts of the same remote in the repo list
- **app**: show remote protocol and config override badges in repo detail
- **filters**: add ssh, https, and config_override predicate atoms
- **vcs**: detect remote protocol and diff local git config against global
- **models**: track remote protocol and git config overrides on RepoSummary

### Fix

- **app**: surface status messages in every view, not just PR detail

## v1.3.3 (2026-07-31)

### Fix

- **tui**: remove the p binding that never opened or created a PR

## v1.3.2 (2026-07-30)

### Fix

- **release**: resolve the tap deploy key with envOrDefault and gate the upload on it
- make 1p deploy key idempotent

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
