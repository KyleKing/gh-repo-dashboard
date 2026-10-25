//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestPrefetchOnCursorMovement(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelBranches
	m.selectedRepo = testRepoPath
	m.branches = []vcs.BranchInfo{{Name: "one"}, {Name: "two"}, {Name: "three"}}
	m.detailCursor = 0

	// Move down - should trigger prefetch
	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	updatedModel, cmd := m.Update(msg)
	m = mustModel(t, updatedModel)

	if m.detailCursor != 1 {
		t.Errorf("cursor should move to 1, got %d", m.detailCursor)
	}

	if cmd == nil {
		t.Error("moving cursor should trigger prefetch command")
	}

	// Move up - should trigger prefetch
	msg = tea.KeyPressMsg{Code: tea.KeyUp}
	updatedModel, cmd = m.Update(msg)
	m = mustModel(t, updatedModel)

	if m.detailCursor != 0 {
		t.Errorf("cursor should move to 0, got %d", m.detailCursor)
	}

	if cmd == nil {
		t.Error("moving cursor up should trigger prefetch command")
	}
}

func TestPanelsWithNoDetailFetchIssueNoCommand(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelPeers
	m.selectedRepo = testRepoPath
	m.summaries = map[string]models.RepoSummary{
		testRepoPath: {RepoSummary: vcs.RepoSummary{Path: testRepoPath, RemoteRepo: "acme/app"}},
		"/other":     {RepoSummary: vcs.RepoSummary{Path: "/other", RemoteRepo: "acme/app", Branch: "feature"}},
		"/third":     {RepoSummary: vcs.RepoSummary{Path: "/third", RemoteRepo: "acme/app", Branch: "spike"}},
	}

	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = mustModel(t, updatedModel)

	if m.detailCursor != 1 {
		t.Fatalf("cursor at %d, want it to move", m.detailCursor)
	}
	if cmd != nil {
		t.Error("a peer checkout renders from cached data, so moving onto one fetches nothing")
	}
}
