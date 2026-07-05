package batch_test

import (
	"context"
	"errors"
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
	fetchResult   func() (bool, string, error)
	pruneResult   func() (bool, string, error)
	cleanupResult func() (bool, string, error)
}

func (*mockVCS) GetRepoSummary(_ context.Context, _ string) (models.RepoSummary, error) {
	return models.RepoSummary{}, nil
}

func (*mockVCS) GetCurrentBranch(_ context.Context, _ string) (string, error) {
	return "main", nil
}

func (*mockVCS) GetUpstream(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

//nolint:gocritic // matches vcs.Operations.GetAheadBehind's (ahead, behind int, err error)
func (*mockVCS) GetAheadBehind(_ context.Context, _, _, _ string) (int, int, error) {
	return 0, 0, nil
}

//nolint:gocritic // matches vcs.Operations.CompareBranches's (ahead, behind int, err error)
func (*mockVCS) CompareBranches(_ context.Context, _, _, _ string) (int, int, error) {
	return 0, 0, nil
}

func (*mockVCS) GetStagedCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (*mockVCS) GetUnstagedCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (*mockVCS) GetUntrackedCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (*mockVCS) GetConflictedCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (*mockVCS) GetBranchList(_ context.Context, _ string) ([]models.BranchInfo, error) {
	return nil, nil
}

func (*mockVCS) GetStashList(_ context.Context, _ string) ([]models.StashDetail, error) {
	return nil, nil
}

func (*mockVCS) GetWorktreeList(_ context.Context, _ string) ([]models.WorktreeInfo, error) {
	return nil, nil
}

func (*mockVCS) GetCommitLog(_ context.Context, _ string, _ int) ([]models.CommitInfo, error) {
	return nil, nil
}

func (*mockVCS) GetLastModified(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (*mockVCS) GetRemoteURL(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (*mockVCS) VCSType() models.VCSType {
	return models.VCSTypeGit
}

//nolint:gocritic // matches vcs.Operations.FetchAll's (ok bool, msg string, err error)
func (m *mockVCS) FetchAll(_ context.Context, _ string) (bool, string, error) {
	if m.fetchResult != nil {
		return m.fetchResult()
	}

	return true, testSuccessMsg, nil
}

//nolint:gocritic // matches vcs.Operations.PruneRemote's (ok bool, msg string, err error)
func (m *mockVCS) PruneRemote(_ context.Context, _ string) (bool, string, error) {
	if m.pruneResult != nil {
		return m.pruneResult()
	}

	return true, testSuccessMsg, nil
}

//nolint:gocritic // matches vcs.Operations.CleanupMergedBranches's (ok bool, msg string, err error)
func (m *mockVCS) CleanupMergedBranches(_ context.Context, _ string) (bool, string, error) {
	if m.cleanupResult != nil {
		return m.cleanupResult()
	}

	return true, testSuccessMsg, nil
}

var _ vcs.Operations = (*mockVCS)(nil)

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

func TestCleanupMerged(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			mock := &mockVCS{cleanupResult: tt.result}
			ctx := context.Background()
			_, msg, err := batch.CleanupMerged(ctx, mock, "/repo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if msg != tt.wantMsg {
				t.Errorf("expected msg=%q, got %q", tt.wantMsg, msg)
			}
		})
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
