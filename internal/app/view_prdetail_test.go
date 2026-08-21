//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

func TestRenderPRDetailShowsChecksAndLatestComment(t *testing.T) {
	t.Parallel()

	started := time.Now().Add(-time.Hour)
	m := New(nil, 1)
	m.width, m.height = 120, 40
	m.selectedRepo = testRepo1Path
	m.summaries[testRepo1Path] = models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: testRepo1Path}}
	m.prDetail = forge.PRDetail{
		PullRequest: forge.PullRequest{Number: 42, Title: "Add login", State: "OPEN", HeadRef: featureBranchName},
		Author:      "alice",
		Body:        "Adds the login flow",
		CheckDetails: []forge.CheckDetail{
			{
				Name: "ci", Workflow: "CI", Status: "COMPLETED", Conclusion: "SUCCESS",
				StartedAt: started, CompletedAt: started.Add(90 * time.Second),
			},
			{Name: "lint", Status: "IN_PROGRESS"},
		},
		LatestComment: &forge.PRComment{Author: "dave", Body: "looks good now", CreatedAt: started},
	}

	rendered := m.renderPRDetail()

	want := []string{
		"Checks", "CI / ci", "success", "1m30s", "in progress",
		"Latest comment", "dave", "looks good now",
	}
	for _, fragment := range want {
		if !strings.Contains(rendered, fragment) {
			t.Errorf("expected %q in the PR detail view:\n%s", fragment, rendered)
		}
	}
}

// Only a failing or skipped check should draw the eye; everything else
// (passing, pending, unknown) reads as settled so a long checks list is not a
// wall of color.
func TestCheckStatusStyle_OnlyFailingAndSkippedStandOut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   lipgloss.Style
	}{
		{"failure", styles.ErrorStyle},
		{"error", styles.ErrorStyle},
		{"cancelled", styles.ErrorStyle}, //nolint:misspell // GitHub's own conclusion value is spelled "cancelled"
		{"timed_out", styles.ErrorStyle},
		{"skipped", styles.WarningStyle},
		{"success", styles.SubtitleStyle},
		{"neutral", styles.SubtitleStyle},
		{"pending", styles.SubtitleStyle},
		{"in_progress", styles.SubtitleStyle},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()
			if got := checkStatusStyle(tt.status); got.String() != tt.want.String() {
				t.Errorf("checkStatusStyle(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestRenderPRDetailWithoutChecksOrComments(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width, m.height = 120, 40
	m.selectedRepo = testRepo1Path
	m.summaries[testRepo1Path] = models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: testRepo1Path}}
	m.prDetail = forge.PRDetail{
		PullRequest: forge.PullRequest{Number: 42, Title: "Add login", State: "OPEN"},
		Author:      "alice",
	}

	rendered := m.renderPRDetail()

	if strings.Contains(rendered, "Checks") || strings.Contains(rendered, "Latest comment") {
		t.Errorf("expected no empty check/comment sections:\n%s", rendered)
	}
}
