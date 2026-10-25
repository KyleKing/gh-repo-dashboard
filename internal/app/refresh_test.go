//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestRefreshFromRepoList(t *testing.T) {
	t.Parallel()
	m := New([]string{"/test"}, 1)
	m.viewMode = ViewModeRepoList
	m.summaries[testRepo1Path] = models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: testRepo1Path}}
	m.freshness[fetchKey{testRepo1Path, fetchCI}] = freshnessFresh

	msg := tea.KeyPressMsg{Code: 'r', Text: "r"}
	if !key.Matches(msg, m.keys.Refresh) {
		t.Error("'r' should match refresh key")
	}

	updatedModel, cmd := m.Update(msg)
	m = mustModel(t, updatedModel)

	if cmd == nil {
		t.Error("refresh should return a command")
	}

	if !m.loading {
		t.Error("refresh should set loading to true for repo list")
	}
	if len(m.summaries) != 0 {
		t.Error("refresh should clear summaries")
	}
	if len(m.freshness) != 0 {
		t.Error("refresh should forget which cells were read live")
	}
}

func TestRefreshFromRepoDetail(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = testRepoPath
	m.summaries[testRepoPath] = models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: testRepoPath, Upstream: "origin"}}
	m.branches = []vcs.BranchInfo{
		{Name: mainBranchName},
		{Name: featureBranchName},
	}

	msg := tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	if !key.Matches(msg, m.keys.Refresh) {
		t.Error("ctrl+r should match refresh key")
	}

	updatedModel, cmd := m.Update(msg)
	_ = mustModel(t, updatedModel)

	if cmd == nil {
		t.Error("refresh should return a command")
	}
}

func TestRefreshFromBranchDetail(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeBranchDetail
	m.selectedRepo = testRepoPath
	m.selectedBranch = vcs.BranchInfo{Name: featureBranchName}
	m.branchDetail = models.BranchDetail{
		Branch: vcs.BranchInfo{Name: featureBranchName},
	}

	msg := tea.KeyPressMsg{Code: 'r', Text: "r"}
	updatedModel, cmd := m.Update(msg)
	_ = mustModel(t, updatedModel)

	if cmd == nil {
		t.Error("refresh should return a command from branch detail view")
	}
}

func TestRefreshCompleteMessage(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoList

	msg := RefreshCompleteMsg{ViewMode: ViewModeRepoList}
	updatedModel, cmd := m.Update(msg)
	m = mustModel(t, updatedModel)

	if m.statusMessage != "Data refreshed" {
		t.Errorf("expected 'Data refreshed' status message, got %q", m.statusMessage)
	}

	if cmd == nil {
		t.Error("refresh complete should return clear status command")
	}
}

func TestRefreshKeybindings(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)

	rKey := tea.KeyPressMsg{Code: 'r', Text: "r"}
	if !key.Matches(rKey, m.keys.Refresh) {
		t.Error("'r' key should match Refresh binding")
	}

	ctrlR := tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	if !key.Matches(ctrlR, m.keys.Refresh) {
		t.Error("ctrl+r should match Refresh binding")
	}
}

func TestRefreshClearsCache(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoList
	m.summaries[testRepo1Path] = models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: testRepo1Path}}

	m, _ = m.handleRefresh()

	if len(m.summaries) != 0 {
		t.Error("handleRefresh should clear summaries map")
	}
	if len(m.freshness) != 0 {
		t.Error("handleRefresh should clear the freshness map")
	}
}

func TestRefreshFromEmptyState(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeBranchDetail

	_, cmd := m.handleRefresh()

	if cmd == nil {
		t.Error("refresh should always return a command")
	}
}

func TestRefreshPreservesViewMode(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		viewMode ViewMode
	}{
		{"repo list", ViewModeRepoList},
		{"repo detail", ViewModeRepoDetail},
		{"branch detail", ViewModeBranchDetail},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := New(nil, 1)
			m.viewMode = tc.viewMode
			m.selectedRepo = testRepoPath
			m.selectedBranch = vcs.BranchInfo{Name: mainBranchName}

			m, _ = m.handleRefresh()

			if m.viewMode != tc.viewMode {
				t.Errorf("refresh should preserve view mode, expected %v, got %v", tc.viewMode, m.viewMode)
			}
		})
	}
}

func TestRefreshClearsDownstreamFromRepoList(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoList
	m.branches = []vcs.BranchInfo{{Name: mainBranchName}}
	m.stashes = []vcs.StashDetail{{Index: 0}}
	m.worktrees = []vcs.WorktreeInfo{{Path: "/test"}}
	m.prs = []forge.PullRequest{{Number: 1}}
	m.branchDetail = models.BranchDetail{
		Branch: vcs.BranchInfo{Name: featureBranchName},
	}

	m, _ = m.handleRefresh()

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
}

func TestRefreshClearsDownstreamFromRepoDetail(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = testRepoPath
	m.summaries[testRepoPath] = models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: testRepoPath}}
	m.branches = []vcs.BranchInfo{{Name: mainBranchName}}
	m.prs = []forge.PullRequest{{Number: 1}}
	m.branchDetail = models.BranchDetail{
		Branch: vcs.BranchInfo{Name: featureBranchName},
	}

	m, _ = m.handleRefresh()

	if m.branches != nil {
		t.Error("refresh from repo detail should clear branches")
	}
	if m.prs != nil {
		t.Error("refresh from repo detail should clear PRs")
	}
	if m.branchDetail.Branch.Name != "" {
		t.Error("refresh from repo detail should clear branch detail")
	}
}

func TestRefreshClearsBranchDetail(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeBranchDetail
	m.selectedRepo = testRepoPath
	m.selectedBranch = vcs.BranchInfo{Name: featureBranchName}
	m.branchDetail = models.BranchDetail{
		Branch: vcs.BranchInfo{Name: featureBranchName},
		Commits: []vcs.CommitInfo{
			{Hash: "abc123"},
		},
	}

	m, _ = m.handleRefresh()

	if m.branchDetail.Branch.Name != "" {
		t.Error("refresh should clear branch detail")
	}
	if len(m.branchDetail.Commits) != 0 {
		t.Error("refresh should clear branch commits")
	}
}
