package tasks

import (
	"context"

	"github.com/kyleking/aragonite/vcs"
)

// SetGetMergedPRHeadsForTest overrides the getMergedPRHeads seam for black-box
// tests and returns a func that restores the original.
func SetGetMergedPRHeadsForTest(
	fn func(ctx context.Context, repoPath, remoteID string) (map[string]string, error),
) func() {
	orig := getMergedPRHeads
	getMergedPRHeads = fn

	return func() { getMergedPRHeads = orig }
}

// SetGetOperationsForTest overrides the getOperations seam for black-box tests
// and returns a func that restores the original.
func SetGetOperationsForTest(fn func(repoPath string) vcs.Operations) func() {
	orig := getOperations
	getOperations = fn

	return func() { getOperations = orig }
}
