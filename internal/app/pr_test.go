package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestPRTabNavigation(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabBranches

	m.detailTab = DetailTab((int(m.detailTab) + 1) % 4)
	if m.detailTab != DetailTabStashes {
		t.Errorf("expected DetailTabStashes, got %v", m.detailTab)
	}

	m.detailTab = DetailTab((int(m.detailTab) + 1) % 4)
	if m.detailTab != DetailTabWorktrees {
		t.Errorf("expected DetailTabWorktrees, got %v", m.detailTab)
	}

	m.detailTab = DetailTab((int(m.detailTab) + 1) % 4)
	if m.detailTab != DetailTabPRs {
		t.Errorf("expected DetailTabPRs, got %v", m.detailTab)
	}

	m.detailTab = DetailTab((int(m.detailTab) + 1) % 4)
	if m.detailTab != DetailTabBranches {
		t.Errorf("expected DetailTabBranches (wrapped), got %v", m.detailTab)
	}
}

func TestPRTabBackwardNavigation(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabBranches

	newTab := int(m.detailTab) - 1
	if newTab < 0 {
		newTab = 3
	}
	m.detailTab = DetailTab(newTab)

	if m.detailTab != DetailTabPRs {
		t.Errorf("expected DetailTabPRs (wrapped), got %v", m.detailTab)
	}
}

func TestDetailListLenWithPRs(t *testing.T) {
	m := New(nil, 1)
	m.branches = make([]models.BranchInfo, 5)
	m.stashes = make([]models.StashDetail, 3)
	m.worktrees = make([]models.WorktreeInfo, 2)
	m.prs = []models.PRInfo{
		{Number: 1, Title: "PR 1"},
		{Number: 2, Title: "PR 2"},
		{Number: 3, Title: "PR 3"},
	}

	m.detailTab = DetailTabBranches
	if m.detailListLen() != 5 {
		t.Errorf("expected 5 branches, got %d", m.detailListLen())
	}

	m.detailTab = DetailTabStashes
	if m.detailListLen() != 3 {
		t.Errorf("expected 3 stashes, got %d", m.detailListLen())
	}

	m.detailTab = DetailTabWorktrees
	if m.detailListLen() != 2 {
		t.Errorf("expected 2 worktrees, got %d", m.detailListLen())
	}

	m.detailTab = DetailTabPRs
	if m.detailListLen() != 3 {
		t.Errorf("expected 3 PRs, got %d", m.detailListLen())
	}
}

func TestPRCountInModel(t *testing.T) {
	m := New(nil, 1)
	if m.prCount == nil {
		t.Error("prCount should be initialized")
	}

	m.prCount["/repo1"] = 5
	m.prCount["/repo2"] = 3

	if m.prCount["/repo1"] != 5 {
		t.Errorf("expected 5 PRs for repo1, got %d", m.prCount["/repo1"])
	}

	if m.prCount["/repo2"] != 3 {
		t.Errorf("expected 3 PRs for repo2, got %d", m.prCount["/repo2"])
	}

	if m.prCount["/repo3"] != 0 {
		t.Errorf("expected 0 PRs for repo3, got %d", m.prCount["/repo3"])
	}
}

func TestPRListSelection(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabPRs
	m.prs = []models.PRInfo{
		{Number: 123, Title: "First PR", HeadRef: "feature-1"},
		{Number: 456, Title: "Second PR", HeadRef: "feature-2"},
		{Number: 789, Title: "Third PR", HeadRef: "feature-3"},
	}
	m.detailCursor = 1

	if m.detailCursor >= len(m.prs) {
		t.Error("cursor should be within bounds")
	}

	selectedPR := m.prs[m.detailCursor]
	if selectedPR.Number != 456 {
		t.Errorf("expected PR #456, got #%d", selectedPR.Number)
	}
	if selectedPR.Title != "Second PR" {
		t.Errorf("expected 'Second PR', got %q", selectedPR.Title)
	}
}

