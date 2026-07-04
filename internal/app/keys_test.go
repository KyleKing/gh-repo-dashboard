package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func keyPress(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func newListModel(paths ...string) Model {
	m := New(nil, 1)
	m.loading = false
	m.repoPaths = paths
	for _, p := range paths {
		m.summaries[p] = models.RepoSummary{Path: p}
	}
	m.updateFilteredPaths()

	return m
}

func TestHelpToggle(t *testing.T) {
	m := New(nil, 1)

	updatedModel, _ := m.Update(keyPress('?'))
	m = mustModel(t, updatedModel)
	if m.viewMode != ViewModeHelp {
		t.Errorf("expected ViewModeHelp, got %v", m.viewMode)
	}

	updatedModel, _ = m.Update(keyPress('?'))
	m = mustModel(t, updatedModel)
	if m.viewMode != ViewModeRepoList {
		t.Errorf("expected ViewModeRepoList after toggle, got %v", m.viewMode)
	}
}

func TestQuitKey(t *testing.T) {
	m := New(nil, 1)

	_, cmd := m.Update(keyPress('q'))
	if cmd == nil {
		t.Error("quit key should return a command")
	}
}

func TestFilterModalOpenClose(t *testing.T) {
	m := New(nil, 1)

	updatedModel, _ := m.Update(keyPress('f'))
	m = mustModel(t, updatedModel)
	if m.viewMode != ViewModeFilter {
		t.Errorf("expected ViewModeFilter, got %v", m.viewMode)
	}

	updatedModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mustModel(t, updatedModel)
	if m.viewMode != ViewModeRepoList {
		t.Errorf("expected ViewModeRepoList after esc, got %v", m.viewMode)
	}
}

func TestSortModalOpenClose(t *testing.T) {
	m := New(nil, 1)
	m.sortCursor = 3

	updatedModel, _ := m.Update(keyPress('s'))
	m = mustModel(t, updatedModel)
	if m.viewMode != ViewModeSort {
		t.Errorf("expected ViewModeSort, got %v", m.viewMode)
	}
	if m.sortCursor != 0 {
		t.Errorf("opening sort modal should reset sortCursor, got %d", m.sortCursor)
	}

	updatedModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mustModel(t, updatedModel)
	if m.viewMode != ViewModeRepoList {
		t.Errorf("expected ViewModeRepoList after esc, got %v", m.viewMode)
	}
}

func TestEnterOpensRepoDetail(t *testing.T) {
	m := newListModel("/alpha", "/beta")
	m.cursor = 1
	m.detailTab = DetailTabPRs
	m.detailCursor = 3

	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)

	if m.viewMode != ViewModeRepoDetail {
		t.Errorf("expected ViewModeRepoDetail, got %v", m.viewMode)
	}
	if m.selectedRepo != "/beta" {
		t.Errorf("expected selectedRepo /beta, got %q", m.selectedRepo)
	}
	if m.detailTab != DetailTabBranches || m.detailCursor != 0 {
		t.Error("entering detail should reset tab and cursor")
	}
	if cmd == nil {
		t.Error("entering detail should return a load command")
	}
}

func TestEnterOnEmptyListIsNoop(t *testing.T) {
	m := newListModel()

	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)

	if m.viewMode != ViewModeRepoList {
		t.Errorf("empty list enter should not change view mode, got %v", m.viewMode)
	}
	if m.selectedRepo != "" {
		t.Errorf("empty list enter should not select a repo, got %q", m.selectedRepo)
	}
	if cmd != nil {
		t.Error("empty list enter should not return a command")
	}
}

func TestCursorMovement(t *testing.T) {
	tests := []struct {
		name       string
		paths      []string
		cursor     int
		key        tea.KeyPressMsg
		wantCursor int
	}{
		{"down moves", []string{"/a", "/b", "/c"}, 0, keyPress('j'), 1},
		{"down clamps at bottom", []string{"/a", "/b"}, 1, keyPress('j'), 1},
		{"up moves", []string{"/a", "/b"}, 1, keyPress('k'), 0},
		{"up clamps at top", []string{"/a", "/b"}, 0, keyPress('k'), 0},
		{"g goes to top", []string{"/a", "/b", "/c"}, 2, keyPress('g'), 0},
		{"G goes to bottom", []string{"/a", "/b", "/c"}, 0, keyPress('G'), 2},
		{"G on empty list stays 0", nil, 0, keyPress('G'), 0},
		{"down on empty list stays 0", nil, 0, keyPress('j'), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newListModel(tt.paths...)
			m.cursor = tt.cursor

			updatedModel, _ := m.Update(tt.key)
			m = mustModel(t, updatedModel)

			if m.cursor != tt.wantCursor {
				t.Errorf("expected cursor %d, got %d", tt.wantCursor, m.cursor)
			}
		})
	}
}

