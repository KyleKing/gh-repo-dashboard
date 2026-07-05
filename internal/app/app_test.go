//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestNewModel(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	m := New(nil, 1)

	m.CycleFilterState(models.FilterModeAll)

	for _, f := range m.activeFilters {
		if f.Mode == models.FilterModeAll && !f.Enabled {
			t.Error("FilterModeAll should still be enabled")
		}
	}
}

func TestModelCycleFilter(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	modes := models.AllFilterModes()

	for i := range len(modes) + 1 {
		expectedIdx := (i + 1) % len(modes)
		m.CycleFilter()
		current := m.CurrentFilter()
		if current != modes[expectedIdx] {
			t.Errorf("cycle %d: expected %v, got %v", i, modes[expectedIdx], current)
		}
	}
}

func TestModelCycleSortState(t *testing.T) {
	t.Parallel()
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
	if m.activeSorts[modifiedIdx].Priority != 1 {
		t.Errorf("first cycle: expected contiguous priority 1 after Name at 0, got %d", m.activeSorts[modifiedIdx].Priority)
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

func TestModelCycleSortStateCompactsOnDisable(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)

	m.CycleSortState(models.SortModeModified)
	m.CycleSortState(models.SortModeStatus)

	m.CycleSortState(models.SortModeModified)
	m.CycleSortState(models.SortModeModified)
	m.CycleSortState(models.SortModeModified)

	for _, s := range m.activeSorts {
		switch s.Mode {
		case models.SortModeName:
			if s.Priority != 0 {
				t.Errorf("Name priority: expected 0, got %d", s.Priority)
			}
		case models.SortModeStatus:
			if s.Priority != 1 {
				t.Errorf("Status priority: expected 1 after Modified disabled, got %d", s.Priority)
			}
		default:
			// only enabled sorts have contiguity requirements
		}
	}
}

func TestMoveSortUpAfterEnable(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)

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

	m.CycleSortState(models.SortModeModified)
	m.sortCursor = modIdx
	m.MoveSortUp()

	if m.activeSorts[modIdx].Priority != 0 {
		t.Errorf("MoveSortUp after enable: expected Modified priority 0, got %d", m.activeSorts[modIdx].Priority)
	}
	if m.activeSorts[nameIdx].Priority != 1 {
		t.Errorf("MoveSortUp after enable: expected Name priority 1, got %d", m.activeSorts[nameIdx].Priority)
	}
}

func TestModelResetFilters(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	m := New(nil, 1)
	m.summaries = map[string]models.RepoSummary{
		testRepo1Path: {Staged: 1},
		"/repo2":      {Ahead: 2},
		"/repo3":      {},
	}

	if m.DirtyCount() != 2 {
		t.Errorf("expected 2 dirty, got %d", m.DirtyCount())
	}
}

func TestModelPRCount(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.summaries = map[string]models.RepoSummary{
		testRepo1Path: {PRInfo: &models.PRInfo{Number: 1}},
		"/repo2":      {PRInfo: &models.PRInfo{Number: 2}},
		"/repo3":      {},
	}

	if m.PRCount() != 2 {
		t.Errorf("expected 2 PRs, got %d", m.PRCount())
	}
}

func TestModelSelectedSummary(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.filteredPaths = []string{testRepo1Path, "/repo2"}
	m.summaries = map[string]models.RepoSummary{
		testRepo1Path: {Branch: mainBranchName},
		"/repo2":      {Branch: "develop"},
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
	t.Parallel()
	m := New(nil, 1)
	m.cursor = 5

	_, ok := m.SelectedSummary()
	if ok {
		t.Error("should not find summary for out of bounds cursor")
	}
}

func TestModelActiveFilterModes(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestBranchDetailDefaultComparison(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		detail   models.BranchDetail
		contains []string
		excludes []string
	}{
		{
			name: "shows ahead and behind vs default",
			detail: models.BranchDetail{
				Branch:        models.BranchInfo{Name: featureBranchName},
				DefaultBranch: mainBranchName,
				DefaultAhead:  2,
				DefaultBehind: 1,
			},
			contains: []string{"vs main:", "↑2 ahead", "↓1 behind"},
		},
		{
			name: "up to date vs default",
			detail: models.BranchDetail{
				Branch:        models.BranchInfo{Name: featureBranchName},
				DefaultBranch: mainBranchName,
			},
			contains: []string{"vs main:", "up to date"},
		},
		{
			name:     "hidden without comparison data",
			detail:   models.BranchDetail{Branch: models.BranchInfo{Name: featureBranchName}},
			excludes: []string{"vs main:"},
		},
		{
			name: "hidden on the default branch itself",
			detail: models.BranchDetail{
				Branch:        models.BranchInfo{Name: mainBranchName},
				DefaultBranch: mainBranchName,
			},
			excludes: []string{"vs main:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := New(nil, 1)
			m.viewMode = ViewModeBranchDetail
			m.branchDetail = tt.detail

			out := m.renderBranchDetail()
			for _, want := range tt.contains {
				if !strings.Contains(out, want) {
					t.Errorf("expected output to contain %q", want)
				}
			}
			for _, unwanted := range tt.excludes {
				if strings.Contains(out, unwanted) {
					t.Errorf("expected output to omit %q", unwanted)
				}
			}
		})
	}
}
