// Package app implements the Bubble Tea TUI model, update, and view for gh-repo-dashboard.
package app

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// ViewMode identifies which screen the TUI is currently displaying.
type ViewMode int

// ViewMode values.
const (
	ViewModeRepoList ViewMode = iota
	ViewModeRepoDetail
	ViewModeBranchDetail
	ViewModePRDetail
	ViewModeHelp
	ViewModeFilter
	ViewModeSort
	ViewModeBatchProgress
	ViewModeConfirm
	ViewModePRMap
	ViewModePalette
	ViewModePRList
)

// Model is the root Bubble Tea model holding all TUI state.
type Model struct {
	scanPaths []string
	maxDepth  int

	repoPaths []string
	summaries map[string]models.RepoSummary

	filteredPaths []string
	cursor        int
	// expandOpen is the one region the list opens below its table, holding the
	// peers, branches, pull requests, and notes of the repo under the cursor.
	expandOpen bool
	// notesPreview caches each repo's notes read in full, keyed by repo path,
	// so reopening the region or scrolling back never re-reads a file.
	notesPreview map[string][]models.NoteFileContent

	activeFilters []models.ActiveFilter
	activeSorts   []models.ActiveSort
	searchText    string
	searching     bool
	searchInput   textinput.Model

	// headless is set by script mode, where a command that would open a modal
	// has to answer in the status line instead.
	headless             bool
	commandMode          bool
	commandInput         textinput.Model
	registry             Registry
	completionCandidates []string
	completionIndex      int
	commandHistory       []string
	pendingRepeat        bool
	pendingTop           bool

	predicateText string
	predicate     filters.Predicate
	selectedPaths map[string]bool

	pendingOperator string
	pendingObject   string

	viewMode     ViewMode
	selectedRepo string

	// repoStack records the repos a peer-checkout jump came from, so esc walks
	// back through them before leaving the focused view.
	repoStack []string

	// prMap holds the fleet map's per-repo pull requests and branch lists,
	// keyed by repo path and populated only while ":prs" is open.
	prMap       map[string]PRMapLoadedMsg
	prMapCursor int

	// peerBranches holds each peer checkout's local branch list, keyed by the
	// peer's own path rather than by the repo that asked for it, so two rows
	// sharing a peer share one fetch. Populated only once a row's expand
	// region has requested it.
	peerBranches map[string][]models.BranchInfo

	// ciRequested marks the repos a CI fetch has already been issued for, so
	// scrolling back over a row does not re-request it.
	ciRequested map[string]bool
	// ciBranch names the default branch each repo's CI runs belong to.
	ciBranch map[string]string

	// fetching holds the per-repo fetches still in flight, so a cell they would
	// fill reads as pending rather than as empty.
	fetching map[fetchKey]bool

	width        int
	height       int
	loading      bool
	loadingCount int
	loadedCount  int

	detailLoading       bool
	branchDetailLoading bool
	spinner             spinner.Model

	// focusedPanel is which panel of the focused repo view has the cursor.
	// Focus only decides which rows j/k walk first and what the detail pane
	// describes; every panel with content stays on screen either way.
	focusedPanel panelID
	detailCursor int
	// detailFocused moves the keyboard into the detail pane, where j/k scroll
	// the selected item's text instead of moving between rows.
	detailFocused bool
	detailScroll  int
	// prViewIndex selects which of models.PRViews the PRs tab is showing, and
	// prFleet widens that view from this repo to everything the search reaches.
	prViewIndex     int
	prFleet         bool
	prSearch        []models.PRInfo
	prSearchCursor  int
	prSearchLoading bool
	prViewMenu      bool
	prSearchError   string
	// prListReturn is where esc goes from a pull request opened out of the PRs
	// tab, which is the tab rather than the repo grid it was never in.
	prListReturn ViewMode

	// panelActions is the leader key's verb menu, open over the focused panel's
	// selection until a verb runs or any other key backs out.
	panelActions bool

	branches          []models.BranchInfo
	deletableBranches map[string]bool
	stashes           []models.StashDetail
	worktrees         []models.WorktreeInfo
	notesFiles        []models.NoteFileContent

	selectedBranch models.BranchInfo
	branchDetail   models.BranchDetail

	prs     []models.PRInfo
	prCount map[string]int
	// stashDiffstat caches each stash's diffstat by index, filled lazily as
	// the panel cursor lands on one. stashDiff caches the full patch the same
	// way, but only once the Stashes panel's full-diff verb asks for it.
	stashDiffstat map[int]string
	stashDiff     map[int]string
	stashFullDiff bool
	selectedPR    models.PRInfo
	prDetail      models.PRDetail

	// Universal find state. The palette answers from cached data only, so it
	// holds no results of its own: they are recomputed from the query.
	paletteInput      textinput.Model
	paletteCursor     int
	paletteMarks      map[string]bool
	paletteFleetScope bool
	paletteActions    bool
	paletteReturnMode ViewMode

	filterCursor int
	sortCursor   int

	batchRunning  bool
	batchTask     string
	batchResults  []BatchResult
	batchProgress int
	batchTotal    int

	statusMessage string
	pendingAction *pendingAction

	keys KeyMap
	help help.Model
}