func TestBackNavigationChain(t *testing.T) {
	tests := []struct {
		name string
		from ViewMode
		want ViewMode
	}{
		{"repo detail to list", ViewModeRepoDetail, ViewModeRepoList},
		{"branch detail to repo detail", ViewModeBranchDetail, ViewModeRepoDetail},
		{"PR detail to repo detail", ViewModePRDetail, ViewModeRepoDetail},
		{"help to list", ViewModeHelp, ViewModeRepoList},
		{"filter to list", ViewModeFilter, ViewModeRepoList},
		{"sort to list", ViewModeSort, ViewModeRepoList},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(nil, 1)
			m.viewMode = tt.from

			updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			m = mustModel(t, updatedModel)

			if m.viewMode != tt.want {
				t.Errorf("expected %v, got %v", tt.want, m.viewMode)
			}
		})
	}
}

func TestDetailTabCycling(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail

	expected := []DetailTab{DetailTabStashes, DetailTabWorktrees, DetailTabPRs, DetailTabBranches}
	for i, want := range expected {
		updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		m = mustModel(t, updatedModel)
		if m.detailTab != want {
			t.Errorf("tab press %d: expected %v, got %v", i+1, want, m.detailTab)
		}
	}
}

func TestDetailTabLeftWrapsBackward(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabBranches
	m.detailCursor = 2

	updatedModel, _ := m.Update(keyPress('h'))
	m = mustModel(t, updatedModel)

	if m.detailTab != DetailTabPRs {
		t.Errorf("left from first tab should wrap to PRs, got %v", m.detailTab)
	}
	if m.detailCursor != 0 {
		t.Errorf("tab switch should reset detail cursor, got %d", m.detailCursor)
	}
}

func TestDetailCursorMovement(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabBranches
	m.branches = []models.BranchInfo{{Name: "a"}, {Name: "b"}, {Name: "c"}}

	updatedModel, _ := m.Update(keyPress('j'))
	m = mustModel(t, updatedModel)
	if m.detailCursor != 1 {
		t.Errorf("expected detailCursor 1, got %d", m.detailCursor)
	}

	updatedModel, _ = m.Update(keyPress('G'))
	m = mustModel(t, updatedModel)
	if m.detailCursor != 2 {
		t.Errorf("G should move to last item, got %d", m.detailCursor)
	}

	updatedModel, _ = m.Update(keyPress('j'))
	m = mustModel(t, updatedModel)
	if m.detailCursor != 2 {
		t.Errorf("down at bottom should clamp, got %d", m.detailCursor)
	}

	updatedModel, _ = m.Update(keyPress('g'))
	m = mustModel(t, updatedModel)
	if m.detailCursor != 0 {
		t.Errorf("g should move to top, got %d", m.detailCursor)
	}

	updatedModel, _ = m.Update(keyPress('k'))
	m = mustModel(t, updatedModel)
	if m.detailCursor != 0 {
		t.Errorf("up at top should clamp, got %d", m.detailCursor)
	}
}

func TestDetailBottomOnEmptyTab(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabStashes

	updatedModel, _ := m.Update(keyPress('G'))
	m = mustModel(t, updatedModel)

	if m.detailCursor != 0 {
		t.Errorf("G on empty tab should keep cursor 0, got %d", m.detailCursor)
	}
}

func TestDetailEnterOpensBranchDetail(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/repo1"
	m.detailTab = DetailTabBranches
	m.detailCursor = 1
	m.branches = []models.BranchInfo{{Name: "main"}, {Name: "feature"}}
	m.branchDetail = models.BranchDetail{Branch: models.BranchInfo{Name: "stale"}}

	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)

	if m.viewMode != ViewModeBranchDetail {
		t.Errorf("expected ViewModeBranchDetail, got %v", m.viewMode)
	}
	if m.selectedBranch.Name != "feature" {
		t.Errorf("expected selected branch 'feature', got %q", m.selectedBranch.Name)
	}
	if m.branchDetail.Branch.Name != "" {
		t.Error("previous branch detail should be cleared")
	}
	if cmd == nil {
		t.Error("branch detail entry should return a load command")
	}
}

