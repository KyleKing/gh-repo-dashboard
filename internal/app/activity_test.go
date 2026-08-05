//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestActivitySummaryReportsAgeAndAuthor(t *testing.T) {
	t.Parallel()

	pr := models.PRInfo{
		Number:   7,
		Activity: &models.PRActivity{Author: "reviewer", At: time.Now().Add(-2 * time.Hour)},
	}

	got := pr.ActivitySummary()
	if !strings.Contains(got, "reviewer") || !strings.Contains(got, "hour") {
		t.Errorf("activity summary = %q, want an age and an author", got)
	}

	quiet := models.PRInfo{Number: 8}
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

			pr := models.PRInfo{Number: 1, State: "OPEN", ReviewDecision: tt.decision}
			if got := prStateCell(&pr); got != tt.want {
				t.Errorf("state cell = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPRPanelShowsActivityInsteadOfReview(t *testing.T) {
	t.Parallel()

	m := detailTableModel(160)
	m.prs[0].Activity = &models.PRActivity{Author: "cjs", At: time.Now().Add(-3 * time.Hour)}

	rendered := plainText(renderPanel(m, panelPRs))
	if !strings.Contains(rendered, "cjs") || !strings.Contains(rendered, "3 hours ago") {
		t.Errorf("PR panel does not name who the PR is waiting on:\n%s", rendered)
	}

	if strings.Contains(rendered, models.ReviewApproved) {
		t.Errorf("the review decision should have given its width to activity:\n%s", rendered)
	}
}

func TestCheckoutPRRefusesABranchAPeerHolds(t *testing.T) {
	t.Parallel()

	m := peerFleet(140, 35)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelPRs
	m.prs = []models.PRInfo{{Number: 3, Title: "Peer work", State: "OPEN", HeadRef: "feature/side"}}
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
	m.focusedPanel = panelPRs
	m.prs = []models.PRInfo{{Number: 4, Title: "New work", State: "OPEN", HeadRef: "feature/fresh"}}

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
