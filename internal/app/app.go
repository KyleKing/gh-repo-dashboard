// Package app implements the Bubble Tea TUI model, update, and view for gh-repo-dashboard.
package app

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
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
)

// DetailTab identifies which tab is active on the repo detail screen.
type DetailTab int

// DetailTab values.
const (
	DetailTabBranches DetailTab = iota
	DetailTabStashes
	DetailTabWorktrees
	DetailTabPRs
)

// detailTabCount is the number of DetailTab values, used to cycle tabs.
const detailTabCount = 4

// Model is the root Bubble Tea model holding all TUI state.
type Model struct {
	scanPaths []string
	maxDepth  int

	repoPaths []string
	summaries map[string]models.RepoSummary

	filteredPaths []string
	cursor        int

	activeFilters []models.ActiveFilter
	activeSorts   []models.ActiveSort
	searchText    string
	searching     bool
	searchInput   textinput.Model

	commandMode          bool
	commandInput         textinput.Model
	registry             Registry
	completionCandidates []string
	completionIndex      int

	predicateText string
	predicate     filters.Predicate
	selectedPaths map[string]bool

	pendingOperator string
	pendingObject   string

	viewMode     ViewMode
	selectedRepo string
	width        int
	height       int
	loading      bool
	loadingCount int
	loadedCount  int

	detailTab    DetailTab
	detailCursor int
	branches     []models.BranchInfo
	stashes      []models.StashDetail
	worktrees    []models.WorktreeInfo

	selectedBranch models.BranchInfo
	branchDetail   models.BranchDetail

	prs        []models.PRInfo
	prCount    map[string]int
	selectedPR models.PRInfo
	prDetail   models.PRDetail

	filterCursor int
	sortCursor   int

	batchRunning  bool
	batchTask     string
	batchResults  []BatchResult
	batchProgress int
	batchTotal    int

	statusMessage string

	keys KeyMap
	help help.Model
}

// New builds the initial Model for the given repo scan roots.
func New(scanPaths []string, maxDepth int) Model {
	ti := textinput.New()
	ti.Placeholder = "Search repos..."
	ti.CharLimit = 100

	ci := textinput.New()
	ci.Prompt = ":"
	ci.CharLimit = 200

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

	return Model{
		scanPaths:     scanPaths,
		maxDepth:      maxDepth,
		summaries:     make(map[string]models.RepoSummary),
		prCount:       make(map[string]int),
		activeFilters: activeFilters,
		activeSorts:   sorts,
		searchInput:   ti,
		commandInput:  ci,
		registry:      DefaultRegistry(),
		viewMode:      ViewModeRepoList,
		loading:       true,
		keys:          DefaultKeyMap(),
		help:          help.New(),
	}
}

// Init kicks off the initial repo discovery command.
func (m Model) Init() tea.Cmd {
	return discoverReposCmd(m.scanPaths, m.maxDepth)
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
			if !m.activeFilters[i].Enabled {
				m.activeFilters[i].Enabled = true
				m.activeFilters[i].Inverted = false
			} else if !m.activeFilters[i].Inverted {
				m.activeFilters[i].Inverted = true
			} else {
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
		if m.activeSorts[i].Mode == mode {
			switch m.activeSorts[i].Direction {
			case models.SortDirectionOff:
				m.activeSorts[i].Direction = models.SortDirectionAsc
				highestPriority := -1
				for _, s := range m.activeSorts {
					if s.IsEnabled() && s.Priority > highestPriority {
						highestPriority = s.Priority
					}
				}
				m.activeSorts[i].Priority = highestPriority + 1
			case models.SortDirectionAsc:
				m.activeSorts[i].Direction = models.SortDirectionDesc
			case models.SortDirectionDesc:
				m.activeSorts[i].Direction = models.SortDirectionOff
				m.activeSorts[i].Priority = len(m.activeSorts)
			}
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

// DirtyCount returns the number of repos with uncommitted changes.
func (m Model) DirtyCount() int {
	count := 0
	for _, s := range m.summaries {
		if s.IsDirty() {
			count++
		}
	}

	return count
}

// PRCount returns the number of repos with an associated pull request.
func (m Model) PRCount() int {
	count := 0
	for _, s := range m.summaries {
		if s.PRInfo != nil {
			count++
		}
	}

	return count
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
