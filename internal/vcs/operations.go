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

// defaultMainBranch, masterBranch, and trunkBranch are the conventional
// primary branch names assumed when scanning for merged branches to clean up.
const (
	defaultMainBranch = "main"
	masterBranch      = "master"
	trunkBranch       = "trunk"
)

// IsDefaultBranchName reports whether name is one of the conventional
// default-branch names (main, master, trunk) that cleanup should never treat
// as a feature branch.
func IsDefaultBranchName(name string) bool {
	return name == defaultMainBranch || name == masterBranch || name == trunkBranch
}

// StatusReader answers summary-level queries about a repository's current state.
type StatusReader interface {
	CompareBranches(ctx context.Context, repoPath, branch, target string) (ahead, behind int, err error)
	GetAheadBehind(ctx context.Context, repoPath, branch, upstream string) (ahead, behind int, err error)
	GetCurrentBranch(ctx context.Context, repoPath string) (string, error)
	GetLastModified(ctx context.Context, repoPath string) (int64, error)
	GetRemoteURL(ctx context.Context, repoPath string) (string, error)
	GetRepoSummary(ctx context.Context, repoPath string) (models.RepoSummary, error)
	GetUpstream(ctx context.Context, repoPath, branch string) (string, error)
	VCSType() models.VCSType
}

// DetailReader answers drill-down queries about a repository's branches, stashes,
// worktrees, and commit history.
type DetailReader interface {
	GetBranchList(ctx context.Context, repoPath string) ([]models.BranchInfo, error)
	GetCommitLog(ctx context.Context, repoPath string, count int) ([]models.CommitInfo, error)
	GetStashList(ctx context.Context, repoPath string) ([]models.StashDetail, error)
	GetWorktreeList(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error)
}

// Mutator performs write operations against a repository. Each method returns
// (success, message) alongside an error so callers can surface per-repo feedback
// in the UI even when the operation itself didn't error.
type Mutator interface {
	// CleanupMergedBranches deletes local branches fully merged into the
	// default branch, plus any names in squashMerged: branches the caller has
	// already verified (via merged PR head OIDs) as squash-merged, which
	// `git branch --merged`/`jj bookmark` can't detect on their own.
	CleanupMergedBranches(ctx context.Context, repoPath string, squashMerged []string) (bool, string, error)
	FetchAll(ctx context.Context, repoPath string) (bool, string, error)
	PruneRemote(ctx context.Context, repoPath string) (bool, string, error)
}

// Operations abstracts the git/jj commands used to inspect and mutate a repository.
type Operations interface {
	StatusReader
	DetailReader
	Mutator
}