func TestPRDetailViewMode(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail

	if m.viewMode != ViewModePRDetail {
		t.Errorf("expected ViewModePRDetail, got %v", m.viewMode)
	}

	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{
			Number:  123,
			Title:   "Test PR",
			State:   "OPEN",
			HeadRef: "feature-branch",
			BaseRef: "main",
		},
		Author:    "testuser",
		Assignees: []string{"reviewer1", "reviewer2"},
		Reviewers: []string{"reviewer3"},
	}

	if m.prDetail.Number != 123 {
		t.Errorf("expected PR #123, got #%d", m.prDetail.Number)
	}
	if len(m.prDetail.Assignees) != 2 {
		t.Errorf("expected 2 assignees, got %d", len(m.prDetail.Assignees))
	}
	if len(m.prDetail.Reviewers) != 1 {
		t.Errorf("expected 1 reviewer, got %d", len(m.prDetail.Reviewers))
	}
}

func TestPRInfoStatusDisplay(t *testing.T) {
	tests := []struct {
		name     string
		pr       models.PRInfo
		expected string
	}{
		{
			name:     "draft PR",
			pr:       models.PRInfo{IsDraft: true, State: "OPEN"},
			expected: "DRAFT",
		},
		{
			name:     "open PR",
			pr:       models.PRInfo{IsDraft: false, State: "OPEN"},
			expected: "OPEN",
		},
		{
			name:     "merged PR",
			pr:       models.PRInfo{IsDraft: false, State: "MERGED"},
			expected: "MERGED",
		},
		{
			name:     "closed PR",
			pr:       models.PRInfo{IsDraft: false, State: "CLOSED"},
			expected: "CLOSED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pr.StatusDisplay()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestPRInfoReviewStatus(t *testing.T) {
	tests := []struct {
		name     string
		pr       models.PRInfo
		expected string
	}{
		{
			name:     "approved",
			pr:       models.PRInfo{ReviewDecision: "APPROVED"},
			expected: "approved",
		},
		{
			name:     "changes requested",
			pr:       models.PRInfo{ReviewDecision: "CHANGES_REQUESTED"},
			expected: "changes requested",
		},
		{
			name:     "review required",
			pr:       models.PRInfo{ReviewDecision: "REVIEW_REQUIRED"},
			expected: "review required",
		},
		{
			name:     "approved by reviewers",
			pr:       models.PRInfo{ApprovedBy: []string{"user1", "user2"}},
			expected: "approved",
		},
		{
			name:     "no review",
			pr:       models.PRInfo{},
			expected: "—",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pr.ReviewStatus()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestPRDetailMetadata(t *testing.T) {
	pr := models.PRDetail{
		PRInfo: models.PRInfo{
			Number:  100,
			Title:   "Add new feature",
			HeadRef: "feature/new-thing",
			BaseRef: "main",
		},
		Author:    "alice",
		Assignees: []string{"bob", "charlie"},
		Reviewers: []string{"dave", "eve"},
		Additions: 150,
		Deletions: 50,
		Comments:  12,
	}

	if pr.Author != "alice" {
		t.Errorf("expected author 'alice', got %q", pr.Author)
	}

	if len(pr.Assignees) != 2 {
		t.Errorf("expected 2 assignees, got %d", len(pr.Assignees))
	}
	if pr.Assignees[0] != "bob" || pr.Assignees[1] != "charlie" {
		t.Errorf("unexpected assignees: %v", pr.Assignees)
	}

	if len(pr.Reviewers) != 2 {
		t.Errorf("expected 2 reviewers, got %d", len(pr.Reviewers))
	}
	if pr.Reviewers[0] != "dave" || pr.Reviewers[1] != "eve" {
		t.Errorf("unexpected reviewers: %v", pr.Reviewers)
	}

	if pr.Additions != 150 {
		t.Errorf("expected 150 additions, got %d", pr.Additions)
	}
	if pr.Deletions != 50 {
		t.Errorf("expected 50 deletions, got %d", pr.Deletions)
	}
	if pr.Comments != 12 {
		t.Errorf("expected 12 comments, got %d", pr.Comments)
	}
}

func TestRenderPRListEmpty(t *testing.T) {
	m := New(nil, 1)
	m.prs = []models.PRInfo{}

	output := m.renderPRList()
	if output == "" {
		t.Error("empty PR list should render a message")
	}
	if len(output) < 10 {
		t.Error("empty message should be visible")
	}
}

func TestRenderPRListWithPRs(t *testing.T) {
	m := New(nil, 1)
	m.prs = []models.PRInfo{
		{Number: 123, Title: "Test PR 1", State: "OPEN", HeadRef: "feature-1"},
		{Number: 456, Title: "Test PR 2", State: "MERGED", HeadRef: "feature-2"},
		{Number: 789, Title: "Draft PR", State: "OPEN", IsDraft: true, HeadRef: "feature-3"},
	}

	output := m.renderPRList()
	if output == "" {
		t.Error("PR list should render content")
	}
	if len(output) < 50 {
		t.Error("PR list output too short")
	}
}

func TestPRCountMessages(t *testing.T) {
	msg := PRCountLoadedMsg{
		Path:  "/test/repo",
		Count: 5,
	}

	if msg.Path != "/test/repo" {
		t.Errorf("expected path '/test/repo', got %q", msg.Path)
	}
	if msg.Count != 5 {
		t.Errorf("expected count 5, got %d", msg.Count)
	}
}

func TestPRListLoadedMessage(t *testing.T) {
	prs := []models.PRInfo{
		{Number: 1, Title: "PR 1"},
		{Number: 2, Title: "PR 2"},
	}

	msg := PRListLoadedMsg{
		Path: "/test/repo",
		PRs:  prs,
	}

	if msg.Path != "/test/repo" {
		t.Errorf("expected path '/test/repo', got %q", msg.Path)
	}
	if len(msg.PRs) != 2 {
		t.Errorf("expected 2 PRs, got %d", len(msg.PRs))
	}
}

func TestPRDetailLoadedMessage(t *testing.T) {
	detail := models.PRDetail{
		PRInfo: models.PRInfo{
			Number: 123,
			Title:  "Test PR",
		},
		Author: "testuser",
	}

	msg := PRDetailLoadedMsg{
		Path:     "/test/repo",
		PRNumber: 123,
		Detail:   detail,
	}

	if msg.Path != "/test/repo" {
		t.Errorf("expected path '/test/repo', got %q", msg.Path)
	}
	if msg.PRNumber != 123 {
		t.Errorf("expected PR number 123, got %d", msg.PRNumber)
	}
	if msg.Detail.Author != "testuser" {
		t.Errorf("expected author 'testuser', got %q", msg.Detail.Author)
	}
}

func TestPRDetailUpdateWithMessage(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/test/repo"
	m.selectedPR = models.PRInfo{Number: 123}

	detail := models.PRDetail{
		PRInfo: models.PRInfo{
			Number:  123,
			Title:   "Test PR",
			HeadRef: "feature-branch",
			BaseRef: "main",
			State:   "OPEN",
		},
		Author:    "alice",
		Assignees: []string{"bob"},
		Reviewers: []string{"charlie"},
		Additions: 100,
		Deletions: 50,
		Comments:  5,
	}

	msg := PRDetailLoadedMsg{
		Path:     "/test/repo",
		PRNumber: 123,
		Detail:   detail,
	}

	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if m.prDetail.Number != 123 {
		t.Errorf("expected PR #123, got #%d", m.prDetail.Number)
	}
	if m.prDetail.Title != "Test PR" {
		t.Errorf("expected title 'Test PR', got %q", m.prDetail.Title)
	}
	if m.prDetail.Author != "alice" {
		t.Errorf("expected author 'alice', got %q", m.prDetail.Author)
	}
	if len(m.prDetail.Assignees) != 1 {
		t.Errorf("expected 1 assignee, got %d", len(m.prDetail.Assignees))
	}
	if len(m.prDetail.Reviewers) != 1 {
		t.Errorf("expected 1 reviewer, got %d", len(m.prDetail.Reviewers))
	}
}

func TestPRDetailViewRender(t *testing.T) {
	m := New(nil, 1)
	m.width = 120
	m.height = 40
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{
		Path: "/test/repo",
	}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{
			Number:         456,
			Title:          "Add amazing feature",
			HeadRef:        "feature/amazing",
			BaseRef:        "main",
			State:          "OPEN",
			ReviewDecision: "APPROVED",
		},
		Author:    "dev1",
		Assignees: []string{"dev2", "dev3"},
		Reviewers: []string{"reviewer1"},
		Additions: 250,
		Deletions: 100,
		Comments:  10,
		Body:      "This is the PR description",
	}

	output := m.View()

	if !strings.Contains(output, "PR #456") {
		t.Error("output should contain PR number")
	}
	if !strings.Contains(output, "Add amazing feature") {
		t.Error("output should contain PR title")
	}
	if !strings.Contains(output, "dev1") {
		t.Error("output should contain author")
	}
	if !strings.Contains(output, "dev2, dev3") {
		t.Error("output should contain assignees")
	}
	if !strings.Contains(output, "reviewer1") {
		t.Error("output should contain reviewers")
	}
	if !strings.Contains(output, "feature/amazing") {
		t.Error("output should contain head branch")
	}
	if !strings.Contains(output, "main") {
		t.Error("output should contain base branch")
	}
	if !strings.Contains(output, "+250") {
		t.Error("output should contain additions")
	}
	if !strings.Contains(output, "-100") {
		t.Error("output should contain deletions")
	}
	if !strings.Contains(output, "This is the PR description") {
		t.Error("output should contain PR description")
	}
	if !strings.Contains(output, "open in browser") {
		t.Error("output should show open action")
	}
	if !strings.Contains(output, "copy URL") {
		t.Error("output should show copy URL action")
	}
	if !strings.Contains(output, "copy PR number") {
		t.Error("output should show copy PR number action")
	}
	if !strings.Contains(output, "copy branch name") {
		t.Error("output should show copy branch action")
	}
}

func TestStatusMessages(t *testing.T) {
	m := New(nil, 1)

	msg := StatusMsg{Message: "Test status"}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if m.statusMessage != "Test status" {
		t.Errorf("expected status message 'Test status', got %q", m.statusMessage)
	}

	clearMsg := ClearStatusMsg{}
	updatedModel, _ = m.Update(clearMsg)
	m = updatedModel.(Model)

	if m.statusMessage != "" {
		t.Errorf("expected empty status message, got %q", m.statusMessage)
	}
}

func TestCopySuccessMessage(t *testing.T) {
	m := New(nil, 1)

	msg := CopySuccessMsg{Text: "https://github.com/test/pr/123"}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if !strings.Contains(m.statusMessage, "Copied to clipboard") {
		t.Errorf("expected copy success message, got %q", m.statusMessage)
	}
	if !strings.Contains(m.statusMessage, "https://github.com/test/pr/123") {
		t.Errorf("expected URL in message, got %q", m.statusMessage)
	}
}

func TestURLOpenedMessage(t *testing.T) {
	m := New(nil, 1)

	msg := URLOpenedMsg{URL: "https://github.com/test/pr/123"}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if !strings.Contains(m.statusMessage, "Opened in browser") {
		t.Errorf("expected URL opened message, got %q", m.statusMessage)
	}
	if !strings.Contains(m.statusMessage, "https://github.com/test/pr/123") {
		t.Errorf("expected URL in message, got %q", m.statusMessage)
	}
}

func TestPRDetailViewWithStatusMessage(t *testing.T) {
	m := New(nil, 1)
	m.width = 120
	m.height = 40
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{
		Path: "/test/repo",
	}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{
			Number:  123,
			Title:   "Test PR",
			HeadRef: "feature",
			BaseRef: "main",
		},
		Author: "user1",
	}
	m.statusMessage = "Copied to clipboard: #123"

	output := m.View()

	if !strings.Contains(output, "Copied to clipboard: #123") {
		t.Error("output should contain status message")
	}
}

func TestPRDetailErrorHandling(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/test/repo"
	m.selectedPR = models.PRInfo{Number: 999}

	msg := PRDetailLoadedMsg{
		Path:     "/test/repo",
		PRNumber: 999,
		Error:    nil,
		Detail: models.PRDetail{
			PRInfo: models.PRInfo{
				Number: 999,
			},
		},
	}

	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if m.prDetail.Number != 999 {
		t.Errorf("PR detail should be loaded even without error")
	}
}

func TestPRNavigationFlow(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabPRs
	m.selectedRepo = "/test/repo"
	m.prs = []models.PRInfo{
		{Number: 1, Title: "First PR"},
		{Number: 2, Title: "Second PR"},
		{Number: 3, Title: "Third PR"},
	}
	m.detailCursor = 1

	if m.detailCursor >= len(m.prs) {
		t.Error("cursor should be within bounds")
	}

	selectedPR := m.prs[m.detailCursor]
	if selectedPR.Number != 2 {
		t.Errorf("expected PR #2 to be selected, got #%d", selectedPR.Number)
	}

	m.selectedPR = selectedPR
	m.viewMode = ViewModePRDetail

	if m.viewMode != ViewModePRDetail {
		t.Error("should be in PR detail view mode")
	}
	if m.selectedPR.Number != 2 {
		t.Errorf("selected PR should be #2, got #%d", m.selectedPR.Number)
	}
}

func TestPRCountLoading(t *testing.T) {
	m := New(nil, 1)

	msg1 := PRCountLoadedMsg{Path: "/repo1", Count: 5}
	updatedModel, _ := m.Update(msg1)
	m = updatedModel.(Model)

	if m.prCount["/repo1"] != 5 {
		t.Errorf("expected 5 PRs for /repo1, got %d", m.prCount["/repo1"])
	}

	msg2 := PRCountLoadedMsg{Path: "/repo2", Count: 3}
	updatedModel, _ = m.Update(msg2)
	m = updatedModel.(Model)

	if m.prCount["/repo2"] != 3 {
		t.Errorf("expected 3 PRs for /repo2, got %d", m.prCount["/repo2"])
	}

	if m.prCount["/repo1"] != 5 {
		t.Error("first repo PR count should be preserved")
	}
}

func TestEmptyPRDetailFields(t *testing.T) {
	m := New(nil, 1)
	m.width = 120
	m.height = 40
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{
			Number:  100,
			Title:   "Minimal PR",
			HeadRef: "feature",
			BaseRef: "main",
		},
		Author:    "user1",
		Assignees: []string{},
		Reviewers: []string{},
		Comments:  0,
		Body:      "",
	}

	output := m.View()

	if !strings.Contains(output, "Minimal PR") {
		t.Error("should contain PR title")
	}
	if !strings.Contains(output, "user1") {
		t.Error("should contain author")
	}

	if strings.Count(output, "Assignees:") > 0 {
		if !strings.Contains(output, "Assignees:") {
			t.Error("should not show assignees section when empty")
		}
	}
	if strings.Count(output, "Reviewers:") > 0 {
		if !strings.Contains(output, "Reviewers:") {
			t.Error("should not show reviewers section when empty")
		}
	}
}

func TestPRDetailLoadingState(t *testing.T) {
	m := New(nil, 1)
	m.width = 120
	m.height = 40
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}
	// prDetail.Number is 0 (not loaded yet)
	m.prDetail = models.PRDetail{}

	output := m.View()

	if !strings.Contains(output, "Loading PR details") {
		t.Error("should show loading message when PR detail not loaded")
	}
}

func TestPRDetailClearedOnNavigation(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabPRs
	m.selectedRepo = "/test/repo"
	m.prs = []models.PRInfo{
		{Number: 123, Title: "Test PR", State: "OPEN", HeadRef: "feature", BaseRef: "main"},
	}
	m.detailCursor = 0
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{Number: 999}, // Old detail from different PR
	}

	// Simulate Enter key on PR list
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if m.prDetail.Number != 123 {
		t.Errorf("prDetail should be populated with basic info from list, got number %d", m.prDetail.Number)
	}
	if m.viewMode != ViewModePRDetail {
		t.Error("should transition to PR detail view")
	}
	if m.selectedPR.Number != 123 {
		t.Errorf("selected PR should be #123, got #%d", m.selectedPR.Number)
	}
}