// New builds the initial Model for the given repo scan roots.
func New(scanPaths []string, maxDepth int) Model {
	ti := textinput.New()
	ti.Placeholder = "Search repos..."
	ti.CharLimit = 100
	// textinput truncates its placeholder to a single rune at width zero.
	ti.SetWidth(lipgloss.Width(ti.Placeholder))

	ci := textinput.New()
	ci.Prompt = ":"
	ci.CharLimit = 200

	pi := textinput.New()
	pi.Prompt = ""
	pi.CharLimit = 100
	pi.SetWidth(paletteInputWidth)

	activeFilters := make([]models.ActiveFilter, 0, len(models.AllFilterModes()))
	for _, mode := range models.AllFilterModes() {
		activeFilters = append(activeFilters, models.NewActiveFilter(mode))
	}

	sorts := make([]models.ActiveSort, 0, len(models.AllSortModes()))
	for i, mode := range models.AllSortModes() {
		sort := models.NewActiveSort(mode, i)
		if mode == models.SortModeName {
			sort.Direction = models.SortDirectionAsc
		}
		sorts = append(sorts, sort)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(styles.Blue)

	return Model{
		spinner:       sp,
		scanPaths:     scanPaths,
		maxDepth:      maxDepth,
		summaries:     make(map[string]models.RepoSummary),
		notesPreview:  make(map[string][]models.NoteFileContent),
		prCount:       make(map[string]int),
		prMap:         make(map[string]PRMapLoadedMsg),
		peerBranches:  make(map[string][]models.BranchInfo),
		activeFilters: activeFilters,
		activeSorts:   sorts,
		searchInput:   ti,
		commandInput:  ci,
		paletteInput:  pi,
		registry:      DefaultRegistry(),
		viewMode:      ViewModeRepoList,
		loading:       true,
		keys:          DefaultKeyMap(),
		help:          help.New(),
	}
}

// paletteInputWidth is the find line's typing room; results wrap below it, so
// the line itself never needs the full frame.
const paletteInputWidth = 40

// Init kicks off the initial repo discovery command.
func (m Model) Init() tea.Cmd {
	return tea.Batch(discoverReposCmd(m.scanPaths, m.maxDepth), m.spinner.Tick)
}

// anyLoading reports whether any view is waiting on data. The spinner ticks
// only while this holds, so a settled dashboard does not repaint on a timer.
func (m Model) anyLoading() bool {
	if m.viewMode == ViewModePRDetail && m.prDetail.Number == 0 {
		return true
	}

	return m.loading || m.detailLoading || m.branchDetailLoading || m.fetchesInFlight() > 0
}

// CurrentFilter returns the single active, non-inverted filter mode, or FilterModeAll if none is set.
func (m Model) CurrentFilter() models.FilterMode {
	for _, f := range m.activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			return f.Mode
		}
	}

	return models.FilterModeAll
}

// ActiveFilterModes returns all enabled, non-inverted filter modes.
func (m Model) ActiveFilterModes() []models.FilterMode {
	var modes []models.FilterMode
	for _, f := range m.activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			modes = append(modes, f.Mode)
		}
	}

	return modes
}

