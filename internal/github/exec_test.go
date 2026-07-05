package github

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

var (
	errGHFailed   = errors.New("gh failed")
	errNoPRsFound = errors.New("no pull requests found")
)

// stubRunGH returns a context that makes runGH answer with (out, err) instead
// of shelling out, plus a pointer to the recorded call args. It's local to the
// returned context, so subtests using their own stubRunGH call can run with
// t.Parallel() safely.
func stubRunGH(out []byte, err error) (context.Context, *[][]string) {
	var calls [][]string
	ctx := withGHRunner(context.Background(), func(_ context.Context, _ string, _ []string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return out, err
	})

	return ctx, &calls
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRForBranch(t *testing.T) {
	successJSON := []byte(`{
		"number": 42,
		"title": "Add feature",
		"state": "OPEN",
		"url": "https://github.com/owner/repo/pull/42",
		"isDraft": true,
		"mergeStateStatus": "CLEAN",
		"headRefName": "feature-branch",
		"baseRefName": "main",
		"statusCheckRollup": [
			{"status": "COMPLETED", "conclusion": "SUCCESS"},
			{"status": "IN_PROGRESS"},
			{"state": "FAILURE"}
		]
	}`)

	tests := []struct {
		name      string
		output    []byte
		runErr    error
		expected  *models.PRInfo
		expectErr bool
		cachesNil bool
	}{
		{
			name:   "success",
			output: successJSON,
			expected: &models.PRInfo{
				Number:    42,
				Title:     "Add feature",
				State:     "OPEN",
				URL:       "https://github.com/owner/repo/pull/42",
				IsDraft:   true,
				Mergeable: "CLEAN",
				HeadRef:   "feature-branch",
				BaseRef:   "main",
				Checks:    models.ChecksStatus{Total: 3, Passing: 1, Pending: 1, Failing: 1},
			},
		},
		{
			name:      "gh error caches nil",
			runErr:    errNoPRsFound,
			expectErr: true,
			cachesNil: true,
		},
		{
			name:      "malformed JSON",
			output:    []byte(`{"number": "not-a-number"}`),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx, calls := stubRunGH(tt.output, tt.runErr)

			pr, err := GetPRForBranch(ctx, "/repo", "feature-branch", "owner/repo")
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(pr, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, pr)
			}

			if tt.expected != nil || tt.cachesNil {
				cachedPR, cachedErr := GetPRForBranch(ctx, "/repo", "feature-branch", "owner/repo")
				if cachedErr != nil {
					t.Errorf("expected cached result without error, got %v", cachedErr)
				}
				if !reflect.DeepEqual(cachedPR, tt.expected) {
					t.Errorf("expected cached %+v, got %+v", tt.expected, cachedPR)
				}
				if len(*calls) != 1 {
					t.Errorf("expected 1 gh invocation, got %d", len(*calls))
				}
			}
		})
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRForBranchArgs(t *testing.T) {
	cache.ClearAll()
	ctx, calls := stubRunGH([]byte(`{"number": 1}`), nil)

	if _, err := GetPRForBranch(ctx, "/repo", "my-branch", "owner/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"pr", "view", "my-branch",
		"--json", "number,title,state,url,isDraft,mergeStateStatus,headRefName,baseRefName,statusCheckRollup",
	}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], expected) {
		t.Errorf("expected args %v, got %v", expected, *calls)
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRDetail(t *testing.T) {
	successJSON := []byte(`{
		"number": 7,
		"title": "Fix bug",
		"state": "OPEN",
		"url": "https://github.com/owner/repo/pull/7",
		"isDraft": false,
		"mergeStateStatus": "BLOCKED",
		"headRefName": "bugfix",
		"baseRefName": "main",
		"body": "Fixes the bug",
		"author": {"login": "alice"},
		"assignees": [{"login": "bob"}, {"login": "carol"}],
		"reviewRequests": [{"login": "dave"}],
		"createdAt": "2026-01-02T03:04:05Z",
		"updatedAt": "2026-01-03T06:07:08Z",
		"additions": 10,
		"deletions": 3,
		"comments": 2,
		"reviewDecision": "CHANGES_REQUESTED"
	}`)

	expectedDetail := &models.PRDetail{
		PRInfo: models.PRInfo{
			Number:         7,
			Title:          "Fix bug",
			State:          "OPEN",
			URL:            "https://github.com/owner/repo/pull/7",
			IsDraft:        false,
			Mergeable:      "BLOCKED",
			HeadRef:        "bugfix",
			BaseRef:        "main",
			ReviewDecision: "CHANGES_REQUESTED",
		},
		Body:      "Fixes the bug",
		Author:    "alice",
		Assignees: []string{"bob", "carol"},
		Reviewers: []string{"dave"},
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt: time.Date(2026, 1, 3, 6, 7, 8, 0, time.UTC),
		Additions: 10,
		Deletions: 3,
		Comments:  2,
	}

	tests := []struct {
		name      string
		output    []byte
		runErr    error
		expected  *models.PRDetail
		expectErr bool
	}{
		{name: "success", output: successJSON, expected: expectedDetail},
		{name: "gh error", runErr: errGHFailed, expectErr: true},
		{name: "malformed JSON", output: []byte(`not json`), expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx, calls := stubRunGH(tt.output, tt.runErr)

			detail, err := GetPRDetail(ctx, "/repo", 7)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(detail, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, detail)
			}

			cachedDetail, err := GetPRDetail(ctx, "/repo", 7)
			if err != nil {
				t.Fatalf("unexpected error on cached call: %v", err)
			}
			if !reflect.DeepEqual(cachedDetail, tt.expected) {
				t.Errorf("expected cached %+v, got %+v", tt.expected, cachedDetail)
			}
			if len(*calls) != 1 {
				t.Errorf("expected 1 gh invocation, got %d", len(*calls))
			}
		})
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRsForRepo(t *testing.T) {
	successJSON := []byte(`[
		{
			"number": 1,
			"title": "First",
			"state": "OPEN",
			"url": "https://github.com/owner/repo/pull/1",
			"isDraft": false,
			"headRefName": "one",
			"baseRefName": "main",
			"reviewDecision": "APPROVED"
		},
		{
			"number": 2,
			"title": "Second",
			"state": "OPEN",
			"url": "https://github.com/owner/repo/pull/2",
			"isDraft": true,
			"headRefName": "two",
			"baseRefName": "main",
			"reviewDecision": ""
		}
	]`)

	tests := []struct {
		name      string
		upstream  string
		output    []byte
		runErr    error
		expected  []models.PRInfo
		expectErr bool
		expectGH  bool
	}{
		{
			name:     "empty upstream short-circuits",
			upstream: "",
			expected: []models.PRInfo{},
		},
		{
			name:     "success",
			upstream: "owner/repo",
			output:   successJSON,
			expected: []models.PRInfo{
				{Number: 1, Title: "First", State: "OPEN", URL: "https://github.com/owner/repo/pull/1", HeadRef: "one", BaseRef: "main", ReviewDecision: "APPROVED"},
				{Number: 2, Title: "Second", State: "OPEN", URL: "https://github.com/owner/repo/pull/2", IsDraft: true, HeadRef: "two", BaseRef: "main"},
			},
			expectGH: true,
		},
		{
			name:      "gh error returns empty list",
			upstream:  "owner/repo",
			runErr:    errGHFailed,
			expected:  []models.PRInfo{},
			expectErr: true,
			expectGH:  true,
		},
		{
			name:      "malformed JSON returns empty list",
			upstream:  "owner/repo",
			output:    []byte(`{"not": "an array"}`),
			expected:  []models.PRInfo{},
			expectErr: true,
			expectGH:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx, calls := stubRunGH(tt.output, tt.runErr)

			prs, err := GetPRsForRepo(ctx, "/repo", tt.upstream)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(prs, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, prs)
			}

			expectedCalls := 0
			if tt.expectGH {
				expectedCalls = 1
			}
			if len(*calls) != expectedCalls {
				t.Errorf("expected %d gh invocations, got %d", expectedCalls, len(*calls))
			}
		})
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRsForRepoUsesCache(t *testing.T) {
	cache.ClearAll()
	ctx, calls := stubRunGH([]byte(`[{"number": 5, "title": "Cached"}]`), nil)

	first, err := GetPRsForRepo(ctx, "/repo", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := GetPRsForRepo(ctx, "/repo", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("expected cached result %+v, got %+v", first, second)
	}
	if len(*calls) != 1 {
		t.Errorf("expected 1 gh invocation, got %d", len(*calls))
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRCount(t *testing.T) {
	tests := []struct {
		name      string
		output    []byte
		runErr    error
		expected  int
		expectErr bool
	}{
		{name: "counts PRs", output: []byte(`[{"number": 1}, {"number": 2}, {"number": 3}]`), expected: 3},
		{name: "empty list", output: []byte(`[]`), expected: 0},
		{name: "gh error", runErr: errGHFailed, expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx, _ := stubRunGH(tt.output, tt.runErr)

			count, err := GetPRCount(ctx, "/repo", "owner/repo")
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if count != tt.expected {
				t.Errorf("expected count %d, got %d", tt.expected, count)
			}
		})
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetWorkflowRunsForCommit(t *testing.T) {
	successJSON := []byte(`[
		{
			"databaseId": 100,
			"name": "CI",
			"status": "completed",
			"conclusion": "success",
			"url": "https://github.com/owner/repo/actions/runs/100",
			"createdAt": "2026-01-02T03:04:05Z",
			"updatedAt": "2026-01-02T03:10:00Z"
		},
		{
			"databaseId": 101,
			"name": "Lint",
			"status": "in_progress",
			"conclusion": "",
			"url": "https://github.com/owner/repo/actions/runs/101",
			"createdAt": "2026-01-02T03:05:00Z",
			"updatedAt": "2026-01-02T03:06:00Z"
		},
		{
			"databaseId": 102,
			"name": "Deploy",
			"status": "completed",
			"conclusion": "failure",
			"url": "https://github.com/owner/repo/actions/runs/102",
			"createdAt": "2026-01-02T03:07:00Z",
			"updatedAt": "2026-01-02T03:08:00Z"
		},
		{
			"databaseId": 103,
			"name": "Nightly",
			"status": "queued",
			"conclusion": "",
			"url": "https://github.com/owner/repo/actions/runs/103",
			"createdAt": "2026-01-02T03:09:00Z",
			"updatedAt": "2026-01-02T03:09:00Z"
		}
	]`)

	expectedSummary := &models.WorkflowSummary{
		Runs: []models.WorkflowRun{
			{ID: 100, Name: "CI", Status: "completed", Conclusion: "success", URL: "https://github.com/owner/repo/actions/runs/100", CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), UpdatedAt: time.Date(2026, 1, 2, 3, 10, 0, 0, time.UTC)},
			{ID: 101, Name: "Lint", Status: "in_progress", URL: "https://github.com/owner/repo/actions/runs/101", CreatedAt: time.Date(2026, 1, 2, 3, 5, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 1, 2, 3, 6, 0, 0, time.UTC)},
			{ID: 102, Name: "Deploy", Status: "completed", Conclusion: "failure", URL: "https://github.com/owner/repo/actions/runs/102", CreatedAt: time.Date(2026, 1, 2, 3, 7, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 1, 2, 3, 8, 0, 0, time.UTC)},
			{ID: 103, Name: "Nightly", Status: "queued", URL: "https://github.com/owner/repo/actions/runs/103", CreatedAt: time.Date(2026, 1, 2, 3, 9, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 1, 2, 3, 9, 0, 0, time.UTC)},
		},
		Total:      4,
		Passing:    1,
		Failing:    1,
		InProgress: 2,
	}

	tests := []struct {
		name      string
		commitSHA string
		output    []byte
		runErr    error
		expected  *models.WorkflowSummary
		expectErr bool
		expectGH  bool
		cachesNil bool
	}{
		{name: "empty commit SHA short-circuits", commitSHA: ""},
		{name: "success", commitSHA: "abc123", output: successJSON, expected: expectedSummary, expectGH: true},
		{name: "gh error caches nil", commitSHA: "abc123", runErr: errGHFailed, expectErr: true, expectGH: true, cachesNil: true},
		{name: "malformed JSON", commitSHA: "abc123", output: []byte(`{`), expectErr: true, expectGH: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx, calls := stubRunGH(tt.output, tt.runErr)

			summary, err := GetWorkflowRunsForCommit(ctx, "/repo", tt.commitSHA)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(summary, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, summary)
			}

			expectedCalls := 0
			if tt.expectGH {
				expectedCalls = 1
			}
			if len(*calls) != expectedCalls {
				t.Errorf("expected %d gh invocations, got %d", expectedCalls, len(*calls))
			}

			if tt.expected != nil || tt.cachesNil {
				cachedSummary, cachedErr := GetWorkflowRunsForCommit(ctx, "/repo", tt.commitSHA)
				if cachedErr != nil {
					t.Errorf("expected cached result without error, got %v", cachedErr)
				}
				if !reflect.DeepEqual(cachedSummary, tt.expected) {
					t.Errorf("expected cached %+v, got %+v", tt.expected, cachedSummary)
				}
				if len(*calls) != expectedCalls {
					t.Errorf("expected still %d gh invocations after cache hit, got %d", expectedCalls, len(*calls))
				}
			}
		})
	}
}
