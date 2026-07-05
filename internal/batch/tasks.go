package batch

import (
	"context"
	"fmt"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// FetchAll is a batch.TaskFunc that fetches all remotes for a repo.
//
//nolint:gocritic // matches vcs.Mutator.FetchAll's (ok bool, msg string, err error)
func FetchAll(ctx context.Context, ops vcs.Mutator, repoPath string) (bool, string, error) {
	ok, msg, err := ops.FetchAll(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("fetch all: %w", err)
	}

	return ok, msg, nil
}

// PruneRemote is a batch.TaskFunc that prunes stale remote-tracking refs for a repo.
//
//nolint:gocritic // matches vcs.Mutator.PruneRemote's (ok bool, msg string, err error)
func PruneRemote(ctx context.Context, ops vcs.Mutator, repoPath string) (bool, string, error) {
	ok, msg, err := ops.PruneRemote(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("prune remote: %w", err)
	}

	return ok, msg, nil
}

// CleanupMerged is a batch.TaskFunc that deletes merged branches for a repo.
//
//nolint:gocritic // matches vcs.Mutator.CleanupMergedBranches's (ok bool, msg string, err error)
func CleanupMerged(ctx context.Context, ops vcs.Mutator, repoPath string) (bool, string, error) {
	ok, msg, err := ops.CleanupMergedBranches(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("cleanup merged branches: %w", err)
	}

	return ok, msg, nil
}
