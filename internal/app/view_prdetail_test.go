//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestRenderPRDetailShowsChecksAndLatestComment(t *testing.T) {
	t.Parallel()

	started := time.Now().Add(-time.Hour)
	m := New(nil, 1)
	m.width, m.height = 120, 40
	m.selectedRepo = testRepo1Path
	m.summaries[testRepo1Path] = models.RepoSummary{Path: testRepo1Path}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{Number: 42, Title: "Add login", State: "OPEN", HeadRef: featureBranchName},
		Author: "alice",
		Body:   "Adds the login flow",
		CheckDetails: []models.CheckDetail{
			{
				Name: "ci", Workflow: "CI", Status: "COMPLETED", Conclusion: "SUCCESS",
				StartedAt: started, CompletedAt: started.Add(90 * time.Second),
			},
			{Name: "lint", Status: "IN_PROGRESS"},
		},
		LatestComment: &models.PRComment{Author: "dave", Body: "looks good now", CreatedAt: started},
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

func TestRenderPRDetailWithoutChecksOrComments(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width, m.height = 120, 40
	m.selectedRepo = testRepo1Path
	m.summaries[testRepo1Path] = models.RepoSummary{Path: testRepo1Path}
	m.prDetail = models.PRDetail{
		PRInfo: models.PRInfo{Number: 42, Title: "Add login", State: "OPEN"},
		Author: "alice",
	}

	rendered := m.renderPRDetail()

	if strings.Contains(rendered, "Checks") || strings.Contains(rendered, "Latest comment") {
		t.Errorf("expected no empty check/comment sections:\n%s", rendered)
	}
}
