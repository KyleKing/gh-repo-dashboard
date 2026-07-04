package vcs

import (
	"context"
	"errors"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// ErrCommandFailed wraps a non-zero exit from the underlying git/jj CLI.
var ErrCommandFailed = errors.New("command failed")

// ErrUnexpectedOutput wraps a CLI output that didn't match the expected format.
var ErrUnexpectedOutput = errors.New("unexpected command output")

// defaultMainBranch and masterBranch are the conventional primary branch
// names assumed when scanning for merged branches to clean up.
const (
	defaultMainBranch = "main"
	masterBranch      = "master"
)

type Operations interface {
	GetRepoSummary(ctx context.Context, repoPath string) (models.RepoSummary, error)
	GetCurrentBranch(ctx context.Context, repoPath string) (string, error)
	GetUpstream(ctx context.Context, repoPath, branch string) (string, error)
	GetAheadBehind(ctx context.Context, repoPath, branch, upstream string) (ahead, behind int, err error)
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