func TestFilterModalEnterCyclesState(t *testing.T) {
	m := newListModel("/alpha")
	m.viewMode = ViewModeFilter
	m.filterCursor = 0
	mode := models.SelectableFilterModes()[0]

	find := func(m Model) models.ActiveFilter {
		for _, f := range m.activeFilters {
			if f.Mode == mode {
				return f
			}
		}
		t.Fatalf("filter mode %v not found", mode)

		return models.ActiveFilter{}
	}

	updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)
	if f := find(m); !f.Enabled || f.Inverted {
		t.Error("first enter should enable the filter")
	}

	updatedModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)
	if f := find(m); !f.Enabled || !f.Inverted {
		t.Error("second enter should invert the filter")
	}

	updatedModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)
	if f := find(m); f.Enabled || f.Inverted {
		t.Error("third enter should disable the filter")
	}
}

func TestFilterModalShortKey(t *testing.T) {
	m := newListModel("/alpha")
	m.viewMode = ViewModeFilter
	m.cursor = 0

	updatedModel, _ := m.Update(keyPress('d'))
	m = mustModel(t, updatedModel)

	for _, f := range m.activeFilters {
		if f.Mode == models.FilterModeDirty && !f.Enabled {
			t.Error("'d' should enable the dirty filter")
		}
	}
	if m.cursor != 0 {
		t.Errorf("filter change should reset cursor, got %d", m.cursor)
	}
}

func TestFilterModalReset(t *testing.T) {
	m := newListModel("/alpha")
	m.viewMode = ViewModeFilter
	m.CycleFilterState(models.FilterModeAhead)
	m.CycleFilterState(models.FilterModeDirty)

	updatedModel, _ := m.Update(keyPress('*'))
	m = mustModel(t, updatedModel)

	if len(m.ActiveFilterModes()) != 0 {
		t.Error("'*' should reset all filters")
	}
}

func TestFilterModalCursorClamping(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeFilter
	modes := models.SelectableFilterModes()

	updatedModel, _ := m.Update(keyPress('k'))
	m = mustModel(t, updatedModel)
	if m.filterCursor != 0 {
		t.Errorf("up at top should clamp filterCursor to 0, got %d", m.filterCursor)
	}

	m.filterCursor = len(modes) - 1
	updatedModel, _ = m.Update(keyPress('j'))
	m = mustModel(t, updatedModel)
	if m.filterCursor != len(modes)-1 {
		t.Errorf("down at bottom should clamp filterCursor, got %d", m.filterCursor)
	}

	m.filterCursor = 0
	updatedModel, _ = m.Update(keyPress('j'))
	m = mustModel(t, updatedModel)
	if m.filterCursor != 1 {
		t.Errorf("down should advance filterCursor, got %d", m.filterCursor)
	}
}

func TestSortModalEnterCyclesDirection(t *testing.T) {
	m := newListModel("/alpha")
	m.viewMode = ViewModeSort
	m.sortCursor = 0

	updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)

	if m.activeSorts[0].Direction != models.SortDirectionDesc {
		t.Errorf("Name starts Asc, enter should cycle to Desc, got %v", m.activeSorts[0].Direction)
	}
}

func TestSortModalPriorityMoves(t *testing.T) {
	m := newListModel("/alpha")
	m.viewMode = ViewModeSort
	m.CycleSortState(models.SortModeModified)

	var nameIdx, modIdx int
	for i, s := range m.activeSorts {
		switch s.Mode {
		case models.SortModeName:
			nameIdx = i
		case models.SortModeModified:
			modIdx = i
		default:
			// only name/modified indices are needed below
		}
	}

	m.activeSorts[nameIdx].Priority = 0
	m.activeSorts[modIdx].Priority = 1
	m.sortCursor = modIdx
	updatedModel, _ := m.Update(keyPress('['))
	m = mustModel(t, updatedModel)

	if m.activeSorts[modIdx].Priority != 0 {
		t.Errorf("'[' should move Modified to priority 0, got %d", m.activeSorts[modIdx].Priority)
	}
	if m.activeSorts[nameIdx].Priority != 1 {
		t.Errorf("'[' should demote Name to priority 1, got %d", m.activeSorts[nameIdx].Priority)
	}

	updatedModel, _ = m.Update(keyPress(']'))
	m = mustModel(t, updatedModel)

	if m.activeSorts[modIdx].Priority != 1 {
		t.Errorf("']' should move Modified back to priority 1, got %d", m.activeSorts[modIdx].Priority)
	}
	if m.activeSorts[nameIdx].Priority != 0 {
		t.Errorf("']' should restore Name to priority 0, got %d", m.activeSorts[nameIdx].Priority)
	}
}

