//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestCICellStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		workflow  *models.WorkflowSummary
		requested bool
		settled   bool
		want      string
	}{
		{name: "not yet requested", want: emDash},
		{name: "request in flight", requested: true, want: pendingGlyph},
		{name: "failed fetch stops spinning", requested: true, settled: true, want: emDash},
		{name: "no runs for the commit", workflow: &models.WorkflowSummary{}, requested: true, want: emDash},
		{
			name:     "all passing",
			workflow: &models.WorkflowSummary{Total: 3, Passing: 3}, requested: true, want: "✓",
		},
		{
			name:     "one failure names the count",
			workflow: &models.WorkflowSummary{Total: 2, Passing: 1, Failing: 1}, requested: true, want: "✗ 1/2",
		},
		{
			name:     "still running",
			workflow: &models.WorkflowSummary{Total: 2, Passing: 1, InProgress: 1}, requested: true, want: "…1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := New(nil, 1)
			m.ciRequested = map[string]bool{"/dev/app": tt.requested}
			if tt.requested && !tt.settled {
				m.startFetch("/dev/app", fetchCI)
			}
			summary := models.RepoSummary{Path: "/dev/app", WorkflowInfo: tt.workflow}
			m.summaries["/dev/app"] = summary

			if got, _ := m.ciCell(summary, plainStyle, false); got != tt.want {
				t.Errorf("ciCell = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFailedCIFetchSettlesTheCell(t *testing.T) {
	t.Parallel()

	m := New([]string{"/dev"}, 1)
	m.summaries = map[string]models.RepoSummary{"/dev/app": {Path: "/dev/app", Branch: mainBranchName}}
	m.ciRequested = map[string]bool{"/dev/app": true}
	m.startFetch("/dev/app", fetchCI)

	updated, _ := m.Update(WorkflowLoadedMsg{Path: "/dev/app", Error: errGHFailed})
	m = mustModel(t, updated)

	if got, _ := m.ciCell(m.summaries["/dev/app"], plainStyle, false); got != emDash {
		t.Errorf("ciCell after a failed fetch = %q, want %q; the placeholder must not spin forever", got, emDash)
	}
}

func TestCIFetchesOnlyVisibleRepos(t *testing.T) {
	t.Parallel()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = 120, 20
	m.loading = false
	m.summaries = map[string]models.RepoSummary{}
	m.repoPaths = nil

	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"} {
		path := "/dev/" + name
		m.summaries[path] = models.RepoSummary{Path: path, Branch: "main"}
		m.repoPaths = append(m.repoPaths, path)
	}
	m.updateFilteredPaths()

	first := m.visibleCICmds()
	if len(first) == 0 || len(first) >= len(m.repoPaths) {
		t.Fatalf("requested CI for %d of %d repos; a screenful should be fewer than all of them",
			len(first), len(m.repoPaths))
	}

	if again := m.visibleCICmds(); len(again) != 0 {
		t.Errorf("re-requested CI for %d repos already asked for", len(again))
	}

	m.cursor = len(m.repoPaths) - 1
	if scrolled := m.visibleCICmds(); len(scrolled) == 0 {
		t.Error("scrolling to unseen rows should request CI for them")
	}
}

func TestOverviewNamesTheBranchItsCIRanOn(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.ciRequested = map[string]bool{"/dev/app": true}
	m.ciBranch = map[string]string{"/dev/app": "trunk"}
	summary := models.RepoSummary{
		Path: "/dev/app", WorkflowInfo: &models.WorkflowSummary{Total: 1, Passing: 1},
	}

	if got := m.overviewCI(summary); got != "✓ on trunk" {
		t.Errorf("overview CI = %q, want %q", got, "✓ on trunk")
	}
}

func TestFocusedHeaderShowsCIStatus(t *testing.T) {
	t.Parallel()

	m := focusedModel(140, 35)
	summary := m.summaries["/dev/alpha"]
	summary.WorkflowInfo = &models.WorkflowSummary{Total: 2, Passing: 1, Failing: 1}
	m.summaries["/dev/alpha"] = summary

	if header := plainText(m.renderRepoDetailBreadcrumbs()); !strings.Contains(header, "CI ✗ 1/2") {
		t.Errorf("focused header does not carry the CI badge: %q", header)
	}
}
