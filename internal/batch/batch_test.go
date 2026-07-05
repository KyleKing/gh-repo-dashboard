package batch_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

var errNetwork = errors.New("network error")

const testSuccessMsg = "success"

func TestRepoName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		expected string
	}{
		{"/home/user/projects/my-repo", "my-repo"},
		{"/repo", "repo"},
		{"repo", "repo"},
		{"/a/b/c/d", "d"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			result := batch.RepoName(tt.path)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

type mockVCS struct {
	fetchResult     func() (bool, string, error)
	pruneResult     func() (bool, string, error)
	cleanupResult   func() (bool, string, error)
	gotSquashMerged []string
	sawSquashMerged bool
}

//nolint:gocritic // matches vcs.Mutator.FetchAll's (ok bool, msg string, err error)
func (m *mockVCS) FetchAll(_ context.Context, _ string) (bool, string, error) {
	if m.fetchResult != nil {
		return m.fetchResult()
	}

	return true, testSuccessMsg, nil
}

//nolint:gocritic // matches vcs.Mutator.PruneRemote's (ok bool, msg string, err error)
func (m *mockVCS) PruneRemote(_ context.Context, _ string) (bool, string, error) {
	if m.pruneResult != nil {
		return m.pruneResult()
	}

	return true, testSuccessMsg, nil
}

//nolint:gocritic // matches vcs.Mutator.CleanupMergedBranches's (ok bool, msg string, err error)
func (m *mockVCS) CleanupMergedBranches(_ context.Context, _ string, squashMerged []string) (bool, string, error) {
	m.gotSquashMerged = squashMerged
	m.sawSquashMerged = true
	if m.cleanupResult != nil {
		return m.cleanupResult()
	}

	return true, testSuccessMsg, nil
}

var _ vcs.Mutator = (*mockVCS)(nil)

func TestFetchAll(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		result      func() (bool, string, error)
		wantSuccess bool
		wantErr     bool
	}{
		{
			name:        testSuccessMsg,
			result:      func() (bool, string, error) { return true, "ok", nil },
			wantSuccess: true,
			wantErr:     false,
		},
		{
			name:        "failure returns false",
			result:      func() (bool, string, error) { return false, "failed", nil },
			wantSuccess: false,
			wantErr:     false,
		},
		{
			name:        "error propagates",
			result:      func() (bool, string, error) { return false, "", errNetwork },
			wantSuccess: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockVCS{fetchResult: tt.result}
			ctx := context.Background()
			success, _, err := batch.FetchAll(ctx, mock, "/repo")

			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if success != tt.wantSuccess {
				t.Errorf("expected success=%v, got %v", tt.wantSuccess, success)
			}
		})
	}
}

func TestPruneRemote(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		result      func() (bool, string, error)
		wantSuccess bool
	}{
		{
			name:        testSuccessMsg,
			result:      func() (bool, string, error) { return true, "pruned", nil },
			wantSuccess: true,
		},
		{
			name:        "failure",
			result:      func() (bool, string, error) { return false, "no remote", nil },
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mock := &mockVCS{pruneResult: tt.result}
			ctx := context.Background()
			success, _, err := batch.PruneRemote(ctx, mock, "/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if success != tt.wantSuccess {
				t.Errorf("expected success=%v, got %v", tt.wantSuccess, success)
			}
		})
	}
}

// stubMergedPRHeads and stubOperations swap batch's getMergedPRHeads/
// getOperations seams (internal/batch/export_test.go) for the test's
// duration, avoiding a real gh/git/jj invocation; t.Cleanup restores the originals.
func stubMergedPRHeads(t *testing.T, heads map[string]string) {
	t.Helper()

	restore := batch.SetGetMergedPRHeadsForTest(
		func(context.Context, string) (map[string]string, error) { return heads, nil },
	)
	t.Cleanup(restore)
}

func stubOperations(t *testing.T, branches []models.BranchInfo) {
	t.Helper()

	restore := batch.SetGetOperationsForTest(func(string) vcs.Operations {
		return &vcs.MockOperations{
			GetBranchListFn: func(context.Context, string) ([]models.BranchInfo, error) {
				return branches, nil
			},
		}
	})
	t.Cleanup(restore)
}

//nolint:paralleltest // stubs shared batch package-level seam state
func TestCleanupMerged(t *testing.T) {
	tests := []struct {
		name    string
		result  func() (bool, string, error)
		wantMsg string
	}{
		{
			name:    "deleted branches",
			result:  func() (bool, string, error) { return true, "Deleted 2 branches", nil },
			wantMsg: "Deleted 2 branches",
		},
		{
			name:    "no branches to delete",
			result:  func() (bool, string, error) { return true, "No merged branches to delete", nil },
			wantMsg: "No merged branches to delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubMergedPRHeads(t, nil)

			mock := &mockVCS{cleanupResult: tt.result}
			_, msg, err := batch.CleanupMerged(context.Background(), mock, "/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg != tt.wantMsg {
				t.Errorf("expected msg=%q, got %q", tt.wantMsg, msg)
			}
			if !mock.sawSquashMerged {
				t.Error("expected CleanupMergedBranches to be called")
			}
			if len(mock.gotSquashMerged) != 0 {
				t.Errorf("expected no squash-merged branches, got %v", mock.gotSquashMerged)
			}
		})
	}
}

//nolint:paralleltest // stubs shared batch package-level seam state
func TestCleanupMergedDetectsSquashMerged(t *testing.T) {
	stubMergedPRHeads(t, map[string]string{"feature-a": "abc123"})
	stubOperations(t, []models.BranchInfo{
		{Name: "feature-a", Head: "abc123"},
		{Name: "feature-b", Head: "zzz999"},
	})

	mock := &mockVCS{}
	if _, _, err := batch.CleanupMerged(context.Background(), mock, "/repo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(mock.gotSquashMerged, []string{"feature-a"}) {
		t.Errorf("expected squash-merged [feature-a], got %v", mock.gotSquashMerged)
	}
}

//nolint:paralleltest // stubs shared batch package-level seam state
func TestPreviewCleanupReportsWouldDelete(t *testing.T) {
	stubMergedPRHeads(t, map[string]string{"feature-a": "abc123"})
	stubOperations(t, []models.BranchInfo{
		{Name: "feature-a", Head: "abc123"},
		{Name: "feature-b", Head: "zzz999"},
	})

	ok, msg, err := batch.PreviewCleanup(context.Background(), &mockVCS{}, "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if msg != "Would delete 1 branches: feature-a" {
		t.Errorf("unexpected message: %q", msg)
	}
}

//nolint:paralleltest // stubs shared batch package-level seam state
func TestPreviewCleanupNoCandidates(t *testing.T) {
	stubMergedPRHeads(t, nil)
	stubOperations(t, nil)

	ok, msg, err := batch.PreviewCleanup(context.Background(), &mockVCS{}, "/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected ok=true")
	}
	if msg != "No merged branches to delete" {
		t.Errorf("unexpected message: %q", msg)
	}
}

func TestTaskResultTracksRepoName(t *testing.T) {
	t.Parallel()
	result := batch.TaskResult{
		RepoName: batch.RepoName("/home/user/projects/my-app"),
	}

	if result.RepoName != "my-app" {
		t.Errorf("expected RepoName='my-app', got %q", result.RepoName)
	}
}
