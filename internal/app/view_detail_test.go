//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestFormatBranchPRCell(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pr       *models.PRInfo
		expected string
	}{
		{"no pull request", nil, emDash},
		{"draft", &models.PRInfo{Number: 7, IsDraft: true}, "#7 draft"},
		{"approved", &models.PRInfo{Number: 7, ReviewDecision: "APPROVED"}, "#7 ✓"},
		{"changes requested", &models.PRInfo{Number: 7, ReviewDecision: "CHANGES_REQUESTED"}, "#7 ✗"},
		{"awaiting review", &models.PRInfo{Number: 7}, "#7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatBranchPRCell(tt.pr); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFormatChecksCell(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pr       *models.PRInfo
		expected string
	}{
		{"no pull request", nil, emDash},
		{"no checks", &models.PRInfo{Number: 1}, emDash},
		{"passing", &models.PRInfo{Checks: models.ChecksStatus{Total: 3, Passing: 3}}, "passing 3/3"},
		{"failing", &models.PRInfo{Checks: models.ChecksStatus{Total: 3, Passing: 2, Failing: 1}}, "failing 2/3"},
		{"pending", &models.PRInfo{Checks: models.ChecksStatus{Total: 2, Passing: 1, Pending: 1}}, "pending 1/2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatChecksCell(tt.pr); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestPRsByHeadRef(t *testing.T) {
	t.Parallel()

	prs := []models.PRInfo{{Number: 1, HeadRef: "feature"}, {Number: 2, HeadRef: "fix"}}
	byRef := prsByHeadRef(prs)

	if pr, ok := byRef["fix"]; !ok || pr.Number != 2 {
		t.Errorf("expected PR #2 for 'fix', got %+v (ok=%v)", pr, ok)
	}
	if _, ok := byRef["missing"]; ok {
		t.Error("expected no PR for an unknown branch")
	}
}

func TestRenderBranchListShowsParallelCheckout(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.selectedRepo = testRepo1Path
	m.summaries[testRepo1Path] = models.RepoSummary{
		Path: testRepo1Path, Branch: mainBranchName, RemoteRepo: "acme/app",
	}
	m.summaries["/repos/app-feature"] = models.RepoSummary{
		Path: "/repos/app-feature", Branch: featureBranchName, RemoteRepo: "acme/app", Ahead: 2,
	}
	m.branches = []models.BranchInfo{
		{Name: mainBranchName, IsCurrent: true},
		{Name: featureBranchName},
	}
	m.prs = []models.PRInfo{
		{Number: 42, HeadRef: featureBranchName, Checks: models.ChecksStatus{Total: 2, Passing: 2}},
	}

	rendered := m.renderBranchList()

	if !strings.Contains(rendered, "app-feature") {
		t.Errorf("expected the peer checkout folder in the branch list:\n%s", rendered)
	}
	if !strings.Contains(rendered, "#42") || !strings.Contains(rendered, "passing 2/2") {
		t.Errorf("expected PR and checks columns in the branch list:\n%s", rendered)
	}
	if !strings.Contains(rendered, "here") {
		t.Errorf("expected the current branch marked as checked out here:\n%s", rendered)
	}
}
