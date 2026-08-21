package github_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

var (
	errGHFailed   = errors.New("gh failed")
	errNoPRsFound = errors.New("no pull requests found")
)

// testRemoteID is the upstream identity every single-repo case caches under.
const testRemoteID = "github.com/owner/repo"

// stubRunGH returns a context that makes runGH answer with (out, err) instead
// of shelling out, plus a pointer to the recorded call args. It's local to the
// returned context, so subtests using their own stubRunGH call can run with
// t.Parallel() safely.
//
//nolint:gocritic // context.Context and *[][]string are unambiguous by type
func stubRunGH(out []byte, err error) (context.Context, *[][]string) {
	var calls [][]string
	stub := func(_ context.Context, _ string, _ []string, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return out, err
	}
	ctx := github.WithGHRunner(context.Background(), stub)

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

	tests := []getPRForBranchCase{
		{
			name:   "success",
			output: successJSON,
			expected: &forge.PullRequest{
				Number:    42,
				Title:     "Add feature",
				State:     "OPEN",
				URL:       "https://github.com/owner/repo/pull/42",
				IsDraft:   true,
				Mergeable: "CLEAN",
				HeadRef:   "feature-branch",
				BaseRef:   "main",
				Checks:    forge.ChecksStatus{Total: 3, Passing: 1, Pending: 1, Failing: 1},
			},
		},
		{
			name:      "gh error",
			runErr:    errNoPRsFound,
			expectErr: true,
		},
		{
			name:      "malformed JSON",
			output:    []byte(`{"number": "not-a-number"}`),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runGetPRForBranchCase(t, tt)
		})
	}
}

type getPRForBranchCase struct {
	name      string
	output    []byte
	runErr    error
	expected  *forge.PullRequest
	expectErr bool
}

