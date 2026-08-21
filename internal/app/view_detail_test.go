//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestIsStaleBranch(t *testing.T) {
	t.Parallel()

	old := time.Now().Add(-2 * staleBranchAge)
	recent := time.Now().Add(-time.Hour)

	tests := []struct {
		name   string
		branch vcs.BranchInfo
		want   bool
	}{
		{"old commit", vcs.BranchInfo{Name: "feature/x", LastCommit: old}, true},
		{"recent commit", vcs.BranchInfo{Name: "feature/x", LastCommit: recent}, false},
		{"no commit info", vcs.BranchInfo{Name: "feature/x"}, false},
		{
			"current branch exempt despite age",
			vcs.BranchInfo{Name: "feature/x", LastCommit: old, IsCurrent: true},
			false,
		},
		{"default branch exempt despite age", vcs.BranchInfo{Name: mainBranchName, LastCommit: old}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isStaleBranch(tt.branch); got != tt.want {
				t.Errorf("isStaleBranch(%+v) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

func TestFormatBranchPRCell(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pr       *forge.PullRequest
		expected string
	}{
		{"no pull request", nil, emDash},
		{"draft", &forge.PullRequest{Number: 7, IsDraft: true}, "#7 draft"},
		{"approved", &forge.PullRequest{Number: 7, ReviewDecision: "APPROVED"}, "#7 ✓"},
		{"changes requested", &forge.PullRequest{Number: 7, ReviewDecision: "CHANGES_REQUESTED"}, "#7 ✗"},
		{"awaiting review", &forge.PullRequest{Number: 7}, "#7"},
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
		pr       *forge.PullRequest
		expected string
	}{
		{"no pull request", nil, emDash},
		{"no checks", &forge.PullRequest{Number: 1}, emDash},
		{"passing", &forge.PullRequest{Checks: forge.ChecksStatus{Total: 3, Passing: 3}}, "passing 3/3"},
		{"failing", &forge.PullRequest{Checks: forge.ChecksStatus{Total: 3, Passing: 2, Failing: 1}}, "failing 2/3"},
		{"pending", &forge.PullRequest{Checks: forge.ChecksStatus{Total: 2, Passing: 1, Pending: 1}}, "pending 1/2"},
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

	prs := []forge.PullRequest{{Number: 1, HeadRef: "feature"}, {Number: 2, HeadRef: "fix"}}
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
	m.width = 160
	m.selectedRepo = testRepo1Path
	m.summaries[testRepo1Path] = models.RepoSummary{
		RepoSummary: vcs.RepoSummary{Path: testRepo1Path, Branch: mainBranchName, RemoteRepo: "acme/app"},
	}
	m.summaries["/repos/app-feature"] = models.RepoSummary{
		RepoSummary: vcs.RepoSummary{
			Path:       "/repos/app-feature",
			Branch:     featureBranchName,
			RemoteRepo: "acme/app",
			Ahead:      2,
		},
	}
	m.branches = []vcs.BranchInfo{
		{Name: mainBranchName, IsCurrent: true},
		{Name: featureBranchName},
	}
	m.prs = []forge.PullRequest{
		{Number: 42, HeadRef: featureBranchName, Checks: forge.ChecksStatus{Total: 2, Passing: 2}},
	}

	rendered := renderPanel(m, panelBranches)

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