// filtersActive reports whether anything is narrowing the fleet, counting an
// inverted filter and a predicate alongside a plain one.
func (m Model) filtersActive() bool {
	if m.predicateText != "" || m.predicate != nil {
		return true
	}

	for _, f := range m.activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			return true
		}
	}

	return false
}

// SetFilter enables only the given filter mode, disabling all others.
func (m *Model) SetFilter(mode models.FilterMode) {
	for i := range m.activeFilters {
		m.activeFilters[i].Enabled = m.activeFilters[i].Mode == mode
	}
}

// CycleFilterState advances the given filter mode through off -> enabled -> inverted -> off.
func (m *Model) CycleFilterState(mode models.FilterMode) {
	if mode == models.FilterModeAll {
		return
	}

	for i := range m.activeFilters {
		if m.activeFilters[i].Mode == mode {
			switch {
			case !m.activeFilters[i].Enabled:
				m.activeFilters[i].Enabled = true
				m.activeFilters[i].Inverted = false
			case !m.activeFilters[i].Inverted:
				m.activeFilters[i].Inverted = true
			default:
				m.activeFilters[i].Enabled = false
				m.activeFilters[i].Inverted = false
			}
		}
	}
}

// CycleFilter advances the single-selection filter to the next filter mode.
func (m *Model) CycleFilter() {
	current := m.CurrentFilter()
	modes := models.AllFilterModes()
	for i, mode := range modes {
		if mode == current {
			next := modes[(i+1)%len(modes)]
			m.SetFilter(next)

			return
		}
	}
	m.SetFilter(models.FilterModeAll)
}

// CycleSortState advances the given sort mode through off -> ascending -> descending -> off.
func (m *Model) CycleSortState(mode models.SortMode) {
	for i := range m.activeSorts {
		if m.activeSorts[i].Mode != mode {
			continue
		}

		switch m.activeSorts[i].Direction {
		case models.SortDirectionOff:
			m.activateSort(i)
		case models.SortDirectionAsc:
			m.activeSorts[i].Direction = models.SortDirectionDesc
		case models.SortDirectionDesc:
			m.deactivateSort(i)
		}
	}
}

// activateSort turns on the sort at index i (off -> ascending), assigning it
// the next available priority after any already-enabled sorts.
func (m *Model) activateSort(i int) {
	highestPriority := -1
	for _, s := range m.activeSorts {
		if s.IsEnabled() && s.Priority > highestPriority {
			highestPriority = s.Priority
		}
	}
	m.activeSorts[i].Direction = models.SortDirectionAsc
	m.activeSorts[i].Priority = highestPriority + 1
}

// deactivateSort turns off the sort at index i (descending -> off), compacting
// the priorities of any sorts that were ranked below it.
func (m *Model) deactivateSort(i int) {
	freedPriority := m.activeSorts[i].Priority
	m.activeSorts[i].Direction = models.SortDirectionOff
	m.activeSorts[i].Priority = len(m.activeSorts)
	for j := range m.activeSorts {
		if m.activeSorts[j].IsEnabled() && m.activeSorts[j].Priority > freedPriority {
			m.activeSorts[j].Priority--
		}
	}
}

// MoveSortUp raises the priority of the sort at the cursor, swapping with the sort above it.
func (m *Model) MoveSortUp() {
	if m.sortCursor < 0 || m.sortCursor >= len(m.activeSorts) {
		return
	}

	currentSort := &m.activeSorts[m.sortCursor]
	if !currentSort.IsEnabled() || currentSort.Priority == 0 {
		return
	}

	for i := range m.activeSorts {
		if m.activeSorts[i].IsEnabled() && m.activeSorts[i].Priority == currentSort.Priority-1 {
			m.activeSorts[i].Priority++
			currentSort.Priority--

			return
		}
	}
}

// MoveSortDown lowers the priority of the sort at the cursor, swapping with the sort below it.
func (m *Model) MoveSortDown() {
	if m.sortCursor < 0 || m.sortCursor >= len(m.activeSorts) {
		return
	}

	currentSort := &m.activeSorts[m.sortCursor]
	if !currentSort.IsEnabled() {
		return
	}

	maxPriority := -1
	for _, s := range m.activeSorts {
		if s.IsEnabled() && s.Priority > maxPriority {
			maxPriority = s.Priority
		}
	}

	if currentSort.Priority >= maxPriority {
		return
	}

	for i := range m.activeSorts {
		if m.activeSorts[i].IsEnabled() && m.activeSorts[i].Priority == currentSort.Priority+1 {
			m.activeSorts[i].Priority--
			currentSort.Priority++

			return
		}
	}
}