func TestPRDetailErrorPreservesBasicInfo(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.selectedPR = models.PRInfo{
		Number:  456,
		Title:   "Feature PR",
		State:   "OPEN",
		HeadRef: "feature",
		BaseRef: "main",
	}
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}

	// Populate prDetail with basic info (simulating progressive loading)
	m.prDetail = models.PRDetail{
		PRInfo: m.selectedPR,
	}

	// Verify basic info is present
	if m.prDetail.Number != 456 {
		t.Fatalf("expected PR #456, got #%d", m.prDetail.Number)
	}

	// Simulate error response from loadPRDetailCmd
	errorMsg := PRDetailLoadedMsg{
		Path:     "/test/repo",
		PRNumber: 456,
		Detail:   models.PRDetail{}, // Empty detail due to error
		Error:    fmt.Errorf("failed to load PR details"),
	}

	updatedModel, _ := m.Update(errorMsg)
	m = updatedModel.(Model)

	// BUG: prDetail.Number should NOT be cleared when there's an error
	// It should preserve the basic info that was already populated
	if m.prDetail.Number == 0 {
		t.Error("ERROR: prDetail.Number was cleared to 0 when error occurred - basic info should be preserved!")
	}
	if m.prDetail.Number != 456 {
		t.Errorf("expected PR #456 to be preserved after error, got #%d", m.prDetail.Number)
	}
	if m.prDetail.Title != "Feature PR" {
		t.Errorf("expected title to be preserved after error, got %q", m.prDetail.Title)
	}
}

