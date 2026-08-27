package batch

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyleking/aragonite/forge/github"
	"github.com/kyleking/aragonite/vcs"
)

// FetchAll is a batch.TaskFunc that fetches all remotes for a repo.
func FetchAll(ctx context.Context, ops vcs.Mutator, repoPath string) (bool, string, error) {
	ok, msg, err := ops.FetchAll(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("fetch all: %w", err)
	}

	return ok, msg, nil
}

// RefreshPRs is a batch.TaskFunc that drops a repo's cached pull request data
// and re-reads it, so the fleet's PR columns reflect the remote without a
// whole-app refresh. One gh call per repo, as the API budget requires.
func RefreshPRs(ctx context.Context, _ vcs.Mutator, repoPath string) (bool, string, error) {
	summary, err := vcs.GetOperations(repoPath).GetRepoSummary(ctx, repoPath)
	if err != nil {
		return false, "no repo summary", fmt.Errorf("reading repo summary: %w", err)
	}
	if summary.Upstream == "" {
		return true, "no upstream, nothing to refresh", nil
	}

	github.InvalidatePRCaches()

	prs, err := github.GetPRsForRepo(ctx, repoPath, summary.RemoteID, summary.Upstream)
	if err != nil {
		return false, "refresh failed", fmt.Errorf("refreshing pull requests: %w", err)
	}

	return true, fmt.Sprintf("%d open PRs", len(prs)), nil
}

// PruneRemote is a batch.TaskFunc that prunes stale remote-tracking refs for a repo.
func PruneRemote(ctx context.Context, ops vcs.Mutator, repoPath string) (bool, string, error) {
	ok, msg, err := ops.PruneRemote(ctx, repoPath)
	if err != nil {
		return ok, msg, fmt.Errorf("prune remote: %w", err)
	}

	return ok, msg, nil
}

// getMergedPRHeads and getOperations are swappable seams over github.GetMergedPRHeads
// and vcs.GetOperations so tests can stub them without shelling out to gh/git; see
// export_test.go.
var (
	getMergedPRHeads = github.GetMergedPRHeads
	getOperations    = vcs.GetOperations
)

// CleanupMerged is a batch.TaskFunc that deletes merged and squash-merged
// branches for a repo, detecting the squash-merged set via reads on the full
// vcs.Operations for repoPath rather than the narrower Mutator passed in,
// since TaskFunc only needs write access for the call itself.
func CleanupMerged(ctx context.Context, ops vcs.Mutator, repoPath string) (bool, string, error) {
	squashMerged := squashMergedBranches(ctx, repoPath)

	ok, msg, err := ops.CleanupMergedBranches(ctx, repoPath, squashMerged)
	if err != nil {
		return ok, msg, fmt.Errorf("cleanup merged branches: %w", err)
	}

	return ok, msg, nil
}

// squashMergedBranches returns local branch names whose tip commit matches a
// merged pull request's head OID: a squash-merged branch git/jj's own
// merge-tracking can't detect because the squash commit is new history. It's
// best-effort; a missing gh or a read failure just yields no candidates.
func squashMergedBranches(ctx context.Context, repoPath string) []string {
	heads, err := getMergedPRHeads(ctx, repoPath, vcs.RemoteIdentityFor(ctx, repoPath))
	if err != nil || len(heads) == 0 {
		return nil
	}

	branches, err := getOperations(repoPath).GetBranchList(ctx, repoPath)
	if err != nil {
		return nil
	}

	var squashMerged []string
	for _, b := range branches {
		if b.IsCurrent || b.IsRemote || b.Head == "" {
			continue
		}
		if oid, ok := heads[b.Name]; ok && oid == b.Head {
			squashMerged = append(squashMerged, b.Name)
		}
	}

	return squashMerged
}

// mergePreviewer is implemented by GitOperations and JJOperations to report
// what CleanupMergedBranches would delete without deleting anything. It's not
// part of vcs.Operations since it exists only for the dry-run preview.
type mergePreviewer interface {
	PreviewMergedBranches(ctx context.Context, repoPath string) (defaultBranch string, merged []string, err error)
}

// PreviewCleanup is a batch.TaskFunc that reports what CleanupMerged would
// delete for a repo (merged and squash-merged branches) without deleting
// anything, backing `:cleanup --dry-run`. The success result is always true
// since a preview has nothing to fail at beyond the best-effort reads inside it.
//
//nolint:unparam // matches batch.TaskFunc's (ok bool, msg string, err error)
func PreviewCleanup(ctx context.Context, _ vcs.Mutator, repoPath string) (bool, string, error) {
	ops := getOperations(repoPath)

	var merged []string
	if mp, ok := ops.(mergePreviewer); ok {
		_, merged, _ = mp.PreviewMergedBranches(ctx, repoPath) //nolint:errcheck // best-effort, see comment above
	}

	squashMerged := squashMergedBranches(ctx, repoPath)

	wouldDelete := make([]string, 0, len(merged)+len(squashMerged))
	wouldDelete = append(wouldDelete, merged...)
	wouldDelete = append(wouldDelete, squashMerged...)

	if len(wouldDelete) == 0 {
		return true, "No merged branches to delete", nil
	}

	return true, fmt.Sprintf("Would delete %d branches: %s", len(wouldDelete), strings.Join(wouldDelete, ", ")), nil
}