// ResetFilters disables all filters, restores the default "all" mode, and clears any predicate.
func (m *Model) ResetFilters() {
	for i := range m.activeFilters {
		m.activeFilters[i].Enabled = m.activeFilters[i].Mode == models.FilterModeAll
		m.activeFilters[i].Inverted = false
	}
	m.predicate = nil
	m.predicateText = ""
}

// SetPredicate sets the active filter predicate and its source text.
func (m *Model) SetPredicate(text string, pred filters.Predicate) {
	m.predicate = pred
	m.predicateText = text
}

// SelectedCount returns the number of repos currently selected for batch operations.
func (m Model) SelectedCount() int {
	return len(m.selectedPaths)
}

// ResetSorts restores all sorts to their default direction and priority order.
func (m *Model) ResetSorts() {
	for i := range m.activeSorts {
		if m.activeSorts[i].Mode == models.SortModeName {
			m.activeSorts[i].Direction = models.SortDirectionAsc
		} else {
			m.activeSorts[i].Direction = models.SortDirectionOff
		}
		m.activeSorts[i].Priority = i
	}
}

// DirtyCount returns the number of visible repos with uncommitted changes.
func (m Model) DirtyCount() int {
	return m.countVisible(func(s models.RepoSummary) bool { return s.IsDirty() })
}

// PRCount returns the number of visible repos with an associated pull request.
func (m Model) PRCount() int {
	return m.countVisible(func(s models.RepoSummary) bool { return s.PRInfo != nil })
}

// countVisible counts the filtered repos matching match. Every header count
// shares this scope, so a filter narrows all of them together rather than
// leaving one fleet-wide number beside a filtered one.
func (m Model) countVisible(match func(models.RepoSummary) bool) int {
	count := 0
	for _, path := range m.filteredPaths {
		if summary, ok := m.summaries[path]; ok && match(summary) {
			count++
		}
	}

	return count
}

// PeerCheckouts returns the other discovered repos that share path's remote,
// so parallel checkouts of one project are visible from any of them.
func (m Model) PeerCheckouts(path string) []models.PeerCheckout {
	summary, ok := m.summaries[path]
	if !ok {
		return nil
	}

	all := make([]models.RepoSummary, 0, len(m.summaries))
	for key := range m.summaries {
		all = append(all, m.summaries[key])
	}

	return models.FindPeerCheckouts(&summary, all)
}

// RepoCheckouts returns every parallel checkout of the selected repo: sibling
// clones found by discovery plus its own worktrees/workspaces.
func (m Model) RepoCheckouts() []models.PeerCheckout {
	return models.MergeCheckouts(
		m.PeerCheckouts(m.selectedRepo),
		models.WorktreeCheckouts(m.selectedRepo, m.worktrees),
	)
}

// listableRepos are the discovered repos the fleet list stands for. A linked
// checkout whose parent was discovered too is dropped: a worktree or jj
// workspace is a place inside its parent repo, and the parent's Peers panel
// already names it, so listing it again would double-count the fleet. Scanned
// on its own, with no parent in the set, it is still the repo you are in and
// stays listed.
func (m Model) listableRepos() []string {
	paths := make([]string, 0, len(m.repoPaths))

	for _, path := range m.repoPaths {
		parent := m.summaries[path].ParentPath
		if _, discovered := m.summaries[parent]; parent != "" && discovered {
			continue
		}

		paths = append(paths, path)
	}

	return paths
}

// SelectedSummary returns the RepoSummary at the cursor and whether it was found.
func (m Model) SelectedSummary() (models.RepoSummary, bool) {
	if m.cursor >= 0 && m.cursor < len(m.filteredPaths) {
		path := m.filteredPaths[m.cursor]
		if summary, ok := m.summaries[path]; ok {
			return summary, true
		}
	}

	return models.RepoSummary{}, false
}
