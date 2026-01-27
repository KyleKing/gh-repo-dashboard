package batch

import (
	"context"
	"errors"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func TestRepoName(t *testing.T) {
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
			result := repoName(tt.path)
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

func (m *mockVCS) GetRepoSummary(ctx context.Context, repoPath string) (models.RepoSummary, error) {
	return models.RepoSummary{}, nil
}
func (m *mockVCS) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	return "main", nil
}
func (m *mockVCS) GetUpstream(ctx context.Context, repoPath string, branch string) (string, error) {
	return "", nil
}
func (m *mockVCS) GetAheadBehind(ctx context.Context, repoPath string, branch string, upstream string) (int, int, error) {
	return 0, 0, nil
}
func (m *mockVCS) GetStagedCount(ctx context.Context, repoPath string) (int, error) {
	return 0, nil
}
func (m *mockVCS) GetUnstagedCount(ctx context.Context, repoPath string) (int, error) {
	return 0, nil
}
func (m *mockVCS) GetUntrackedCount(ctx context.Context, repoPath string) (int, error) {
	return 0, nil
}
func (m *mockVCS) GetConflictedCount(ctx context.Context, repoPath string) (int, error) {
	return 0, nil
}
func (m *mockVCS) GetBranchList(ctx context.Context, repoPath string) ([]models.BranchInfo, error) {
	return nil, nil
}
func (m *mockVCS) GetStashList(ctx context.Context, repoPath string) ([]models.StashDetail, error) {
	return nil, nil
}
func (m *mockVCS) GetWorktreeList(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error) {
	return nil, nil
}
func (m *mockVCS) GetCommitLog(ctx context.Context, repoPath string, count int) ([]models.CommitInfo, error) {
	return nil, nil
}
func (m *mockVCS) GetLastModified(ctx context.Context, repoPath string) (int64, error) {
	return 0, nil
}
func (m *mockVCS) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	return "", nil
}
func (m *mockVCS) VCSType() models.VCSType {
	return models.VCSTypeGit
}
func (m *mockVCS) FetchAll(ctx context.Context, repoPath string) (bool, string, error) {
	if m.fetchResult != nil {
		return m.fetchResult()
	}
	return true, "success", nil
}
func (m *mockVCS) PruneRemote(ctx context.Context, repoPath string) (bool, string, error) {
	if m.pruneResult != nil {
		return m.pruneResult()
	}
	return true, "success", nil
}
func (m *mockVCS) CleanupMergedBranches(ctx context.Context, repoPath string) (bool, string, error) {
	if m.cleanupResult != nil {
		return m.cleanupResult()
	}
	return true, "success", nil
}

var _ vcs.Operations = (*mockVCS)(nil)

func TestFetchAll(t *testing.T) {
	tests := []struct {
		name        string
		result      func() (bool, string, error)
		wantSuccess bool
		wantErr     bool
	}{
		{
			name:        "success",
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
			result:      func() (bool, string, error) { return false, "", errors.New("network error") },
			wantSuccess: false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockVCS{fetchResult: tt.result}
			ctx := context.Background()
			success, _, err := FetchAll(ctx, mock, "/repo")

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
	tests := []struct {
		name        string
		result      func() (bool, string, error)
		wantSuccess bool
	}{
		{
			name:        "success",
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
			mock := &mockVCS{pruneResult: tt.result}
			ctx := context.Background()
			success, _, _ := PruneRemote(ctx, mock, "/repo")
			if success != tt.wantSuccess {
				t.Errorf("expected success=%v, got %v", tt.wantSuccess, success)
			}
		})
	}
}

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
			mock := &mockVCS{cleanupResult: tt.result}
			ctx := context.Background()
			_, msg, _ := CleanupMerged(ctx, mock, "/repo")
			if msg != tt.wantMsg {
				t.Errorf("expected msg=%q, got %q", tt.wantMsg, msg)
			}
		})
	}
}

func TestTaskResultTracksRepoName(t *testing.T) {
	result := TaskResult{
		Path:     "/home/user/projects/my-app",
		RepoName: repoName("/home/user/projects/my-app"),
		Success:  true,
		Message:  "done",
	}

	if result.RepoName != "my-app" {
		t.Errorf("expected RepoName='my-app', got %q", result.RepoName)
	}
}
