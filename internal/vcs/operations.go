package vcs

import (
	"context"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type Operations interface {
	GetRepoSummary(ctx context.Context, repoPath string) (models.RepoSummary, error)
	GetCurrentBranch(ctx context.Context, repoPath string) (string, error)
	GetUpstream(ctx context.Context, repoPath string, branch string) (string, error)
	GetAheadBehind(ctx context.Context, repoPath string, branch string, upstream string) (ahead int, behind int, err error)
	GetStagedCount(ctx context.Context, repoPath string) (int, error)
	GetUnstagedCount(ctx context.Context, repoPath string) (int, error)
	GetUntrackedCount(ctx context.Context, repoPath string) (int, error)
	GetConflictedCount(ctx context.Context, repoPath string) (int, error)
	GetBranchList(ctx context.Context, repoPath string) ([]models.BranchInfo, error)
	GetStashList(ctx context.Context, repoPath string) ([]models.StashDetail, error)
	GetWorktreeList(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error)
	GetCommitLog(ctx context.Context, repoPath string, count int) ([]models.CommitInfo, error)
	GetLastModified(ctx context.Context, repoPath string) (int64, error)
	GetRemoteURL(ctx context.Context, repoPath string) (string, error)
	VCSType() models.VCSType

	FetchAll(ctx context.Context, repoPath string) (bool, string, error)
	PruneRemote(ctx context.Context, repoPath string) (bool, string, error)
	CleanupMergedBranches(ctx context.Context, repoPath string) (bool, string, error)
}
