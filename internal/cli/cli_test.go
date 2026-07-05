package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/cli"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

var errGH = errors.New("gh failed")

func stubClient(pr *models.PRInfo, prs []models.PRInfo, err error, calls *int) cli.GitHubClient {
	return cli.NewGitHubClient(
		func(_ context.Context, _, _, _ string) (*models.PRInfo, error) {
			*calls++
			return pr, err
		},
		func(_ context.Context, _, _ string) ([]models.PRInfo, error) {
			*calls++
			return prs, err
		},
	)
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestLookupPR(t *testing.T) {
	cachedPR := &models.PRInfo{Number: 7, Title: "cached"}
	freshPR := &models.PRInfo{Number: 9, Title: "fresh"}

	tests := []struct {
		name      string
		upstream  string
		cached    *models.PRInfo
		fresh     bool
		fetchErr  error
		expected  *models.PRInfo
		wantCalls int
	}{
		{name: "no upstream skips lookup", upstream: "", fresh: true, expected: nil, wantCalls: 0},
		{name: "cache hit without fresh", upstream: "origin/main", cached: cachedPR, expected: cachedPR},
		{name: "cache hit ignores fresh", upstream: "origin/main", cached: cachedPR, fresh: true, expected: cachedPR},
		{name: "cache miss without fresh skips gh", upstream: "origin/main", expected: nil, wantCalls: 0},
		{name: "cache miss with fresh fetches", upstream: "origin/main", fresh: true, expected: freshPR, wantCalls: 1},
		{
			name: "fetch error yields nil", upstream: "origin/main", fresh: true,
			fetchErr: errGH, expected: nil, wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			if tt.cached != nil {
				cache.PRCache.Set(tt.upstream+":main", tt.cached)
			}

			calls := 0
			client := stubClient(freshPR, nil, tt.fetchErr, &calls)

			got := cli.LookupPR(context.Background(), client, "/repo", "main", tt.upstream, tt.fresh)
			if got != tt.expected && (got == nil || tt.expected == nil || got.Number != tt.expected.Number) {
				t.Errorf("expected %+v, got %+v", tt.expected, got)
			}
			if calls != tt.wantCalls {
				t.Errorf("expected %d gh calls, got %d", tt.wantCalls, calls)
			}
		})
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestLookupPRCount(t *testing.T) {
	cachedPRs := []models.PRInfo{{Number: 1}, {Number: 2}}
	freshPRs := []models.PRInfo{{Number: 3}}

	tests := []struct {
		name      string
		upstream  string
		cached    []models.PRInfo
		fresh     bool
		fetchErr  error
		expected  *int
		wantCalls int
	}{
		{name: "no upstream skips lookup", upstream: "", fresh: true, expected: nil, wantCalls: 0},
		{name: "cache hit without fresh", upstream: "origin/main", cached: cachedPRs, expected: intPtr(2)},
		{name: "cache miss without fresh skips gh", upstream: "origin/main", expected: nil, wantCalls: 0},
		{
			name: "cache miss with fresh fetches", upstream: "origin/main", fresh: true,
			expected: intPtr(1), wantCalls: 1,
		},
		{
			name: "fetch error yields nil", upstream: "origin/main", fresh: true,
			fetchErr: errGH, expected: nil, wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			if tt.cached != nil {
				cache.PRListCache.Set(tt.upstream+":all_prs", tt.cached)
			}

			calls := 0
			client := stubClient(nil, freshPRs, tt.fetchErr, &calls)

			got := cli.LookupPRCount(context.Background(), client, "/repo", tt.upstream, tt.fresh)
			switch {
			case got == nil && tt.expected != nil, got != nil && tt.expected == nil:
				t.Errorf("expected %v, got %v", tt.expected, got)
			case got != nil && *got != *tt.expected:
				t.Errorf("expected count %d, got %d", *tt.expected, *got)
			}
			if calls != tt.wantCalls {
				t.Errorf("expected %d gh calls, got %d", tt.wantCalls, calls)
			}
		})
	}
}

func intPtr(n int) *int { return &n }

func TestRepoJSONShape(t *testing.T) {
	t.Parallel()

	lastModified := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name     string
		repo     cli.Repo
		expected string
	}{
		{
			name: "minimal repo omits optional fields",
			repo: cli.NewRepo(&models.RepoSummary{
				Path:    "/repos/demo",
				VCSType: models.VCSTypeGit,
				Branch:  "main",
			}, 0, nil, nil),
			expected: `{"path":"/repos/demo","name":"demo","vcs":"git","branch":"main",` +
				`"ahead":0,"behind":0,"staged":0,"unstaged":0,"untracked":0,"conflicted":0,` +
				`"dirty":false,"status":"clean","stash_count":0,"worktree_count":0}`,
		},
		{
			name: "full repo includes pr and counts",
			repo: cli.NewRepo(&models.RepoSummary{
				Path:         "/repos/wip",
				VCSType:      models.VCSTypeJJ,
				Branch:       "feature",
				Upstream:     "origin/feature",
				Ahead:        2,
				Behind:       1,
				Unstaged:     3,
				StashCount:   1,
				LastModified: lastModified,
			}, 2, &models.PRInfo{
				Number:  42,
				Title:   "Add feature",
				State:   "OPEN",
				URL:     "https://example.com/pr/42",
				HeadRef: "feature",
				BaseRef: "main",
				Checks:  models.ChecksStatus{Total: 2, Passing: 1, Pending: 1},
			}, intPtr(3)),
			expected: `{"path":"/repos/wip","name":"wip","vcs":"jj","branch":"feature",` +
				`"upstream":"origin/feature","ahead":2,"behind":1,"staged":0,"unstaged":3,` +
				`"untracked":0,"conflicted":0,"dirty":true,"status":"diverged",` +
				`"stash_count":1,"worktree_count":2,"last_modified":"2026-01-02T03:04:05Z",` +
				`"pr":{"number":42,"title":"Add feature","state":"OPEN",` +
				`"url":"https://example.com/pr/42","is_draft":false,"head_ref":"feature",` +
				`"base_ref":"main","checks":{"total":2,"passing":1,"failing":0,"pending":1,"skipped":0}},` +
				`"pr_count":3}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(tt.repo)
			if err != nil {
				t.Fatalf("marshaling repo: %v", err)
			}
			if string(got) != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, got)
			}
		})
	}
}

func TestWriteOutput(t *testing.T) {
	t.Parallel()

	out := cli.Output{
		GeneratedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		ScanPaths:   []string{"/repos"},
		Repos:       []cli.Repo{},
	}

	var buf bytes.Buffer
	if err := cli.WriteOutput(&buf, out); err != nil {
		t.Fatalf("writing output: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	repos, ok := decoded["repos"].([]any)
	if !ok {
		t.Fatalf("expected repos to be an array, got %T", decoded["repos"])
	}
	if len(repos) != 0 {
		t.Errorf("expected empty repos array, got %d entries", len(repos))
	}
	if decoded["generated_at"] != "2026-01-02T03:04:05Z" {
		t.Errorf("unexpected generated_at: %v", decoded["generated_at"])
	}
}
