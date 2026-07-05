package cache_test

import (
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
)

func TestTTLCacheSetGet(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	c.Set("key1", "value1")

	value, ok := c.Get("key1")
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

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected key to not exist")
	}
}

func TestTTLCacheExpiration(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](10 * time.Millisecond)

	c.Set("key1", "value1")

	time.Sleep(20 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected key to be expired")
	}
}

func TestTTLCacheClear(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	c.Clear()

	_, ok1 := c.Get("key1")
	_, ok2 := c.Get("key2")

	if ok1 || ok2 {
		t.Error("expected all keys to be cleared")
	}
}

func TestTTLCacheDelete(t *testing.T) {
	t.Parallel()
	c := cache.NewTTLCache[string](5 * time.Minute)

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	c.Delete("key1")

	_, ok1 := c.Get("key1")
	_, ok2 := c.Get("key2")

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

	c.Set("key1", "value1")
	c.Set("key1", "value2")

	value, ok := c.Get("key1")
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

	c.Set("count", 42)

	value, ok := c.Get("count")
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
	c.Set("data", data)

	value, ok := c.Get("data")
	if !ok {
		t.Error("expected key to exist")
	}
	if value.Name != "test" || value.Count != 5 {
		t.Errorf("expected {test, 5}, got {%s, %d}", value.Name, value.Count)
	}
}

func TestClearAllCaches(t *testing.T) {
	t.Parallel()
	cache.PRCache.Set("test", nil)
	cache.BranchCache.Set("test", nil)
	cache.CommitCache.Set("test", nil)
	cache.WorkflowCache.Set("test", nil)

	cache.ClearAll()

	_, ok1 := cache.PRCache.Get("test")
	_, ok2 := cache.BranchCache.Get("test")
	_, ok3 := cache.CommitCache.Get("test")
	_, ok4 := cache.WorkflowCache.Get("test")

	if ok1 || ok2 || ok3 || ok4 {
		t.Error("expected all caches to be cleared")
	}
}
