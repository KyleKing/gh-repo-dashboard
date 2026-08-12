//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestPrefetchOnCursorMovement(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelBranches
	m.selectedRepo = testRepoPath
	m.branches = []models.BranchInfo{{Name: "one"}, {Name: "two"}, {Name: "three"}}
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

func TestPrefetchOnDetailLoad(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.selectedRepo = testRepoPath

	prs := []models.PRInfo{
		{Number: 100, Title: "PR 100"},
		{Number: 200, Title: "PR 200"},
		{Number: 300, Title: "PR 300"},
		{Number: 400, Title: "PR 400"},
	}

	msg := DetailLoadedMsg{
		Path:     testRepoPath,
		Branches: []models.BranchInfo{},
		PRs:      prs,
	}

	updatedModel, cmd := m.Update(msg)
	m = mustModel(t, updatedModel)

	if len(m.prs) != 4 {
		t.Errorf("expected 4 PRs, got %d", len(m.prs))
	}

	if cmd == nil {
		t.Error("loading PR list should trigger prefetch commands for first 3 PRs")
	}
}

func TestNavigateBetweenPRsInDetailView(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail
	m.selectedRepo = testRepoPath
	m.summaries[testRepoPath] = models.RepoSummary{Path: testRepoPath}
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

	// Press ] to go to the next PR
	updatedModel, cmd := m.Update(keyPress(']'))
	m = mustModel(t, updatedModel)

	if m.selectedPR.Number != 2 {
		t.Errorf("should switch to PR #2, got #%d", m.selectedPR.Number)
	}

	if m.prDetail.Number != 2 {
		t.Error("prDetail should be updated with new PR basic info")
	}

	if cmd == nil {
		t.Error("navigating to next PR should return commands (load + prefetch)")
	}

	// Press [ to go back
	updatedModel, cmd = m.Update(keyPress('['))
	m = mustModel(t, updatedModel)

	if m.selectedPR.Number != 1 {
		t.Errorf("should switch back to PR #1, got #%d", m.selectedPR.Number)
	}

	if cmd == nil {
		t.Error("navigating to previous PR should return commands")
	}
}

func TestNavigatePRDetailAtBoundaries(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail
	m.selectedRepo = testRepoPath
	m.summaries[testRepoPath] = models.RepoSummary{Path: testRepoPath}
	m.prs = []models.PRInfo{
		{Number: 1, Title: "Only PR", State: "OPEN"},
	}
	m.selectedPR = m.prs[0]
	m.prDetail = models.PRDetail{PRInfo: m.prs[0]}

	// Try to go to the next PR (should do nothing, there is only one)
	updatedModel, cmd := m.Update(keyPress(']'))
	m = mustModel(t, updatedModel)

	if m.selectedPR.Number != 1 {
		t.Error("should stay on PR #1")
	}

	if cmd != nil {
		t.Error("navigating past end should return nil command")
	}

	// Try to go to the previous PR (should do nothing, already at the start)
	updatedModel, cmd = m.Update(keyPress('['))
	m = mustModel(t, updatedModel)

	if m.selectedPR.Number != 1 {
		t.Error("should stay on PR #1")
	}

	if cmd != nil {
		t.Error("navigating past beginning should return nil command")
	}
}

// TestScrollPRDetailClampsAtBothEnds pins that j/k scroll the page's own
// content, and never move the offset past what the page actually holds.
func TestScrollPRDetailClampsAtBothEnds(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.width, m.height = 100, 10
	m.viewMode = ViewModePRDetail
	m.selectedRepo = testRepoPath
	m.summaries[testRepoPath] = models.RepoSummary{Path: testRepoPath}
	m.selectedPR = models.PRInfo{Number: 1, Title: "Only PR", State: "OPEN"}
	m.prDetail = models.PRDetail{
		PRInfo: m.selectedPR,
		Author: "user1",
		Body:   strings.Repeat("a long description line\n", 50),
	}

	updatedModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	up := mustModel(t, updatedModel)
	if up.prDetailScroll != 0 {
		t.Errorf("scrolling up from the top should stay at 0, got %d", up.prDetailScroll)
	}

	maxScroll := m.maxPRDetailScroll()
	if maxScroll <= 0 {
		t.Fatal("expected the long description to overflow the page")
	}

	m.prDetailScroll = maxScroll
	updatedModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	atBottom := mustModel(t, updatedModel)
	if atBottom.prDetailScroll != maxScroll {
		t.Errorf("scrolling down past the bottom should clamp at %d, got %d", maxScroll, atBottom.prDetailScroll)
	}
}

func TestPanelsWithNoDetailFetchIssueNoCommand(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelPeers
	m.selectedRepo = testRepoPath
	m.summaries = map[string]models.RepoSummary{
		testRepoPath: {Path: testRepoPath, RemoteRepo: "acme/app"},
		"/other":     {Path: "/other", RemoteRepo: "acme/app", Branch: "feature"},
		"/third":     {Path: "/third", RemoteRepo: "acme/app", Branch: "spike"},
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

func TestPrefetchCacheHit(t *testing.T) {
	t.Parallel()
	// This is more of an integration test concept
	// The actual caching happens in github.GetPRDetail
	// We're testing that prefetchPRDetailCmd doesn't send a message
	cmd := prefetchPRDetailCmd(testRepoPath, "github.com/acme/app", 123)

	if cmd == nil {
		t.Fatal("prefetch command should be created")
	}

	msg := cmd()

	if msg != nil {
		t.Error("prefetch command should return nil message (silent background load)")
	}
}
