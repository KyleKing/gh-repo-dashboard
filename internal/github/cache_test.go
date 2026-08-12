package github_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// repoWithOriginHead builds a repo whose origin/HEAD resolves without a
// network, which is all DefaultBranchHead reads.
func repoWithOriginHead(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "--initial-branch", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--allow-empty", "-m", "init"},
		{"update-ref", "refs/remotes/origin/main", "HEAD"},
		{"symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...) // #nosec G204
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	return dir
}

// useTempDiskCache installs a store under a temp directory and drops both it and
// the memory caches afterwards, since all three are process-global.
func useTempDiskCache(t *testing.T) {
	t.Helper()

	cache.SetDiskCache(cache.NewDiskCache(t.TempDir()))
	t.Cleanup(func() {
		cache.SetDiskCache(nil)
		cache.PRCache.Clear()
		cache.DefaultBranchCICache.Clear()
	})
}

// TestGitHubValuesSurviveAColdMemoryCache is what --cli without --fresh depends
// on: a value written by an earlier process is still readable once the memory
// cache is empty.
//
//nolint:paralleltest // the disk store and the package caches are process-global
func TestGitHubValuesSurviveAColdMemoryCache(t *testing.T) {
	useTempDiskCache(t)

	repo := repoWithOriginHead(t)
	ctx := context.Background()

	pr := &models.PRInfo{Number: 12, Title: "Add the thing", HeadRef: "feat"}
	prKey := github.PRCacheKey(repo, testRemoteID, "origin/main", "feat")
	cache.Persist(cache.PRCache, testRemoteID, prKey, cache.NoStamp, pr)

	ci := &models.DefaultBranchCI{Branch: "main", Workflows: []models.CIWorkflowRun{{Workflow: "ci"}}}
	ciKey := github.DefaultBranchCICacheKey(repo, testRemoteID, defaultBranchSHA(t, repo))
	cache.Persist(cache.DefaultBranchCICache, testRemoteID, ciKey, cache.NoStamp, ci)

	cache.PRCache.Clear()
	cache.DefaultBranchCICache.Clear()

	gotPR, ok := github.CachedPRForBranch(repo, testRemoteID, "feat", "origin/main")
	if !ok || gotPR.Number != pr.Number {
		t.Errorf("CachedPRForBranch = %+v, %v; want the persisted PR #%d", gotPR, ok, pr.Number)
	}

	gotCI, ok := github.CachedDefaultBranchCI(ctx, repo, testRemoteID)
	if !ok || gotCI.Branch != ci.Branch {
		t.Errorf("CachedDefaultBranchCI = %+v, %v; want the persisted run for %q", gotCI, ok, ci.Branch)
	}
}

func defaultBranchSHA(t *testing.T, repo string) string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "git", "-C", repo, "rev-parse", "origin/HEAD").Output() // #nosec G204
	if err != nil {
		t.Fatalf("git rev-parse origin/HEAD: %v", err)
	}

	return strings.TrimSpace(string(out))
}
