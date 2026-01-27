package app

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestNewModel(t *testing.T) {
	m := New([]string{"/path"}, 2)

	if len(m.scanPaths) != 1 || m.scanPaths[0] != "/path" {
		t.Errorf("unexpected scanPaths: %v", m.scanPaths)
	}
	if m.maxDepth != 2 {
		t.Errorf("expected maxDepth=2, got %d", m.maxDepth)
	}
	if m.summaries == nil {
		t.Error("summaries should be initialized")
	}
	if !m.loading {
		t.Error("should start in loading state")
	}
	if m.viewMode != ViewModeRepoList {
		t.Error("should start in repo list view")
	}
}

func TestModelFilterInitialization(t *testing.T) {
	m := New(nil, 1)

	if len(m.activeFilters) != len(models.AllFilterModes()) {
		t.Errorf("expected %d filters, got %d", len(models.AllFilterModes()), len(m.activeFilters))
	}

	enabledCount := 0
	for _, f := range m.activeFilters {
		if f.Enabled {
			enabledCount++
			if f.Mode != models.FilterModeAll {
				t.Error("only FilterModeAll should be enabled by default")
			}
		}
	}
	if enabledCount != 1 {
		t.Errorf("expected 1 enabled filter, got %d", enabledCount)
	}
}

func TestModelSortInitialization(t *testing.T) {
	m := New(nil, 1)

	if len(m.activeSorts) != len(models.AllSortModes()) {
		t.Errorf("expected %d sorts, got %d", len(models.AllSortModes()), len(m.activeSorts))
	}

	for _, s := range m.activeSorts {
		if s.Mode == models.SortModeName && s.Direction != models.SortDirectionAsc {
			t.Error("SortModeName should be Asc by default")
		}
	}
}

func TestModelCurrentFilter(t *testing.T) {
	m := New(nil, 1)

	if m.CurrentFilter() != models.FilterModeAll {
		t.Errorf("expected FilterModeAll, got %v", m.CurrentFilter())
	}

	m.activeFilters[0].Enabled = false
	for i := range m.activeFilters {
		if m.activeFilters[i].Mode == models.FilterModeAhead {
			m.activeFilters[i].Enabled = true
			break
		}
	}

	if m.CurrentFilter() != models.FilterModeAhead {
		t.Errorf("expected FilterModeAhead, got %v", m.CurrentFilter())
	}
}

func TestModelSetFilter(t *testing.T) {
	m := New(nil, 1)

	m.SetFilter(models.FilterModeDirty)

	for _, f := range m.activeFilters {
		if f.Mode == models.FilterModeDirty && !f.Enabled {
			t.Error("FilterModeDirty should be enabled")
		}
		if f.Mode != models.FilterModeDirty && f.Enabled {
			t.Errorf("%v should be disabled", f.Mode)
		}
	}
}

func TestModelCycleFilterState(t *testing.T) {
	m := New(nil, 1)

	var aheadIdx int
	for i, f := range m.activeFilters {
		if f.Mode == models.FilterModeAhead {
			aheadIdx = i
			break
		}
	}

	if m.activeFilters[aheadIdx].Enabled {
		t.Error("should start disabled")
	}

	m.CycleFilterState(models.FilterModeAhead)
	if !m.activeFilters[aheadIdx].Enabled || m.activeFilters[aheadIdx].Inverted {
		t.Error("first cycle: should be enabled, not inverted")
	}

	m.CycleFilterState(models.FilterModeAhead)
	if !m.activeFilters[aheadIdx].Enabled || !m.activeFilters[aheadIdx].Inverted {
		t.Error("second cycle: should be enabled and inverted")
	}

	m.CycleFilterState(models.FilterModeAhead)
	if m.activeFilters[aheadIdx].Enabled || m.activeFilters[aheadIdx].Inverted {
		t.Error("third cycle: should be disabled and not inverted")
	}
}

func TestModelCycleFilterStateIgnoresAll(t *testing.T) {
	m := New(nil, 1)

	m.CycleFilterState(models.FilterModeAll)

	for _, f := range m.activeFilters {
		if f.Mode == models.FilterModeAll && !f.Enabled {
			t.Error("FilterModeAll should still be enabled")
		}
	}
}

func TestModelCycleFilter(t *testing.T) {
	m := New(nil, 1)
	modes := models.AllFilterModes()

	for i := 0; i < len(modes)+1; i++ {
		expectedIdx := (i + 1) % len(modes)
		m.CycleFilter()
		current := m.CurrentFilter()
		if current != modes[expectedIdx] {
			t.Errorf("cycle %d: expected %v, got %v", i, modes[expectedIdx], current)
		}
	}
}

