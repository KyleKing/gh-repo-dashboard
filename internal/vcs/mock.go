package vcs

import (
	"context"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type MockOperations struct {
	GetRepoSummaryFn        func(ctx context.Context, repoPath string) (models.RepoSummary, error)
	GetCurrentBranchFn      func(ctx context.Context, repoPath string) (string, error)
	GetUpstreamFn           func(ctx context.Context, repoPath string, branch string) (string, error)
	GetAheadBehindFn        func(ctx context.Context, repoPath string, branch string, upstream string) (int, int, error)
	GetStagedCountFn        func(ctx context.Context, repoPath string) (int, error)
	GetUnstagedCountFn      func(ctx context.Context, repoPath string) (int, error)
	GetUntrackedCountFn     func(ctx context.Context, repoPath string) (int, error)
	GetConflictedCountFn    func(ctx context.Context, repoPath string) (int, error)
	GetBranchListFn         func(ctx context.Context, repoPath string) ([]models.BranchInfo, error)
	GetStashListFn          func(ctx context.Context, repoPath string) ([]models.StashDetail, error)
	GetWorktreeListFn       func(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error)
	GetCommitLogFn          func(ctx context.Context, repoPath string, count int) ([]models.CommitInfo, error)
	GetLastModifiedFn       func(ctx context.Context, repoPath string) (int64, error)
	GetRemoteURLFn          func(ctx context.Context, repoPath string) (string, error)
	VCSTypeFn               func() models.VCSType
	FetchAllFn              func(ctx context.Context, repoPath string) (bool, string, error)
	PruneRemoteFn           func(ctx context.Context, repoPath string) (bool, string, error)
	CleanupMergedBranchesFn func(ctx context.Context, repoPath string) (bool, string, error)
}

func (m *MockOperations) GetRepoSummary(ctx context.Context, repoPath string) (models.RepoSummary, error) {
	if m.GetRepoSummaryFn != nil {
		return m.GetRepoSummaryFn(ctx, repoPath)
	}
	return models.RepoSummary{Path: repoPath}, nil
}

func (m *MockOperations) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	if m.GetCurrentBranchFn != nil {
		return m.GetCurrentBranchFn(ctx, repoPath)
	}
	return "main", nil
}

func (m *MockOperations) GetUpstream(ctx context.Context, repoPath string, branch string) (string, error) {
	if m.GetUpstreamFn != nil {
		return m.GetUpstreamFn(ctx, repoPath, branch)
	}
	return "", nil
}

func (m *MockOperations) GetAheadBehind(ctx context.Context, repoPath string, branch string, upstream string) (int, int, error) {
	if m.GetAheadBehindFn != nil {
		return m.GetAheadBehindFn(ctx, repoPath, branch, upstream)
	}
	return 0, 0, nil
}

func (m *MockOperations) GetStagedCount(ctx context.Context, repoPath string) (int, error) {
	if m.GetStagedCountFn != nil {
		return m.GetStagedCountFn(ctx, repoPath)
	}
	return 0, nil
}

func (m *MockOperations) GetUnstagedCount(ctx context.Context, repoPath string) (int, error) {
	if m.GetUnstagedCountFn != nil {
		return m.GetUnstagedCountFn(ctx, repoPath)
	}
	return 0, nil
}

func (m *MockOperations) GetUntrackedCount(ctx context.Context, repoPath string) (int, error) {
	if m.GetUntrackedCountFn != nil {
		return m.GetUntrackedCountFn(ctx, repoPath)
	}
	return 0, nil
}

func (m *MockOperations) GetConflictedCount(ctx context.Context, repoPath string) (int, error) {
	if m.GetConflictedCountFn != nil {
		return m.GetConflictedCountFn(ctx, repoPath)
	}
	return 0, nil
}

func (m *MockOperations) GetBranchList(ctx context.Context, repoPath string) ([]models.BranchInfo, error) {
	if m.GetBranchListFn != nil {
		return m.GetBranchListFn(ctx, repoPath)
	}
	return nil, nil
}

func (m *MockOperations) GetStashList(ctx context.Context, repoPath string) ([]models.StashDetail, error) {
	if m.GetStashListFn != nil {
		return m.GetStashListFn(ctx, repoPath)
	}
	return nil, nil
}

func (m *MockOperations) GetWorktreeList(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error) {
	if m.GetWorktreeListFn != nil {
		return m.GetWorktreeListFn(ctx, repoPath)
	}
	return nil, nil
}

func (m *MockOperations) GetCommitLog(ctx context.Context, repoPath string, count int) ([]models.CommitInfo, error) {
	if m.GetCommitLogFn != nil {
		return m.GetCommitLogFn(ctx, repoPath, count)
	}
	return nil, nil
}

func (m *MockOperations) GetLastModified(ctx context.Context, repoPath string) (int64, error) {
	if m.GetLastModifiedFn != nil {
		return m.GetLastModifiedFn(ctx, repoPath)
	}
	return 0, nil
}

func (m *MockOperations) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	if m.GetRemoteURLFn != nil {
		return m.GetRemoteURLFn(ctx, repoPath)
	}
	return "", nil
}

func (m *MockOperations) VCSType() models.VCSType {
	if m.VCSTypeFn != nil {
		return m.VCSTypeFn()
	}
	return models.VCSTypeGit
}

func (m *MockOperations) FetchAll(ctx context.Context, repoPath string) (bool, string, error) {
	if m.FetchAllFn != nil {
		return m.FetchAllFn(ctx, repoPath)
	}
	return true, "Fetched", nil
}

func (m *MockOperations) PruneRemote(ctx context.Context, repoPath string) (bool, string, error) {
	if m.PruneRemoteFn != nil {
		return m.PruneRemoteFn(ctx, repoPath)
	}
	return true, "Pruned", nil
}

func (m *MockOperations) CleanupMergedBranches(ctx context.Context, repoPath string) (bool, string, error) {
	if m.CleanupMergedBranchesFn != nil {
		return m.CleanupMergedBranchesFn(ctx, repoPath)
	}
	return true, "No branches to cleanup", nil
}

var _ Operations = (*MockOperations)(nil)
