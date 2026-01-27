package app

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestRefreshFromRepoList(t *testing.T) {
	m := New([]string{"/test"}, 1)
	m.viewMode = ViewModeRepoList
	m.summaries["/repo1"] = models.RepoSummary{Path: "/repo1"}
	m.prCount["/repo1"] = 5

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	if !key.Matches(msg, m.keys.Refresh) {
		t.Error("'r' should match refresh key")
	}

	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if cmd == nil {
		t.Error("refresh should return a command")
	}

	if !m.loading {
		t.Error("refresh should set loading to true for repo list")
	}
	if len(m.summaries) != 0 {
		t.Error("refresh should clear summaries")
	}
	if len(m.prCount) != 0 {
		t.Error("refresh should clear PR count")
	}
}

func TestRefreshFromRepoDetail(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{
		Path:     "/test/repo",
		Upstream: "origin",
	}
	m.branches = []models.BranchInfo{
		{Name: "main"},
		{Name: "feature"},
	}

	msg := tea.KeyMsg{Type: tea.KeyCtrlR}
	if !key.Matches(msg, m.keys.Refresh) {
		t.Error("ctrl+r should match refresh key")
	}

	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if cmd == nil {
		t.Error("refresh should return a command")
	}
}

func TestRefreshFromBranchDetail(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeBranchDetail
	m.selectedRepo = "/test/repo"
	m.selectedBranch = models.BranchInfo{Name: "feature"}
	m.branchDetail = models.BranchDetail{
		Branch: models.BranchInfo{Name: "feature"},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if cmd == nil {
		t.Error("refresh should return a command from branch detail view")
	}
}

func TestRefreshFromPRDetail(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.selectedPR = models.PRInfo{Number: 123}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{Number: 123},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if cmd == nil {
		t.Error("refresh should return a command from PR detail view")
	}
}

func TestRefreshCompleteMessage(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoList

	msg := RefreshCompleteMsg{ViewMode: ViewModeRepoList}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.statusMessage != "Data refreshed" {
		t.Errorf("expected 'Data refreshed' status message, got %q", m.statusMessage)
	}

	if cmd == nil {
		t.Error("refresh complete should return clear status command")
	}
}

func TestRefreshKeybindings(t *testing.T) {
	m := New(nil, 1)

	rKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	if !key.Matches(rKey, m.keys.Refresh) {
		t.Error("'r' key should match Refresh binding")
	}

	ctrlR := tea.KeyMsg{Type: tea.KeyCtrlR}
	if !key.Matches(ctrlR, m.keys.Refresh) {
		t.Error("ctrl+r should match Refresh binding")
	}
}

func TestRefreshClearsCache(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoList
	m.summaries["/repo1"] = models.RepoSummary{Path: "/repo1"}

	updatedModel, _ := m.handleRefresh()
	m = updatedModel.(Model)

	if len(m.summaries) != 0 {
		t.Error("handleRefresh should clear summaries map")
	}
	if len(m.prCount) != 0 {
		t.Error("handleRefresh should clear prCount map")
	}
}

func TestRefreshFromEmptyState(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail

	updatedModel, cmd := m.handleRefresh()
	m = updatedModel.(Model)

	if cmd == nil {
		t.Error("refresh should always return a command")
	}
}

func TestRefreshPreservesViewMode(t *testing.T) {
	testCases := []struct {
		name     string
		viewMode ViewMode
	}{
		{"repo list", ViewModeRepoList},
		{"repo detail", ViewModeRepoDetail},
		{"branch detail", ViewModeBranchDetail},
		{"PR detail", ViewModePRDetail},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(nil, 1)
			m.viewMode = tc.viewMode
			m.selectedRepo = "/test/repo"
			m.selectedBranch = models.BranchInfo{Name: "main"}
			m.selectedPR = models.PRInfo{Number: 1}

			updatedModel, _ := m.handleRefresh()
			m = updatedModel.(Model)

			if m.viewMode != tc.viewMode {
				t.Errorf("refresh should preserve view mode, expected %v, got %v", tc.viewMode, m.viewMode)
			}
		})
	}
}

func TestRefreshClearsDownstreamFromRepoList(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoList
	m.branches = []models.BranchInfo{{Name: "main"}}
	m.stashes = []models.StashDetail{{Index: 0}}
	m.worktrees = []models.WorktreeInfo{{Path: "/test"}}
	m.prs = []models.PRInfo{{Number: 1}}
	m.branchDetail = models.BranchDetail{
		Branch: models.BranchInfo{Name: "feature"},
	}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{Number: 123},
	}

	updatedModel, _ := m.handleRefresh()
	m = updatedModel.(Model)

	if m.branches != nil {
		t.Error("refresh from repo list should clear branches")
	}
	if m.stashes != nil {
		t.Error("refresh from repo list should clear stashes")
	}
	if m.worktrees != nil {
		t.Error("refresh from repo list should clear worktrees")
	}
	if m.prs != nil {
		t.Error("refresh from repo list should clear PRs")
	}
	if m.branchDetail.Branch.Name != "" {
		t.Error("refresh from repo list should clear branch detail")
	}
	if m.prDetail.Number != 0 {
		t.Error("refresh from repo list should clear PR detail")
	}
}

func TestRefreshClearsDownstreamFromRepoDetail(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}
	m.branches = []models.BranchInfo{{Name: "main"}}
	m.prs = []models.PRInfo{{Number: 1}}
	m.branchDetail = models.BranchDetail{
		Branch: models.BranchInfo{Name: "feature"},
	}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{Number: 123},
	}

	updatedModel, _ := m.handleRefresh()
	m = updatedModel.(Model)

	if m.branches != nil {
		t.Error("refresh from repo detail should clear branches")
	}
	if m.prs != nil {
		t.Error("refresh from repo detail should clear PRs")
	}
	if m.branchDetail.Branch.Name != "" {
		t.Error("refresh from repo detail should clear branch detail")
	}
	if m.prDetail.Number != 0 {
		t.Error("refresh from repo detail should clear PR detail")
	}
}

func TestRefreshClearsBranchDetail(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeBranchDetail
	m.selectedRepo = "/test/repo"
	m.selectedBranch = models.BranchInfo{Name: "feature"}
	m.branchDetail = models.BranchDetail{
		Branch: models.BranchInfo{Name: "feature"},
		Commits: []models.CommitInfo{
			{Hash: "abc123"},
		},
	}

	updatedModel, _ := m.handleRefresh()
	m = updatedModel.(Model)

	if m.branchDetail.Branch.Name != "" {
		t.Error("refresh should clear branch detail")
	}
	if len(m.branchDetail.Commits) != 0 {
		t.Error("refresh should clear branch commits")
	}
}

func TestRefreshClearsPRDetail(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.selectedPR = models.PRInfo{Number: 123}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{
			Number: 123,
			Title:  "Test PR",
		},
		Author: "testuser",
	}

	updatedModel, _ := m.handleRefresh()
	m = updatedModel.(Model)

	if m.prDetail.Number != 0 {
		t.Error("refresh should clear PR detail number")
	}
	if m.prDetail.Title != "" {
		t.Error("refresh should clear PR detail title")
	}
	if m.prDetail.Author != "" {
		t.Error("refresh should clear PR detail author")
	}
}
