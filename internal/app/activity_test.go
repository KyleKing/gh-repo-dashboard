//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestActivitySummaryReportsAgeAndAuthor(t *testing.T) {
	t.Parallel()

	pr := forge.PullRequest{
		Number:   7,
		Activity: &forge.PRActivity{Author: "reviewer", At: time.Now().Add(-2 * time.Hour)},
	}

	got := pr.ActivitySummary()
	if !strings.Contains(got, "reviewer") || !strings.Contains(got, "hour") {
		t.Errorf("activity summary = %q, want an age and an author", got)
	}

	quiet := forge.PullRequest{Number: 8}
	if got := quiet.ActivitySummary(); got != emDash {
		t.Errorf("a PR with no comments or reviews summarizes as %q, want %q", got, emDash)
	}
}

func TestReviewDecisionFoldsIntoTheStateCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decision string
		want     string
	}{
		{name: "approved", decision: "APPROVED", want: "OPEN ✓"},
		{name: "changes requested", decision: "CHANGES_REQUESTED", want: "OPEN ✗"},
		{name: "awaiting review", decision: "", want: "OPEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pr := forge.PullRequest{Number: 1, State: "OPEN", ReviewDecision: tt.decision}
			if got := prStateCell(&pr); got != tt.want {
				t.Errorf("state cell = %q, want %q", got, tt.want)
			}
		})
	}
}

// The PRs tab has to say who a pull request is waiting on, which is the last
// person to touch it rather than the review decision.
func TestPRListShowsActivityInsteadOfReview(t *testing.T) {
	t.Parallel()

	m := focusedModel(200, 40)
	m.viewMode = ViewModePRList
	m.prSearch = []forge.PullRequest{{
		Number: 9, Title: "Add a thing", State: "OPEN", ReviewDecision: forge.ReviewApproved,
		Activity: &forge.PRActivity{Author: "cjs", At: time.Now().Add(-3 * time.Hour)},
	}}

	rendered := plainText(m.renderPRList())
	if !strings.Contains(rendered, "cjs") || !strings.Contains(rendered, "3 hours ago") {
		t.Errorf("the PRs tab does not name who the pull request is waiting on:\n%s", rendered)
	}
}

func TestCheckoutPRRefusesABranchAPeerHolds(t *testing.T) {
	t.Parallel()

	m := peerFleet(140, 35)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelBranches
	m.branches = []models.BranchInfo{{Name: "feature/side"}}
	m.prs = []forge.PullRequest{{Number: 3, Title: "Peer work", State: "OPEN", HeadRef: "feature/side"}}
	m.worktrees = []models.WorktreeInfo{{Path: "/dev/app-a-wt", Branch: "feature/side"}}

	_, cmd := m.startCheckoutPR()
	if cmd == nil {
		t.Fatal("expected a status message refusing the checkout")
	}

	msg, ok := cmd().(StatusMsg)
	if !ok {
		t.Fatalf("got %T, want a StatusMsg", cmd())
	}

	if !strings.Contains(msg.Message, "app-a-wt") {
		t.Errorf("refusal %q does not name the checkout holding the branch", msg.Message)
	}
}

func TestCheckoutPRConfirmsBeforeSwitching(t *testing.T) {
	t.Parallel()

	m := peerFleet(140, 35)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelBranches
	m.branches = []models.BranchInfo{{Name: "feature/fresh"}}
	m.prs = []forge.PullRequest{{Number: 4, Title: "New work", State: "OPEN", HeadRef: "feature/fresh"}}

	next, _ := m.startCheckoutPR()
	gated, ok := next.(Model)
	if !ok {
		t.Fatalf("got %T, want Model", next)
	}

	if gated.viewMode != ViewModeConfirm {
		t.Errorf("checkout landed in %v, want the confirm gate", gated.viewMode)
	}

	if gated.pendingAction == nil || !strings.Contains(gated.pendingAction.detail, "feature/fresh") {
		t.Errorf("confirm prompt does not name the branch: %+v", gated.pendingAction)
	}
}
