package app

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type ViewMode int

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

type DetailTab int

const (
	DetailTabBranches DetailTab = iota
	DetailTabStashes
	DetailTabWorktrees
	DetailTabPRs
)

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

	viewMode       ViewMode
	selectedRepo   string
	width          int
	height         int
	loading        bool
	loadingCount   int
	loadedCount    int

	detailTab      DetailTab
	detailCursor   int
	branches       []models.BranchInfo
	stashes        []models.StashDetail
	worktrees      []models.WorktreeInfo

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

func New(scanPaths []string, maxDepth int) Model {
	ti := textinput.New()
	ti.Placeholder = "Search repos..."
	ti.CharLimit = 100

	filters := make([]models.ActiveFilter, 0, len(models.AllFilterModes()))
	for _, mode := range models.AllFilterModes() {
		filters = append(filters, models.NewActiveFilter(mode))
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
		activeFilters: filters,
		activeSorts:   sorts,
		searchInput:   ti,
		viewMode:      ViewModeRepoList,
		loading:       true,
		keys:          DefaultKeyMap(),
		help:          help.New(),
	}
}

func (m Model) Init() tea.Cmd {
	return discoverReposCmd(m.scanPaths, m.maxDepth)
}

func (m Model) CurrentFilter() models.FilterMode {
	for _, f := range m.activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			return f.Mode
		}
	}
	return models.FilterModeAll
}

func (m Model) ActiveFilterModes() []models.FilterMode {
	var modes []models.FilterMode
	for _, f := range m.activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			modes = append(modes, f.Mode)
		}
	}
	return modes
}

func (m *Model) SetFilter(mode models.FilterMode) {
	for i := range m.activeFilters {
		m.activeFilters[i].Enabled = m.activeFilters[i].Mode == mode
	}
}

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

func (m *Model) ResetFilters() {
	for i := range m.activeFilters {
		m.activeFilters[i].Enabled = m.activeFilters[i].Mode == models.FilterModeAll
		m.activeFilters[i].Inverted = false
	}
}

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

func (m Model) DirtyCount() int {
	count := 0
	for _, s := range m.summaries {
		if s.IsDirty() {
			count++
		}
	}
	return count
}

func (m Model) PRCount() int {
	count := 0
	for _, s := range m.summaries {
		if s.PRInfo != nil {
			count++
		}
	}
	return count
}

func (m Model) SelectedSummary() (models.RepoSummary, bool) {
	if m.cursor >= 0 && m.cursor < len(m.filteredPaths) {
		path := m.filteredPaths[m.cursor]
		if summary, ok := m.summaries[path]; ok {
			return summary, true
		}
	}
	return models.RepoSummary{}, false
}
