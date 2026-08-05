//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// focusedModel returns a model sitting in the focused repo view with data on
// every tab, so a scene switch has something to show.
func focusedModel(width, height int) Model {
	now := time.Now()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = width, height
	m.loading = false
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/dev/alpha"
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {
			Path: "/dev/alpha", VCSType: models.VCSTypeGit, Branch: "main",
			Upstream: "origin/main", Ahead: 1, StashCount: 2, LastModified: now,
			NotesFiles: []models.NoteFile{{Name: ".doing", FirstLine: "wip"}},
		},
	}
	m.repoPaths = []string{"/dev/alpha"}
	m.branches = []models.BranchInfo{{Name: "main", Upstream: "origin/main", Ahead: 1, IsCurrent: true}}
	m.stashes = []models.StashDetail{{Index: 0, Message: "On main: spike", Date: now}}
	m.worktrees = []models.WorktreeInfo{{Path: "/dev/alpha", Branch: "main"}}
	m.prs = []models.PRInfo{{Number: 9, Title: "Add a thing", State: "OPEN", HeadRef: "feature/thing"}}
	m.updateFilteredPaths()

	return m
}

func TestSceneKeysSelectTheirTab(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		name string
		want DetailTab
	}{
		{key: "1", name: "work", want: DetailTabBranches},
		{key: "2", name: "review", want: DetailTabPRs},
		{key: "3", name: "sync", want: DetailTabWorktrees},
		{key: "4", name: "maintain", want: DetailTabStashes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := focusedModel(120, 35)
			m.detailTab = DetailTabNotes

			next, _ := m.handleDetailKey(tea.KeyPressMsg{Code: rune(tt.key[0]), Text: tt.key})
			got, ok := next.(Model)
			if !ok {
				t.Fatalf("handleDetailKey returned %T, want Model", next)
			}

			if got.detailTab != tt.want {
				t.Errorf("scene %s selected tab %v, want %v", tt.key, got.detailTab, tt.want)
			}

			if !strings.Contains(plainText(renderSceneBar(got.detailTab)), "["+tt.key+" "+tt.name+"]") {
				t.Errorf("scene bar does not mark %s as active: %q",
					tt.name, plainText(renderSceneBar(got.detailTab)))
			}
		})
	}
}

func TestSceneBarNamesEveryScene(t *testing.T) {
	t.Parallel()

	bar := plainText(renderSceneBar(DetailTabBranches))
	for _, s := range scenes() {
		if !strings.Contains(bar, s.key+" "+s.name) {
			t.Errorf("scene bar omits %s %s: %q", s.key, s.name, bar)
		}
	}

	// The Notes tab belongs to no scene, so nothing is marked active.
	if strings.Contains(plainText(renderSceneBar(DetailTabNotes)), "[") {
		t.Errorf("the Notes tab must not claim a scene: %q", plainText(renderSceneBar(DetailTabNotes)))
	}
}

func TestOverviewPaneAnswersOnArrival(t *testing.T) {
	t.Parallel()

	view := plainText(focusedModel(120, 35).renderRepoDetail())

	want := []string{"Sync", "↑1 vs origin/main", "Files", "clean", "Peers", "Stashes", "2", "Notes", ".doing"}
	for _, want := range want {
		if !strings.Contains(view, want) {
			t.Errorf("focused view does not answer %q without a keypress:\n%s", want, view)
		}
	}
}

func TestCompactOverviewKeepsOnlySyncAndFiles(t *testing.T) {
	t.Parallel()

	m := focusedModel(80, 24)
	rows := m.overviewRows(m.summaries["/dev/alpha"], true)

	if len(rows) != 2 || rows[0].label != "Sync" || rows[1].label != "Files" {
		t.Fatalf("compact overview has %d rows, want Sync and Files only: %+v", len(rows), rows)
	}

	view := plainText(m.renderRepoDetail())
	if strings.Contains(view, "Template") {
		t.Errorf("the compact overview should drop the Template row:\n%s", view)
	}
}

func TestFocusedViewFitsEightyByTwentyFour(t *testing.T) {
	t.Parallel()

	for _, tab := range []DetailTab{DetailTabBranches, DetailTabStashes, DetailTabWorktrees, DetailTabPRs} {
		m := focusedModel(80, 24)
		m.detailTab = tab
		m.prs = nil // an empty tab is the tallest case: it renders a placeholder box

		lines := strings.Split(m.renderScreen(), "\n")
		if len(lines) > 24 {
			t.Errorf("tab %v renders %d lines at 80x24", tab, len(lines))
		}

		for i, line := range lines {
			if got := lipgloss.Width(line); got > 80 {
				t.Errorf("tab %v line %d is %d cells wide: %q", tab, i, got, plainText(line))
			}
		}
	}
}

func TestFocusedHeaderShowsDetachedHEAD(t *testing.T) {
	t.Parallel()

	m := focusedModel(140, 35)
	summary := m.summaries["/dev/alpha"]
	summary.Branch = models.DetachedBranchLabel("85d16a3")
	m.summaries["/dev/alpha"] = summary

	header := plainText(m.renderRepoDetailBreadcrumbs())
	for _, want := range []string{"detached", "85d16a3"} {
		if !strings.Contains(header, want) {
			t.Errorf("focused header %q is missing %q; a detached HEAD must not be silent", header, want)
		}
	}
}

func TestReviewSceneCarriesTheChecksColumn(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 40)
	m.detailTab = DetailTabPRs
	m.prs = []models.PRInfo{{
		Number: 9, Title: "Add a thing", State: "OPEN", HeadRef: "feature/thing",
		Checks: models.ChecksStatus{Total: 3, Passing: 2, Failing: 1},
	}}

	rendered := plainText(m.renderPRList())
	for _, want := range []string{"CHECKS", "failing 2/3"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("PR table is missing %q:\n%s", want, rendered)
		}
	}
}

func TestHelpNamesTheSceneKeys(t *testing.T) {
	t.Parallel()

	help := plainText(focusedModel(140, 40).renderHelp())
	if !strings.Contains(help, sceneKeyRange()) {
		t.Errorf("help overlay does not name the scene keys %q", sceneKeyRange())
	}
	for _, s := range scenes() {
		if !strings.Contains(help, s.name) {
			t.Errorf("help overlay is missing scene %q; the layer must be discoverable", s.name)
		}
	}
}
