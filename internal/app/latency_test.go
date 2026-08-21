//nolint:testpackage // Model internals are built directly by design; see ROADMAP.md
package app

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// scrollLatencyBudget bounds how long one scroll step may take to update and
// render. Discovery and every per-repo read are git/gh subprocesses and
// network calls, whose speed depends on the machine, the disk, and the
// network, none of which this test can hold constant. What it can hold
// constant is the cost of Update and View themselves: once the data a scroll
// step touches is already in memory, walking and rendering it is pure CPU
// work with no I/O in the timed section, so a fixed budget is meaningful
// regardless of what machine runs the test. The budget is generous relative
// to that in-memory cost (measured under 15ms at this size) specifically so
// the test catches a real regression (an accidental O(n^2) walk, a blocking
// call added to Update or View) without becoming flaky on a loaded CI runner.
const scrollLatencyBudget = 75 * time.Millisecond

// syntheticFleetSize and syntheticBranchSize are generous upper bounds on a
// real fleet and a real repo's local branch count respectively (this app has
// caught an actual quadratic regression well past these sizes, but neither
// dimension realistically reaches even this size on its own).
const (
	syntheticFleetSize  = 300
	syntheticBranchSize = 300
)

// syntheticFleetModel builds a fully-loaded fleet with no pending fetches, so
// timing a scroll step measures only Update and View, not the fetch
// bookkeeping a still-loading model would also walk.
func syntheticFleetModel(t *testing.T) Model {
	t.Helper()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = 200, 50
	m.loading = false

	m.repoPaths = make([]string, syntheticFleetSize)
	m.summaries = make(map[string]models.RepoSummary, syntheticFleetSize)

	for i := range syntheticFleetSize {
		path := fmt.Sprintf("/dev/repo-%04d", i)
		m.repoPaths[i] = path
		m.summaries[path] = models.RepoSummary{
			RepoSummary: vcs.RepoSummary{
				Path:         path,
				VCSType:      vcs.TypeGit,
				Branch:       mainBranchName,
				Upstream:     "origin/main",
				Ahead:        i % 3,
				LastModified: time.Now().Add(-time.Duration(i) * time.Minute),
			},
		}
		m.prCount[path] = i % 5
		m.branchCount[path] = i%7 + 1
	}

	m.updateFilteredPaths()

	return m
}

func scrollStep(t *testing.T, m Model) time.Duration {
	t.Helper()

	start := time.Now()
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = mustModel(t, updated).renderScreen()

	return time.Since(start)
}

func TestScrollLatency_RepoListStaysUnderBudget(t *testing.T) {
	t.Parallel()

	m := syntheticFleetModel(t)

	if elapsed := scrollStep(t, m); elapsed > scrollLatencyBudget {
		t.Errorf("one scroll step and render in the Repos list took %s across %d repos, want under %s",
			elapsed, syntheticFleetSize, scrollLatencyBudget)
	}
}

func TestScrollLatency_SingleRepoViewStaysUnderBudget(t *testing.T) {
	t.Parallel()

	m := syntheticFleetModel(t)
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = m.repoPaths[0]
	m.focusedPanel = panelBranches

	m.branches = make([]vcs.BranchInfo, syntheticBranchSize)
	for i := range m.branches {
		m.branches[i] = vcs.BranchInfo{Name: fmt.Sprintf("feature/branch-%04d", i)}
	}

	if elapsed := scrollStep(t, m); elapsed > scrollLatencyBudget {
		t.Errorf("one scroll step and render in the single-repo view took %s across %d branches, want under %s",
			elapsed, syntheticBranchSize, scrollLatencyBudget)
	}
}
