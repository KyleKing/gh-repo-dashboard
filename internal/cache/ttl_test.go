package cache_test

import (
	"testing"

	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
)

// ClearAll reaches caches registered from another module, which is the only
// reason the registry exists rather than a list this package maintains.
func TestClearAllCaches(t *testing.T) {
	t.Parallel()

	cache.PRCache.Set("test", cache.NoStamp, nil)
	cache.WorkflowCache.Set("test", cache.NoStamp, nil)
	vcs.BranchCache.Set("test", cache.NoStamp, nil)
	vcs.CommitCache.Set("test", cache.NoStamp, nil)

	cache.ClearAll()

	_, pr := cache.PRCache.Get("test", cache.NoStamp)
	_, workflow := cache.WorkflowCache.Get("test", cache.NoStamp)
	_, branch := vcs.BranchCache.Get("test", cache.NoStamp)
	_, commit := vcs.CommitCache.Get("test", cache.NoStamp)

	if pr || workflow || branch || commit {
		t.Error("expected every registered cache to be cleared")
	}
}
