package batch

import (
	"context"
	"fmt"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func FetchAll(ctx context.Context, ops vcs.Operations, repoPath string) (bool, string, error) {
	ok, msg, err := ops.FetchAll(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("fetch all: %w", err)
	}

	return ok, msg, nil
}

func PruneRemote(ctx context.Context, ops vcs.Operations, repoPath string) (bool, string, error) {
	ok, msg, err := ops.PruneRemote(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("prune remote: %w", err)
	}

	return ok, msg, nil
}

func CleanupMerged(ctx context.Context, ops vcs.Operations, repoPath string) (bool, string, error) {
	ok, msg, err := ops.CleanupMergedBranches(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("cleanup merged branches: %w", err)
	}

	return ok, msg, nil
}
