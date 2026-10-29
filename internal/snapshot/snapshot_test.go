package snapshot_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/snapshot"
)

const (
	upstream = "github.com/kyleking/app"
	path     = "/repos/app"
)

var stamp = cache.Stamp{Scope: path, Fingerprint: "head-a"}

func row() snapshot.Row {
	return snapshot.Row{
		Summary:     models.RepoSummary{RepoSummary: vcs.RepoSummary{Branch: "main"}},
		BranchCount: 3,
	}
}

// A second open reads the row the first one left, under the remote the file
// belongs to and nowhere else.
func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	snapshot.Open(cache.NewDiskCache(dir)).Save(upstream, path, stamp, row())

	got, _, ok := snapshot.Open(cache.NewDiskCache(dir)).Load(upstream, path, stamp)
	if !ok {
		t.Fatal("a saved row did not read back")
	}
	if got.Summary.Branch != "main" || got.BranchCount != 3 {
		t.Errorf("row = %+v", got)
	}

	if _, _, ok := snapshot.Open(cache.NewDiskCache(dir)).Load("github.com/other/app", path, stamp); ok {
		t.Error("another upstream read this one's row")
	}
}

// A checkout that committed since the row was written does not get it back:
// the branch it names is not the branch the checkout is on anymore.
func TestStoreMissesAfterACommit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	snapshot.Open(cache.NewDiskCache(dir)).Save(upstream, path, stamp, row())

	moved := cache.Stamp{Scope: path, Fingerprint: "head-b"}
	if _, _, ok := snapshot.Open(cache.NewDiskCache(dir)).Load(upstream, path, moved); ok {
		t.Error("a moved checkout was served the row written before it moved")
	}
}

// A checkout with no remote keeps nothing: its whole row reads back locally
// in the time a lookup would take anyway.
func TestStoreSkipsRemotelessCheckouts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	snapshot.Open(cache.NewDiskCache(dir)).Save("", path, stamp, row())

	if _, _, ok := snapshot.Open(cache.NewDiskCache(dir)).Load("", path, stamp); ok {
		t.Error("a remoteless checkout kept a row")
	}
}

// A record from another schema reads as absent, never as zero values.
func TestStoreDropsOldSchema(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := row()
	r.Version = 99

	mem := cache.NewTTLCache[snapshot.Row](time.Hour)
	cache.NewSnapshot(mem, cache.NewDiskCache(dir), upstream,
		upstream+"\x00row\x00"+path).Save(stamp, r)

	if _, _, ok := snapshot.Open(cache.NewDiskCache(dir)).Load(upstream, path, stamp); ok {
		t.Error("a record under another schema read back")
	}
}

// A nil store answers nothing, which is what a run without a cache directory
// or without the snapshot feature should see.
func TestNilStore(t *testing.T) {
	t.Parallel()

	var store *snapshot.Store
	store.Save(upstream, path, stamp, row())

	if _, _, ok := store.Load(upstream, path, stamp); ok {
		t.Error("a nil store read back a row")
	}
}
