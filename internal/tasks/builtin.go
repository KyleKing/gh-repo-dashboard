package tasks

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kyleking/aragonite/forge/github"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
)

var (
	// ErrReserved reports a configured task named like a built-in.
	ErrReserved = errors.New("shadows a built-in task")
	// ErrNotOK reports a built-in that finished without succeeding. Its
	// message holds the reason.
	ErrNotOK = errors.New("did not succeed")
)

// Built-in task names, shared by the engine, the operator keys, and :commands.
const (
	NameCleanup        = "cleanup"
	NameCleanupPreview = "cleanup-preview"
	NameFetch          = "fetch"
	NamePrune          = "prune"
	NameRefreshPRs     = "refresh-prs"
	NameShell          = "shell"
)

// getMergedPRHeads and getOperations are swappable seams over
// github.GetMergedPRHeads and vcs.GetOperations so tests can stub them without
// shelling out to gh/git; see export_test.go.
var (
	getMergedPRHeads = github.GetMergedPRHeads
	getOperations    = vcs.GetOperations
)

// Builtins returns the function-backed tasks that ship with the dashboard.
func Builtins() []config.Task {
	return []config.Task{
		{Name: NameShell, Desc: "a shell in the repo", Run: `exec "${SHELL:-sh}"`, Interactive: true},
		{Name: NameCleanup, Desc: "delete merged and squash-merged branches", Confirm: true, Func: cleanupMerged},
		{Name: NameCleanupPreview, Desc: "list the branches cleanup would delete", Func: previewCleanup},
		{Name: NameFetch, Desc: "git fetch all remotes", Func: fetchAll},
		{Name: NamePrune, Desc: "prune stale remote-tracking refs", Confirm: true, Func: pruneRemote},
		{
			Name: NameRefreshPRs, Desc: "re-read open pull requests from GitHub",
			Concurrency: 1, Func: refreshPRs,
		},
	}
}

// Registry lists the configured tasks followed by the built-ins. A configured
// task may not reuse a built-in's name.
func Registry(configured []config.Task) ([]config.Task, error) {
	builtins := Builtins()
	for _, task := range configured {
		for _, builtin := range builtins {
			if task.Name == builtin.Name {
				return nil, fmt.Errorf("task %q %w", task.Name, ErrReserved)
			}
		}
	}

	return append(append([]config.Task{}, configured...), builtins...), nil
}

func outcome(ok bool, message string, err error, wrap string) (string, error) {
	switch {
	case err != nil:
		return message, fmt.Errorf("%s: %w", wrap, err)
	case !ok:
		return message, ErrNotOK
	}

	return message, nil
}

func fetchAll(ctx context.Context, repoPath string) (string, error) {
	ok, msg, err := getOperations(repoPath).FetchAll(ctx, repoPath)

	return outcome(ok, msg, err, "fetch all")
}

func pruneRemote(ctx context.Context, repoPath string) (string, error) {
	ok, msg, err := getOperations(repoPath).PruneRemote(ctx, repoPath)

	return outcome(ok, msg, err, "prune remote")
}

// refreshPRs drops a repo's cached pull request data and re-reads it, so the
// fleet's PR columns reflect the remote without a whole-app refresh. One gh
// call per repo, as the API budget requires.
func refreshPRs(ctx context.Context, repoPath string) (string, error) {
	summary, err := getOperations(repoPath).GetRepoSummary(ctx, repoPath)
	if err != nil {
		return "no repo summary", fmt.Errorf("reading repo summary: %w", err)
	}
	if summary.Upstream == "" {
		return "no upstream, nothing to refresh", nil
	}

	github.InvalidatePRCaches()

	prs, err := github.GetPRsForRepo(ctx, repoPath, summary.RemoteID, summary.Upstream)
	if err != nil {
		return "refresh failed", fmt.Errorf("refreshing pull requests: %w", err)
	}

	return fmt.Sprintf("%d open PRs", len(prs)), nil
}

// cleanupMerged deletes merged and squash-merged branches, detecting the
// squash-merged set through reads that the narrower Mutator does not offer.
func cleanupMerged(ctx context.Context, repoPath string) (string, error) {
	ok, msg, err := getOperations(repoPath).CleanupMergedBranches(ctx, repoPath, squashMergedBranches(ctx, repoPath))

	return outcome(ok, msg, err, "cleanup merged branches")
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

// previewCleanup reports what cleanupMerged would delete without deleting
// anything. The preview has nothing to fail at beyond best-effort reads.
func previewCleanup(ctx context.Context, repoPath string) (string, error) {
	var merged []string
	if mp, ok := getOperations(repoPath).(mergePreviewer); ok {
		_, merged, _ = mp.PreviewMergedBranches(ctx, repoPath) //nolint:errcheck // best-effort, see comment above
	}

	wouldDelete := slices.Concat(merged, squashMergedBranches(ctx, repoPath))
	if len(wouldDelete) == 0 {
		return "No merged branches to delete", nil
	}

	return fmt.Sprintf("Would delete %d branches: %s", len(wouldDelete), strings.Join(wouldDelete, ", ")), nil
}