func TestPRDetailProgressiveLoading(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabPRs
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}

	// PR list data (what we have immediately)
	m.prs = []models.PRInfo{
		{
			Number:         456,
			Title:          "Feature PR",
			State:          "OPEN",
			URL:            "https://github.com/test/pr/456",
			HeadRef:        "feature-branch",
			BaseRef:        "main",
			ReviewDecision: "APPROVED",
		},
	}
	m.detailCursor = 0

	// Navigate to PR detail
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	// Verify basic info is immediately available
	if m.prDetail.Number != 456 {
		t.Errorf("expected PR #456, got #%d", m.prDetail.Number)
	}
	if m.prDetail.Title != "Feature PR" {
		t.Errorf("expected title 'Feature PR', got %q", m.prDetail.Title)
	}
	if m.prDetail.State != "OPEN" {
		t.Errorf("expected state 'OPEN', got %q", m.prDetail.State)
	}
	if m.prDetail.URL != "https://github.com/test/pr/456" {
		t.Errorf("expected URL to be set, got %q", m.prDetail.URL)
	}
	if m.prDetail.HeadRef != "feature-branch" {
		t.Error("expected HeadRef to be set immediately")
	}
	if m.prDetail.BaseRef != "main" {
		t.Error("expected BaseRef to be set immediately")
	}

	// Full details (Author, etc.) should be empty initially
	if m.prDetail.Author != "" {
		t.Error("Author should be empty until full details load")
	}
	if len(m.prDetail.Assignees) > 0 {
		t.Error("Assignees should be empty until full details load")
	}
	if len(m.prDetail.Reviewers) > 0 {
		t.Error("Reviewers should be empty until full details load")
	}
}