func TestSortModalReset(t *testing.T) {
	m := newListModel("/alpha")
	m.viewMode = ViewModeSort
	m.CycleSortState(models.SortModeModified)
	m.CycleSortState(models.SortModeStatus)

	updatedModel, _ := m.Update(keyPress('*'))
	m = mustModel(t, updatedModel)

	for _, s := range m.activeSorts {
		if s.Mode == models.SortModeName {
			if s.Direction != models.SortDirectionAsc {
				t.Error("Name should be Asc after reset")
			}
		} else if s.Direction != models.SortDirectionOff {
			t.Errorf("%v should be Off after reset", s.Mode)
		}
	}
}

func TestSortModalShortKey(t *testing.T) {
	m := newListModel("/alpha")
	m.viewMode = ViewModeSort

	updatedModel, _ := m.Update(keyPress('m'))
	m = mustModel(t, updatedModel)

	for _, s := range m.activeSorts {
		if s.Mode == models.SortModeModified && s.Direction != models.SortDirectionAsc {
			t.Errorf("'m' should cycle Modified to Asc, got %v", s.Direction)
		}
	}
}

func TestSearchModeEntry(t *testing.T) {
	m := New(nil, 1)

	updatedModel, _ := m.Update(keyPress('/'))
	m = mustModel(t, updatedModel)

	if !m.searching {
		t.Error("'/' should enter search mode")
	}
	if !m.searchInput.Focused() {
		t.Error("search input should be focused")
	}
}

func TestSearchEscExits(t *testing.T) {
	m := New(nil, 1)
	m.searching = true
	m.searchInput.Focus()

	updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mustModel(t, updatedModel)

	if m.searching {
		t.Error("esc should exit search mode")
	}
	if m.searchInput.Focused() {
		t.Error("esc should blur search input")
	}
}

func TestSearchEnterCommits(t *testing.T) {
	m := newListModel("/alpha", "/beta")
	m.searching = true
	m.searchInput.Focus()
	m.searchInput.SetValue("alpha")

	updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, updatedModel)

	if m.searching {
		t.Error("enter should exit search mode")
	}
	if m.searchText != "alpha" {
		t.Errorf("expected searchText 'alpha', got %q", m.searchText)
	}
	if len(m.filteredPaths) != 1 || m.filteredPaths[0] != "/alpha" {
		t.Errorf("expected filtered to /alpha, got %v", m.filteredPaths)
	}
	if m.cursor != 0 {
		t.Errorf("search should reset cursor, got %d", m.cursor)
	}
}

func TestSearchTypingUpdatesLive(t *testing.T) {
	m := newListModel("/alpha", "/beta")
	m.searching = true
	m.searchInput.Focus()

	updatedModel, _ := m.Update(keyPress('b'))
	m = mustModel(t, updatedModel)

	if !m.searching {
		t.Error("typing should stay in search mode")
	}
	if m.searchText != "b" {
		t.Errorf("expected live searchText 'b', got %q", m.searchText)
	}
	if len(m.filteredPaths) != 1 || m.filteredPaths[0] != "/beta" {
		t.Errorf("expected filtered to /beta, got %v", m.filteredPaths)
	}
}

func TestBatchKeyBlockedWhileRunning(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeBatchProgress
	m.batchRunning = true

	_, cmd := m.Update(keyPress('q'))
	if cmd != nil {
		t.Error("quit should be blocked while batch is running")
	}

	updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m2 := mustModel(t, updatedModel)
	if m2.viewMode != ViewModeBatchProgress {
		t.Error("back should be blocked while batch is running")
	}
}

func TestBatchKeyAfterCompletion(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeBatchProgress
	m.batchRunning = false

	updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m2 := mustModel(t, updatedModel)
	if m2.viewMode != ViewModeRepoList {
		t.Errorf("back after completion should return to repo list, got %v", m2.viewMode)
	}

	_, cmd := m.Update(keyPress('q'))
	if cmd == nil {
		t.Error("quit after completion should return a command")
	}
}