func runGetPRForBranchCase(t *testing.T, tt getPRForBranchCase) {
	t.Helper()
	cache.ClearAll()
	ctx, calls := stubRunGH(tt.output, tt.runErr)

	pr, err := github.GetPRForBranch(ctx, "/repo", testRemoteID, "feature-branch", "owner/repo")
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

	if tt.expected == nil {
		return
	}

	cachedPR, cachedErr := github.GetPRForBranch(ctx, "/repo", testRemoteID, "feature-branch", "owner/repo")
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

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRForBranchDoesNotCacheError(t *testing.T) {
	cache.ClearAll()
	failCtx, _ := stubRunGH(nil, errGHFailed)

	if _, err := github.GetPRForBranch(failCtx, "/repo", testRemoteID, "feature-branch", "owner/repo"); err == nil {
		t.Fatal("expected error, got nil")
	}

	okCtx, okCalls := stubRunGH([]byte(`{"number": 9}`), nil)

	pr, err := github.GetPRForBranch(okCtx, "/repo", testRemoteID, "feature-branch", "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error after gh recovered: %v", err)
	}
	if pr == nil || pr.Number != 9 {
		t.Errorf("expected fresh PR #9 after gh recovered, got %+v", pr)
	}
	if len(*okCalls) != 1 {
		t.Errorf("expected gh to be re-invoked after a failure, got %d invocations", len(*okCalls))
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRForBranchArgs(t *testing.T) {
	cache.ClearAll()
	ctx, calls := stubRunGH([]byte(`{"number": 1}`), nil)

	if _, err := github.GetPRForBranch(ctx, "/repo", testRemoteID, "my-branch", "owner/repo"); err != nil {
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
		"comments": [
			{"author": {"login": "bob"}, "body": "first pass", "createdAt": "2026-01-02T10:00:00Z"},
			{"author": {"login": "dave"}, "body": "looks good now", "createdAt": "2026-01-03T05:00:00Z"}
		],
		"reviewDecision": "CHANGES_REQUESTED",
		"statusCheckRollup": [
			{
				"name": "ci",
				"workflowName": "CI",
				"status": "COMPLETED",
				"conclusion": "SUCCESS",
				"startedAt": "2026-01-03T05:00:00Z",
				"completedAt": "2026-01-03T05:01:30Z"
			},
			{"context": "codecov", "state": "PENDING"}
		]
	}`)

	expectedDetail := &forge.PRDetail{
		PullRequest: forge.PullRequest{
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
		LatestComment: &forge.PRComment{
			Author:    "dave",
			Body:      "looks good now",
			CreatedAt: time.Date(2026, 1, 3, 5, 0, 0, 0, time.UTC),
		},
		CheckDetails: []forge.CheckDetail{
			{
				Name:        "ci",
				Workflow:    "CI",
				Status:      "COMPLETED",
				Conclusion:  "SUCCESS",
				StartedAt:   time.Date(2026, 1, 3, 5, 0, 0, 0, time.UTC),
				CompletedAt: time.Date(2026, 1, 3, 5, 1, 30, 0, time.UTC),
			},
			{Status: "COMPLETED", Conclusion: "PENDING"},
		},
	}

	tests := []getPRDetailCase{
		{name: "success", output: successJSON, expected: expectedDetail},
		{name: "gh error", runErr: errGHFailed, expectErr: true},
		{name: "malformed JSON", output: []byte(`not json`), expectErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runGetPRDetailCase(t, tt)
		})
	}
}

type getPRDetailCase struct {
	name      string
	output    []byte
	runErr    error
	expected  *forge.PRDetail
	expectErr bool
}

func runGetPRDetailCase(t *testing.T, tt getPRDetailCase) {
	t.Helper()
	cache.ClearAll()
	ctx, calls := stubRunGH(tt.output, tt.runErr)

	detail, err := github.GetPRDetail(ctx, "/repo", testRemoteID, 7)
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

	cachedDetail, err := github.GetPRDetail(ctx, "/repo", testRemoteID, 7)
	if err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if !reflect.DeepEqual(cachedDetail, tt.expected) {
		t.Errorf("expected cached %+v, got %+v", tt.expected, cachedDetail)
	}
	if len(*calls) != 1 {
		t.Errorf("expected 1 gh invocation, got %d", len(*calls))
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
			"reviewDecision": "APPROVED",
			"reviewRequests": [{"login": "erin"}]
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
		expected  []forge.PullRequest
		expectErr bool
		expectGH  bool
	}{
		{
			name:     "empty upstream short-circuits",
			upstream: "",
			expected: []forge.PullRequest{},
		},
		{
			name:     "success",
			upstream: "owner/repo",
			output:   successJSON,
			expected: []forge.PullRequest{
				{
					Number: 2, Title: "Second", State: "OPEN", URL: "https://github.com/owner/repo/pull/2",
					IsDraft: true, HeadRef: "two", BaseRef: "main",
				},
				{
					Number: 1, Title: "First", State: "OPEN", URL: "https://github.com/owner/repo/pull/1",
					HeadRef: "one", BaseRef: "main", ReviewDecision: "APPROVED", Reviewers: []string{"erin"},
				},
			},
			expectGH: true,
		},
		{
			name:      "gh error returns empty list",
			upstream:  "owner/repo",
			runErr:    errGHFailed,
			expected:  []forge.PullRequest{},
			expectErr: true,
			expectGH:  true,
		},
		{
			name:      "malformed JSON returns empty list",
			upstream:  "owner/repo",
			output:    []byte(`{"not": "an array"}`),
			expected:  []forge.PullRequest{},
			expectErr: true,
			expectGH:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx, calls := stubRunGH(tt.output, tt.runErr)

			prs, err := github.GetPRsForRepo(ctx, "/repo", testRemoteID, tt.upstream)
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
				expectedCalls = github.PRListPages
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

	first, err := github.GetPRsForRepo(ctx, "/repo", testRemoteID, "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := github.GetPRsForRepo(ctx, "/repo", testRemoteID, "owner/repo")
	if err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("expected cached result %+v, got %+v", first, second)
	}
	if len(*calls) != github.PRListPages {
		t.Errorf("expected %d gh invocations, got %d", github.PRListPages, len(*calls))
	}
}

// Pull request data belongs to the remote, so two checkouts of one remote read
// a single entry however each was cloned, while a checkout with no remote
// stays scoped to its own directory. The upstream ref name cannot separate
// them on its own, since nearly every repo tracks an "origin/..." ref.
//
//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestPRCachesAreScopedByRemote(t *testing.T) {
	tests := []struct {
		name       string
		urlA       string
		urlB       string
		wantShared bool
	}{
		{
			name: "ssh and https clones of one remote share",
			urlA: "git@github.com:Acme/App.git", urlB: "https://github.com/acme/app",
			wantShared: true,
		},
		{
			name: "a repo of the same name on another host does not",
			urlA: "git@github.com:acme/app.git", urlB: "git@github.acme-corp.com:acme/app.git",
		},
		{
			name: "checkouts with no remote stay per-directory",
			urlA: "", urlB: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			idA, idB := vcs.RemoteIdentity(tt.urlA), vcs.RemoteIdentity(tt.urlB)

			ctxA, _ := stubRunGH([]byte(`[{"number": 1, "title": "Only in A"}]`), nil)
			if _, err := github.GetPRsForRepo(ctxA, "/repo-a", idA, "origin/main"); err != nil {
				t.Fatalf("unexpected error for checkout A: %v", err)
			}

			ctxB, callsB := stubRunGH([]byte(`[]`), nil)
			prsB, err := github.GetPRsForRepo(ctxB, "/repo-b", idB, "origin/main")
			if err != nil {
				t.Fatalf("unexpected error for checkout B: %v", err)
			}

			wantPRs, wantCalls := 0, github.PRListPages
			if tt.wantShared {
				wantPRs, wantCalls = 1, 0
			}
			if len(prsB) != wantPRs {
				t.Errorf("checkout B read %d PRs, want %d", len(prsB), wantPRs)
			}
			if len(*callsB) != wantCalls {
				t.Errorf("checkout B invoked gh %d times, want %d", len(*callsB), wantCalls)
			}

			keyA := github.PRCacheKey("/repo-a", idA, "origin/main", "main")
			cache.PRCache.Set(keyA, cache.NoStamp, &forge.PullRequest{Number: 1})
			if _, hit := github.CachedPRForBranch("/repo-b", idB, "main", "origin/main"); hit != tt.wantShared {
				t.Errorf("per-branch cache hit from checkout B = %v, want %v", hit, tt.wantShared)
			}
		})
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

			count, err := github.GetPRCount(ctx, "/repo", testRemoteID, "owner/repo")
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
func TestGetMergedPRHeads(t *testing.T) {
	tests := []struct {
		name      string
		output    []byte
		runErr    error
		expected  map[string]string
		expectErr bool
	}{
		{
			name: "success",
			output: []byte(`[
				{"headRefName": "feature-a", "headRefOid": "aaa111"},
				{"headRefName": "feature-b", "headRefOid": "bbb222"}
			]`),
			expected: map[string]string{"feature-a": "aaa111", "feature-b": "bbb222"},
		},
		{name: "empty list", output: []byte(`[]`), expected: map[string]string{}},
		{name: "gh error", runErr: errGHFailed, expected: map[string]string{}, expectErr: true},
		{
			name:      "malformed JSON",
			output:    []byte(`{"not": "an array"}`),
			expected:  map[string]string{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx, _ := stubRunGH(tt.output, tt.runErr)

			heads, err := github.GetMergedPRHeads(ctx, "/repo", testRemoteID)
			if tt.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(heads, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, heads)
			}
		})
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetMergedPRHeadsArgs(t *testing.T) {
	cache.ClearAll()
	ctx, calls := stubRunGH([]byte(`[]`), nil)

	if _, err := github.GetMergedPRHeads(ctx, "/repo", testRemoteID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"pr", "list", "--state", "merged", "--json", "headRefName,headRefOid", "--limit", "100"}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0], want) {
		t.Errorf("expected gh args %v, got %v", want, *calls)
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetMergedPRHeadsUsesCache(t *testing.T) {
	cache.ClearAll()
	ctx, calls := stubRunGH([]byte(`[{"headRefName": "x", "headRefOid": "y"}]`), nil)

	first, err := github.GetMergedPRHeads(ctx, "/repo", testRemoteID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, err := github.GetMergedPRHeads(ctx, "/repo", testRemoteID)
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

	expectedSummary := &forge.WorkflowSummary{
		Runs: []forge.WorkflowRun{
			{
				ID: 100, Name: "CI", Status: "completed", Conclusion: "success",
				URL:       "https://github.com/owner/repo/actions/runs/100",
				CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
				UpdatedAt: time.Date(2026, 1, 2, 3, 10, 0, 0, time.UTC),
			},
			{
				ID: 101, Name: "Lint", Status: "in_progress",
				URL:       "https://github.com/owner/repo/actions/runs/101",
				CreatedAt: time.Date(2026, 1, 2, 3, 5, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 1, 2, 3, 6, 0, 0, time.UTC),
			},
			{
				ID: 102, Name: "Deploy", Status: "completed", Conclusion: "failure",
				URL:       "https://github.com/owner/repo/actions/runs/102",
				CreatedAt: time.Date(2026, 1, 2, 3, 7, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 1, 2, 3, 8, 0, 0, time.UTC),
			},
			{
				ID: 103, Name: "Nightly", Status: "queued",
				URL:       "https://github.com/owner/repo/actions/runs/103",
				CreatedAt: time.Date(2026, 1, 2, 3, 9, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 1, 2, 3, 9, 0, 0, time.UTC),
			},
		},
		Total:      4,
		Passing:    1,
		Failing:    1,
		InProgress: 2,
	}

	tests := []getWorkflowRunsCase{
		{name: "empty commit SHA short-circuits", commitSHA: ""},
		{name: "success", commitSHA: "abc123", output: successJSON, expected: expectedSummary, expectGH: true},
		{name: "gh error", commitSHA: "abc123", runErr: errGHFailed, expectErr: true, expectGH: true},
		{name: "malformed JSON", commitSHA: "abc123", output: []byte(`{`), expectErr: true, expectGH: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runGetWorkflowRunsCase(t, &tt)
		})
	}
}

type getWorkflowRunsCase struct {
	name      string
	commitSHA string
	output    []byte
	runErr    error
	expected  *forge.WorkflowSummary
	expectErr bool
	expectGH  bool
}

func runGetWorkflowRunsCase(t *testing.T, tt *getWorkflowRunsCase) {
	t.Helper()
	cache.ClearAll()
	ctx, calls := stubRunGH(tt.output, tt.runErr)

	summary, err := github.GetWorkflowRunsForCommit(ctx, "/repo", testRemoteID, tt.commitSHA)
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

	if tt.expected == nil {
		return
	}

	cachedSummary, cachedErr := github.GetWorkflowRunsForCommit(ctx, "/repo", testRemoteID, tt.commitSHA)
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

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetWorkflowRunsForCommitDoesNotCacheError(t *testing.T) {
	cache.ClearAll()
	failCtx, _ := stubRunGH(nil, errGHFailed)

	if _, err := github.GetWorkflowRunsForCommit(failCtx, "/repo", testRemoteID, "abc123"); err == nil {
		t.Fatal("expected error, got nil")
	}

	okJSON := `[{"databaseId": 7, "name": "CI", "status": "completed", "conclusion": "success"}]`
	okCtx, okCalls := stubRunGH([]byte(okJSON), nil)

	summary, err := github.GetWorkflowRunsForCommit(okCtx, "/repo", testRemoteID, "abc123")
	if err != nil {
		t.Fatalf("unexpected error after gh recovered: %v", err)
	}
	if summary == nil || summary.Total != 1 || summary.Passing != 1 {
		t.Errorf("expected fresh summary with 1 passing run after gh recovered, got %+v", summary)
	}
	if len(*okCalls) != 1 {
		t.Errorf("expected gh to be re-invoked after a failure, got %d invocations", len(*okCalls))
	}
}

// stubRunGHByFilter answers the operator's own page and everyone else's page
// separately, so a test can tell which of the two a pull request came from.
func stubRunGHByFilter(mine, others []byte, mineErr, othersErr error) context.Context {
	stub := func(_ context.Context, _ string, _ []string, args ...string) ([]byte, error) {
		if slices.Contains(args, "--author") {
			return mine, mineErr
		}

		return others, othersErr
	}

	return github.WithGHRunner(context.Background(), stub)
}

// The list is read as two pages because one page of 100 times out on a repo
// with enough open pull requests. A repo whose pull requests all belong to a
// bot has nothing on the operator's page, and the operator's older work has
// fallen off everyone else's, so the panel needs both.
//
//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRsForRepoMergesBothPages(t *testing.T) {
	tests := []struct {
		name      string
		mine      []byte
		others    []byte
		mineErr   error
		othersErr error
		want      []int
		expectErr bool
	}{
		{
			name:   "both pages contribute, newest first",
			mine:   []byte(`[{"number": 3, "title": "Mine"}]`),
			others: []byte(`[{"number": 9, "title": "Bot"}]`),
			want:   []int{9, 3},
		},
		{
			name:   "a pull request on both pages is listed once",
			mine:   []byte(`[{"number": 5, "title": "Mine"}]`),
			others: []byte(`[{"number": 5, "title": "Mine"}, {"number": 4, "title": "Theirs"}]`),
			want:   []int{5, 4},
		},
		{
			name:   "a bot-only repo still lists its pull requests",
			mine:   []byte(`[]`),
			others: []byte(`[{"number": 10, "title": "Bump"}, {"number": 9, "title": "Bump"}]`),
			want:   []int{10, 9},
		},
		{
			name:    "one page failing still returns the other",
			mineErr: errGHFailed,
			others:  []byte(`[{"number": 2, "title": "Theirs"}]`),
			want:    []int{2},
		},
		{
			name:      "both pages failing reports the error",
			mineErr:   errGHFailed,
			othersErr: errGHFailed,
			want:      []int{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			ctx := stubRunGHByFilter(tt.mine, tt.others, tt.mineErr, tt.othersErr)

			prs, err := github.GetPRsForRepo(ctx, "/repo", testRemoteID, "owner/repo")
			if gotErr := err != nil; gotErr != tt.expectErr {
				t.Fatalf("error = %v, want an error: %v", err, tt.expectErr)
			}

			got := make([]int, 0, len(prs))
			for _, pr := range prs {
				got = append(got, pr.Number)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pull requests %v, want %v", got, tt.want)
			}
		})
	}
}

// A failed fetch must not be cached: an empty list would read as "this repo has
// no pull requests" for the whole cache window and hide the panel.
//
//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetPRsForRepoDoesNotCacheAFailure(t *testing.T) {
	cache.ClearAll()

	ctx := stubRunGHByFilter(nil, nil, errGHFailed, errGHFailed)
	if _, err := github.GetPRsForRepo(ctx, "/repo", testRemoteID, "owner/repo"); err == nil {
		t.Fatal("expected the failed fetch to report an error")
	}

	if _, cached := github.CachedPRs("/repo", testRemoteID, "owner/repo"); cached {
		t.Error("a failed fetch cached a result, so a later read would report no pull requests")
	}
}