func TestModelCycleSortState(t *testing.T) {
	m := New(nil, 1)

	var modifiedIdx int
	for i, s := range m.activeSorts {
		if s.Mode == models.SortModeModified {
			modifiedIdx = i
			break
		}
	}

	if m.activeSorts[modifiedIdx].Direction != models.SortDirectionOff {
		t.Error("should start off")
	}

	m.CycleSortState(models.SortModeModified)
	if m.activeSorts[modifiedIdx].Direction != models.SortDirectionAsc {
		t.Error("first cycle: should be Asc")
	}

	m.CycleSortState(models.SortModeModified)
	if m.activeSorts[modifiedIdx].Direction != models.SortDirectionDesc {
		t.Error("second cycle: should be Desc")
	}

	m.CycleSortState(models.SortModeModified)
	if m.activeSorts[modifiedIdx].Direction != models.SortDirectionOff {
		t.Error("third cycle: should be Off")
	}
}

func TestModelResetFilters(t *testing.T) {
	m := New(nil, 1)

	m.SetFilter(models.FilterModeDirty)
	m.CycleFilterState(models.FilterModeAhead)

	m.ResetFilters()

	for _, f := range m.activeFilters {
		if f.Mode == models.FilterModeAll {
			if !f.Enabled {
				t.Error("All should be enabled after reset")
			}
		} else {
			if f.Enabled || f.Inverted {
				t.Errorf("%v should be disabled and not inverted after reset", f.Mode)
			}
		}
	}
}

func TestModelResetSorts(t *testing.T) {
	m := New(nil, 1)

	m.CycleSortState(models.SortModeModified)
	m.CycleSortState(models.SortModeStatus)

	m.ResetSorts()

	for _, s := range m.activeSorts {
		if s.Mode == models.SortModeName {
			if s.Direction != models.SortDirectionAsc {
				t.Error("Name should be Asc after reset")
			}
		} else {
			if s.Direction != models.SortDirectionOff {
				t.Errorf("%v should be Off after reset", s.Mode)
			}
		}
	}
}

func TestModelDirtyCount(t *testing.T) {
	m := New(nil, 1)
	m.summaries = map[string]models.RepoSummary{
		"/repo1": {Staged: 1},
		"/repo2": {Ahead: 2},
		"/repo3": {},
	}

	if m.DirtyCount() != 2 {
		t.Errorf("expected 2 dirty, got %d", m.DirtyCount())
	}
}

func TestModelPRCount(t *testing.T) {
	m := New(nil, 1)
	m.summaries = map[string]models.RepoSummary{
		"/repo1": {PRInfo: &models.PRInfo{Number: 1}},
		"/repo2": {PRInfo: &models.PRInfo{Number: 2}},
		"/repo3": {},
	}

	if m.PRCount() != 2 {
		t.Errorf("expected 2 PRs, got %d", m.PRCount())
	}
}

func TestModelSelectedSummary(t *testing.T) {
	m := New(nil, 1)
	m.filteredPaths = []string{"/repo1", "/repo2"}
	m.summaries = map[string]models.RepoSummary{
		"/repo1": {Branch: "main"},
		"/repo2": {Branch: "develop"},
	}
	m.cursor = 1

	summary, ok := m.SelectedSummary()
	if !ok {
		t.Error("expected to find summary")
	}
	if summary.Branch != "develop" {
		t.Errorf("expected 'develop', got %q", summary.Branch)
	}
}

func TestModelSelectedSummaryOutOfBounds(t *testing.T) {
	m := New(nil, 1)
	m.cursor = 5

	_, ok := m.SelectedSummary()
	if ok {
		t.Error("should not find summary for out of bounds cursor")
	}
}

func TestModelActiveFilterModes(t *testing.T) {
	m := New(nil, 1)

	modes := m.ActiveFilterModes()
	if len(modes) != 0 {
		t.Error("initially should have no active non-All filters")
	}

	m.CycleFilterState(models.FilterModeAhead)
	m.CycleFilterState(models.FilterModeDirty)

	modes = m.ActiveFilterModes()
	if len(modes) != 2 {
		t.Errorf("expected 2 active modes, got %d", len(modes))
	}
}

func TestViewModeConstants(t *testing.T) {
	modes := []ViewMode{
		ViewModeRepoList,
		ViewModeRepoDetail,
		ViewModeBranchDetail,
		ViewModePRDetail,
		ViewModeHelp,
		ViewModeFilter,
		ViewModeSort,
		ViewModeBatchProgress,
	}

	for i, m := range modes {
		if int(m) != i {
			t.Errorf("expected ViewMode %d to have value %d", m, i)
		}
	}
}

func TestDetailTabConstants(t *testing.T) {
	tabs := []DetailTab{
		DetailTabBranches,
		DetailTabStashes,
		DetailTabWorktrees,
		DetailTabPRs,
	}

	for i, tab := range tabs {
		if int(tab) != i {
			t.Errorf("expected DetailTab %d to have value %d", tab, i)
		}
	}
}
