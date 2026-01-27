package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestPrefetchOnCursorMovement(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabPRs
	m.selectedRepo = "/test/repo"
	m.prs = []models.PRInfo{
		{Number: 1, Title: "PR 1"},
		{Number: 2, Title: "PR 2"},
		{Number: 3, Title: "PR 3"},
	}
	m.detailCursor = 0

	// Move down - should trigger prefetch
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.detailCursor != 1 {
		t.Errorf("cursor should move to 1, got %d", m.detailCursor)
	}

	if cmd == nil {
		t.Error("moving cursor should trigger prefetch command")
	}

	// Move up - should trigger prefetch
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, cmd = m.Update(msg)
	m = updatedModel.(Model)

	if m.detailCursor != 0 {
		t.Errorf("cursor should move to 0, got %d", m.detailCursor)
	}

	if cmd == nil {
		t.Error("moving cursor up should trigger prefetch command")
	}
}

func TestPrefetchOnTabSwitch(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabBranches
	m.selectedRepo = "/test/repo"
	m.prs = []models.PRInfo{
		{Number: 10, Title: "First PR"},
		{Number: 20, Title: "Second PR"},
	}

	// Switch to PR tab
	msg := tea.KeyMsg{Type: tea.KeyTab}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.detailTab != DetailTabStashes {
		t.Error("first tab should move to stashes")
	}

	// Tab again to worktrees
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(Model)

	// Tab to PRs
	updatedModel, cmd = m.Update(msg)
	m = updatedModel.(Model)

	if m.detailTab != DetailTabPRs {
		t.Error("should be on PR tab")
	}

	if cmd == nil {
		t.Error("switching to PR tab should trigger prefetch for first PR")
	}
}

func TestPrefetchOnDetailLoad(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/test/repo"

	prs := []models.PRInfo{
		{Number: 100, Title: "PR 100"},
		{Number: 200, Title: "PR 200"},
		{Number: 300, Title: "PR 300"},
		{Number: 400, Title: "PR 400"},
	}

	msg := DetailLoadedMsg{
		Path:     "/test/repo",
		Branches: []models.BranchInfo{},
		PRs:      prs,
	}

	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if len(m.prs) != 4 {
		t.Errorf("expected 4 PRs, got %d", len(m.prs))
	}

	if cmd == nil {
		t.Error("loading PR list should trigger prefetch commands for first 3 PRs")
	}
}

func TestNavigateBetweenPRsInDetailView(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}
	m.prs = []models.PRInfo{
		{Number: 1, Title: "First PR", State: "OPEN"},
		{Number: 2, Title: "Second PR", State: "OPEN"},
		{Number: 3, Title: "Third PR", State: "OPEN"},
	}
	m.selectedPR = m.prs[0]
	m.prDetail = models.PRDetail{
		PRInfo: m.prs[0],
		Author: "user1",
	}

	// Press down to go to next PR
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.selectedPR.Number != 2 {
		t.Errorf("should switch to PR #2, got #%d", m.selectedPR.Number)
	}

	if m.prDetail.Number != 2 {
		t.Error("prDetail should be updated with new PR basic info")
	}

	if cmd == nil {
		t.Error("navigating to next PR should return commands (load + prefetch)")
	}

	// Press up to go back
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, cmd = m.Update(msg)
	m = updatedModel.(Model)

	if m.selectedPR.Number != 1 {
		t.Errorf("should switch back to PR #1, got #%d", m.selectedPR.Number)
	}

	if cmd == nil {
		t.Error("navigating to previous PR should return commands")
	}
}

func TestNavigatePRDetailAtBoundaries(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}
	m.prs = []models.PRInfo{
		{Number: 1, Title: "Only PR", State: "OPEN"},
	}
	m.selectedPR = m.prs[0]
	m.prDetail = models.PRDetail{PRInfo: m.prs[0]}

	// Try to go down (should do nothing)
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.selectedPR.Number != 1 {
		t.Error("should stay on PR #1")
	}

	if cmd != nil {
		t.Error("navigating past end should return nil command")
	}

	// Try to go up (should do nothing)
	msg = tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, cmd = m.Update(msg)
	m = updatedModel.(Model)

	if m.selectedPR.Number != 1 {
		t.Error("should stay on PR #1")
	}

	if cmd != nil {
		t.Error("navigating past beginning should return nil command")
	}
}

func TestPrefetchNotTriggeredOnNonPRTabs(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabBranches
	m.selectedRepo = "/test/repo"
	m.branches = []models.BranchInfo{
		{Name: "main"},
		{Name: "feature"},
	}
	m.detailCursor = 0

	// Move down on branch tab
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.detailCursor != 1 {
		t.Error("cursor should move")
	}

	if cmd != nil {
		t.Error("moving cursor on non-PR tab should not trigger prefetch")
	}
}

func TestPrefetchCacheHit(t *testing.T) {
	// This is more of an integration test concept
	// The actual caching happens in github.GetPRDetail
	// We're testing that prefetchPRDetailCmd doesn't send a message
	cmd := prefetchPRDetailCmd("/test/repo", 123)

	if cmd == nil {
		t.Fatal("prefetch command should be created")
	}

	msg := cmd()

	if msg != nil {
		t.Error("prefetch command should return nil message (silent background load)")
	}
}
