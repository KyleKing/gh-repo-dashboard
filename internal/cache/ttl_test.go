package cache_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func TestTTLCacheSetGet(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	c.Set("key1", cache.NoStamp, "value1")

	value, ok := c.Get("key1", cache.NoStamp)
	if !ok {
		t.Error("expected key to exist")
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got '%s'", value)
	}
}

func TestTTLCacheGetMissing(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	_, ok := c.Get("nonexistent", cache.NoStamp)
	if ok {
		t.Error("expected key to not exist")
	}
}

func TestTTLCacheExpiration(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	c := cache.NewTTLCacheWithClock[string](5*time.Minute, clock.now)

	c.Set("key1", cache.NoStamp, "value1")
	clock.advance(6 * time.Minute)

	_, ok := c.Get("key1", cache.NoStamp)
	if ok {
		t.Error("expected key to be expired")
	}
}

// fakeClock ages a cache without sleeping. One clock belongs to one test, so
// it needs no locking.
type fakeClock struct{ at time.Time }

func newFakeClock() *fakeClock {
	return &fakeClock{at: time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time          { return c.at }
func (c *fakeClock) advance(d time.Duration) { c.at = c.at.Add(d) }

func TestTTLCacheClear(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	c.Set("key1", cache.NoStamp, "value1")
	c.Set("key2", cache.NoStamp, "value2")

	c.Clear()

	_, ok1 := c.Get("key1", cache.NoStamp)
	_, ok2 := c.Get("key2", cache.NoStamp)

	if ok1 || ok2 {
		t.Error("expected all keys to be cleared")
	}
}

func TestTTLCacheDelete(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	c.Set("key1", cache.NoStamp, "value1")
	c.Set("key2", cache.NoStamp, "value2")

	c.Delete("key1")

	_, ok1 := c.Get("key1", cache.NoStamp)
	_, ok2 := c.Get("key2", cache.NoStamp)

	if ok1 {
		t.Error("expected key1 to be deleted")
	}
	if !ok2 {
		t.Error("expected key2 to still exist")
	}
}

func TestTTLCacheOverwrite(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	c.Set("key1", cache.NoStamp, "value1")
	c.Set("key1", cache.NoStamp, "value2")

	value, ok := c.Get("key1", cache.NoStamp)
	if !ok {
		t.Error("expected key to exist")
	}
	if value != "value2" {
		t.Errorf("expected 'value2', got '%s'", value)
	}
}

func TestTTLCacheWithInt(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[int](5 * time.Minute)

	c.Set("count", cache.NoStamp, 42)

	value, ok := c.Get("count", cache.NoStamp)
	if !ok {
		t.Error("expected key to exist")
	}
	if value != 42 {
		t.Errorf("expected 42, got %d", value)
	}
}

func TestTTLCacheWithStruct(t *testing.T) {
	t.Parallel()
	type TestData struct {
		Name  string
		Count int
	}

	c := cache.NewTTLCache[TestData](5 * time.Minute)

	data := TestData{Name: "test", Count: 5}
	c.Set("data", cache.NoStamp, data)

	value, ok := c.Get("data", cache.NoStamp)
	if !ok {
		t.Error("expected key to exist")
	}
	if value.Name != "test" || value.Count != 5 {
		t.Errorf("expected {test, 5}, got {%s, %d}", value.Name, value.Count)
	}
}

func TestClearAllCaches(t *testing.T) {
	t.Parallel()
	cache.PRCache.Set("test", cache.NoStamp, nil)
	cache.BranchCache.Set("test", cache.NoStamp, nil)
	cache.CommitCache.Set("test", cache.NoStamp, nil)
	cache.WorkflowCache.Set("test", cache.NoStamp, nil)

	cache.ClearAll()

	_, ok1 := cache.PRCache.Get("test", cache.NoStamp)
	_, ok2 := cache.BranchCache.Get("test", cache.NoStamp)
	_, ok3 := cache.CommitCache.Get("test", cache.NoStamp)
	_, ok4 := cache.WorkflowCache.Get("test", cache.NoStamp)

	if ok1 || ok2 || ok3 || ok4 {
		t.Error("expected all caches to be cleared")
	}
}

// A worktree borrows its parent's object store, but the branch list and commit
// log it reads are relative to its own HEAD, so it keeps its own entry rather
// than reading the answer the parent wrote.
func TestObjectStoreKeysSeparateAWorktreeFromItsParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parent := filepath.Join(root, "app")
	worktree := filepath.Join(root, "app-wt")
	other := filepath.Join(root, "lib")

	for _, dir := range []string{filepath.Join(parent, ".git"), other, worktree} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	pointer := "gitdir: " + filepath.Join(parent, ".git", "worktrees", "app-wt") + "\n"
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := vcs.CheckoutIdentity(worktree); got != parent {
		t.Fatalf("the fixture is not a worktree of the parent: identity = %q, want %q", got, parent)
	}

	branchKey := cache.BranchCacheKey(parent)
	commitKey := cache.CommitCacheKey(parent, 10)

	cache.BranchCache.Set(branchKey, cache.NoStamp, []models.BranchInfo{{Name: "main"}})
	cache.CommitCache.Set(commitKey, cache.NoStamp, []models.CommitInfo{{Subject: "init"}})

	t.Cleanup(func() {
		cache.BranchCache.Delete(branchKey)
		cache.CommitCache.Delete(commitKey)
	})

	if _, ok := cache.BranchCache.Get(cache.BranchCacheKey(worktree), cache.NoStamp); ok {
		t.Error("a worktree read its parent's branch list, whose current-branch marker is the parent's")
	}
	if _, ok := cache.CommitCache.Get(cache.CommitCacheKey(worktree, 10), cache.NoStamp); ok {
		t.Error("a worktree read its parent's commit log, which starts at the parent's HEAD")
	}
	if _, ok := cache.CommitCache.Get(cache.CommitCacheKey(parent, 20), cache.NoStamp); ok {
		t.Error("a different log depth read the 10-commit entry")
	}
	if _, ok := cache.BranchCache.Get(cache.BranchCacheKey(other), cache.NoStamp); ok {
		t.Error("an unrelated checkout read the parent's branch list")
	}
}

// A branch list read from an unchanged checkout is still correct however old
// it is, so the stamp replaces the TTL rather than sitting under it.
func TestAnUnchangedStampCarriesALocalValuePastItsTTL(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	c := cache.NewTTLCacheWithClock[[]models.BranchInfo](5*time.Minute, clock.now)

	stamp := cache.Stamp{Scope: "/repo", Fingerprint: "head-a"}
	moved := cache.Stamp{Scope: "/repo", Fingerprint: "head-b"}
	key := cache.BranchCacheKey("/repo")

	c.Set(key, stamp, []models.BranchInfo{{Name: "main"}})
	clock.advance(time.Hour)

	if got, ok := c.Fresh(key, stamp); !ok || len(got) != 1 {
		t.Errorf("an unchanged checkout refetched its branch list: %+v, hit=%v", got, ok)
	}
	if _, ok := c.Fresh(key, moved); ok {
		t.Error("a commit left the branch list cached")
	}
	if _, ok := c.Fresh(key, cache.NoStamp); ok {
		t.Error("an unstampable checkout was served a local value")
	}
	if _, ok := c.Get(key, stamp); ok {
		t.Error("the TTL is still the ceiling for Get, whatever the stamp says")
	}
}

// A stamp cannot prove a pull request is still open, because someone else can
// merge it without touching this working copy. It only evicts early.
func TestAStampOnlyShortensARemoteValuesLife(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	c := cache.NewTTLCacheWithClock[[]models.PRInfo](5*time.Minute, clock.now)

	const key = "github.com/acme/app\x00acme/app:all_prs"

	before := cache.Stamp{Scope: "/repo", Fingerprint: "unpushed"}
	after := cache.Stamp{Scope: "/repo", Fingerprint: "pushed"}
	peer := cache.Stamp{Scope: "/peer-checkout", Fingerprint: "its-own-head"}

	c.Set(key, before, []models.PRInfo{{Number: 1}})

	if _, ok := c.Get(key, before); !ok {
		t.Error("an unchanged checkout refetched the PR list inside the TTL")
	}
	if _, ok := c.Get(key, peer); !ok {
		t.Error("a second checkout of the same remote refetched the PR list")
	}
	if _, ok := c.Get(key, after); ok {
		t.Error("a push left the PR list cached")
	}

	c.Set(key, after, []models.PRInfo{{Number: 2}})
	clock.advance(6 * time.Minute)

	if _, ok := c.Get(key, after); ok {
		t.Error("an unchanged stamp carried the PR list past its TTL")
	}
}

// A checkout that read the entry earlier is the one a later change evicts it
// for; a checkout meeting the entry for the first time is a new reader, not a
// local change, and must not throw away what its peers just paid for.
func TestOneCheckoutsChangeDoesNotEvictForAnother(t *testing.T) {
	t.Parallel()

	c := cache.NewTTLCache[int](5 * time.Minute)
	const key = "shared"

	c.Set(key, cache.Stamp{Scope: "/a", Fingerprint: "a1"}, 7)

	if _, ok := c.Get(key, cache.Stamp{Scope: "/b", Fingerprint: "b1"}); !ok {
		t.Fatal("a first-time reader was treated as a changed checkout")
	}
	if _, ok := c.Get(key, cache.Stamp{Scope: "/b", Fingerprint: "b2"}); ok {
		t.Error("the reader's own later change did not evict")
	}
}
