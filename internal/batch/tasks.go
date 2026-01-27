package batch

import (
	"context"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func FetchAll(ctx context.Context, ops vcs.Operations, repoPath string) (bool, string, error) {
	return ops.FetchAll(ctx, repoPath)
}

func PruneRemote(ctx context.Context, ops vcs.Operations, repoPath string) (bool, string, error) {
	return ops.PruneRemote(ctx, repoPath)
}

func CleanupMerged(ctx context.Context, ops vcs.Operations, repoPath string) (bool, string, error) {
	return ops.CleanupMergedBranches(ctx, repoPath)
}
