package cache_test

import (
	"os"
	"testing"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
)

const (
	upstream = "github.com/kyleking/gh-repo-dashboard"
	listKey  = "prs"
)

func samplePRs() []forge.PullRequest {
	return []forge.PullRequest{
		{Number: 7, Title: "Cache pull requests on disk", State: "OPEN", HeadRef: "cache", Checks: forge.ChecksStatus{
			Total: 2, Passing: 2,
		}},
		{Number: 4, Title: "Stamp the checkout", State: "OPEN", HeadRef: "stamp"},
	}
}

// Refresh is pressed because something looks wrong; a stale pull request state
// left on disk would survive it.
//
//nolint:paralleltest // installs the process-wide store ClearAll drains
func TestClearAllDropsTheInstalledDiskCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewDiskCache(dir)

	cache.SetDiskCache(store)
	t.Cleanup(func() { cache.SetDiskCache(nil) })

	cache.Persist(cache.PRListCache, upstream, listKey, cache.NoStamp, samplePRs())
	if _, err := os.Stat(cache.DiskPath(store, upstream)); err != nil {
		t.Fatalf("the installed store never wrote the file: %v", err)
	}

	cache.ClearAll()

	if _, err := os.Stat(cache.DiskPath(store, upstream)); !os.IsNotExist(err) {
		t.Errorf("refresh left the file on disk: %v", err)
	}
	if _, ok := cache.Persisted(cache.PRListCache, upstream, listKey, cache.NoStamp); ok {
		t.Error("refresh served a value from the cache it just cleared")
	}
}
