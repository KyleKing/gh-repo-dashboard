package batch

import (
	"context"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// RepoName exposes the unexported repoName helper to black-box tests.
var RepoName = repoName

// SetGetMergedPRHeadsForTest overrides the getMergedPRHeads seam for black-box
// tests and returns a func that restores the original. Avoids shelling out to
// a real gh CLI when testing squash-merged detection.
func SetGetMergedPRHeadsForTest(fn func(ctx context.Context, repoPath string) (map[string]string, error)) func() {
	orig := getMergedPRHeads
	getMergedPRHeads = fn

	return func() { getMergedPRHeads = orig }
}

// SetGetOperationsForTest overrides the getOperations seam for black-box tests
// and returns a func that restores the original. Avoids shelling out to a real
// git/jj CLI when testing squash-merged detection.
func SetGetOperationsForTest(fn func(repoPath string) vcs.Operations) func() {
	orig := getOperations
	getOperations = fn

	return func() { getOperations = orig }
}