func TestPRDetailProgressiveView(t *testing.T) {
	m := New(nil, 1)
	m.width = 120
	m.height = 40
	m.viewMode = ViewModePRDetail
	m.selectedRepo = "/test/repo"
	m.summaries["/test/repo"] = models.RepoSummary{Path: "/test/repo"}

	// Partial data (from list)
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{
			Number:         100,
			Title:          "Test PR",
			State:          "OPEN",
			HeadRef:        "feature",
			BaseRef:        "main",
			ReviewDecision: "APPROVED",
		},
		// Author and other fields empty (not loaded yet)
	}

	output := m.View()

	// Basic info should be visible
	if !strings.Contains(output, "PR #100") {
		t.Error("should show PR number immediately")
	}
	if !strings.Contains(output, "Test PR") {
		t.Error("should show title immediately")
	}
	if !strings.Contains(output, "feature") {
		t.Error("should show head branch immediately")
	}
	if !strings.Contains(output, "main") {
		t.Error("should show base branch immediately")
	}

	// Should show loading indicator
	if !strings.Contains(output, "loading details") {
		t.Error("should show loading indicator when Author is empty")
	}

	// Now simulate full details loaded
	m.prDetail.Author = "testuser"
	m.prDetail.Additions = 100
	m.prDetail.Deletions = 50

	output = m.View()

	// Should no longer show loading indicator
	if strings.Contains(output, "loading details") {
		t.Error("should not show loading indicator when Author is populated")
	}

	// Full details should be visible
	if !strings.Contains(output, "testuser") {
		t.Error("should show author when loaded")
	}
	if !strings.Contains(output, "+100") {
		t.Error("should show additions when loaded")
	}
}
