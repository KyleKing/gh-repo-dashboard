package github_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
)

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRsForRepoDerivesLatestActivity(t *testing.T) {
	cache.ClearAll()

	listJSON := []byte(`[
		{
			"number": 7,
			"title": "Reviewed last",
			"state": "OPEN",
			"headRefName": "feature/a",
			"comments": [{"author": {"login": "commenter"}, "createdAt": "2026-01-01T00:00:00Z"}],
			"reviews": [{"author": {"login": "reviewer"}, "submittedAt": "2026-02-01T00:00:00Z"}]
		},
		{
			"number": 8,
			"title": "Commented last",
			"state": "OPEN",
			"headRefName": "feature/b",
			"comments": [{"author": {"login": "commenter"}, "createdAt": "2026-03-01T00:00:00Z"}],
			"reviews": [{"author": {"login": "reviewer"}, "submittedAt": "2026-02-01T00:00:00Z"}]
		},
		{
			"number": 9,
			"title": "Silent",
			"state": "OPEN",
			"headRefName": "feature/c",
			"comments": [],
			"reviews": []
		}
	]`)

	ctx, calls := stubRunGH(listJSON, nil)

	prs, err := github.GetPRsForRepo(ctx, "/repo", "owner/repo")
	if err != nil {
		t.Fatalf("GetPRsForRepo: %v", err)
	}

	if len(*calls) != 1 {
		t.Errorf("made %d gh calls for one repo, want 1", len(*calls))
	}

	if joined := strings.Join((*calls)[0], " "); !strings.Contains(joined, "comments") ||
		!strings.Contains(joined, "reviews") {
		t.Errorf("gh call does not request the activity fields: %q", joined)
	}

	want := []struct {
		author string
		at     time.Time
	}{
		{author: "reviewer", at: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)},
		{author: "commenter", at: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)},
	}

	for i, w := range want {
		got := prs[i].Activity
		if got == nil {
			t.Fatalf("PR #%d has no activity", prs[i].Number)
		}

		if got.Author != w.author || !got.At.Equal(w.at) {
			t.Errorf("PR #%d activity = %s at %s, want %s at %s",
				prs[i].Number, got.Author, got.At, w.author, w.at)
		}
	}

	if prs[2].Activity != nil {
		t.Errorf("a PR with no comments and no reviews reported activity: %+v", prs[2].Activity)
	}
}
