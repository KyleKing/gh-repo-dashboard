package github_test

import (
	"strings"
	"testing"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
)

const mutateRepoPath = "/repo"

//nolint:paralleltest // asserts against shared global cache state
func TestCreatePR(t *testing.T) {
	cache.ClearAll()

	ctx, calls := stubRunGH([]byte("https://github.com/acme/app/pull/7\n"), nil)
	cache.PRListCache.Set(
		github.PRListCacheKey(mutateRepoPath, testRemoteID, "origin/main"),
		cache.NoStamp,
		[]forge.PullRequest{{Number: 1}},
	)

	url, err := github.CreatePR(ctx, mutateRepoPath, "feature", "main")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/acme/app/pull/7" {
		t.Errorf("expected the new PR URL, got %q", url)
	}

	got := strings.Join((*calls)[0], " ")
	if got != "pr create --fill --head feature --base main" {
		t.Errorf("unexpected gh args: %q", got)
	}
	if _, ok := github.CachedPRs(mutateRepoPath, testRemoteID, "origin/main"); ok {
		t.Error("expected the PR list cache to be invalidated")
	}
}

//nolint:paralleltest // asserts against shared global cache state
func TestCreatePRWithoutBase(t *testing.T) {
	cache.ClearAll()

	ctx, calls := stubRunGH([]byte("https://github.com/acme/app/pull/8"), nil)
	if _, err := github.CreatePR(ctx, mutateRepoPath, "feature", ""); err != nil {
		t.Fatal(err)
	}

	got := strings.Join((*calls)[0], " ")
	if got != "pr create --fill --head feature" {
		t.Errorf("unexpected gh args: %q", got)
	}
}

//nolint:paralleltest // asserts against shared global cache state
func TestCreatePRError(t *testing.T) {
	cache.ClearAll()

	ctx, _ := stubRunGH(nil, errGHFailed)
	if _, err := github.CreatePR(ctx, mutateRepoPath, "feature", "main"); err == nil {
		t.Error("expected an error when gh fails")
	}
}

//nolint:paralleltest // asserts against shared global cache state
func TestSquashMergePR(t *testing.T) {
	cache.ClearAll()

	ctx, calls := stubRunGH([]byte(""), nil)
	cache.MergedPRHeadsCache.Set(
		github.MergedPRHeadsCacheKey(mutateRepoPath, testRemoteID),
		cache.NoStamp,
		map[string]string{"feature": "abc123"},
	)

	if err := github.SquashMergePR(ctx, mutateRepoPath, 42); err != nil {
		t.Fatal(err)
	}

	got := strings.Join((*calls)[0], " ")
	if got != "pr merge 42 --squash --delete-branch" {
		t.Errorf("unexpected gh args: %q", got)
	}
	if _, ok := github.CachedMergedPRHeads(mutateRepoPath, testRemoteID); ok {
		t.Error("expected the merged-PR head cache to be invalidated")
	}
}

//nolint:paralleltest // asserts against shared global cache state
func TestSquashMergePRError(t *testing.T) {
	cache.ClearAll()

	ctx, _ := stubRunGH(nil, errGHFailed)
	if err := github.SquashMergePR(ctx, mutateRepoPath, 42); err == nil {
		t.Error("expected an error when gh fails")
	}
}
