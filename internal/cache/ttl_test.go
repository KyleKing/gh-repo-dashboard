package cache

import (
	"testing"
	"time"
)

func TestTTLCacheSetGet(t *testing.T) {
	cache := NewTTLCache[string](5 * time.Minute)

	cache.Set("key1", "value1")

	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected key to exist")
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got '%s'", value)
	}
}

func TestTTLCacheGetMissing(t *testing.T) {
	cache := NewTTLCache[string](5 * time.Minute)

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected key to not exist")
	}
}

func TestTTLCacheExpiration(t *testing.T) {
	cache := NewTTLCache[string](10 * time.Millisecond)

	cache.Set("key1", "value1")

	time.Sleep(20 * time.Millisecond)

	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected key to be expired")
	}
}

func TestTTLCacheClear(t *testing.T) {
	cache := NewTTLCache[string](5 * time.Minute)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Clear()

	_, ok1 := cache.Get("key1")
	_, ok2 := cache.Get("key2")

	if ok1 || ok2 {
		t.Error("expected all keys to be cleared")
	}
}

func TestTTLCacheDelete(t *testing.T) {
	cache := NewTTLCache[string](5 * time.Minute)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Delete("key1")

	_, ok1 := cache.Get("key1")
	_, ok2 := cache.Get("key2")

	if ok1 {
		t.Error("expected key1 to be deleted")
	}
	if !ok2 {
		t.Error("expected key2 to still exist")
	}
}

func TestTTLCacheOverwrite(t *testing.T) {
	cache := NewTTLCache[string](5 * time.Minute)

	cache.Set("key1", "value1")
	cache.Set("key1", "value2")

	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected key to exist")
	}
	if value != "value2" {
		t.Errorf("expected 'value2', got '%s'", value)
	}
}

func TestTTLCacheWithInt(t *testing.T) {
	cache := NewTTLCache[int](5 * time.Minute)

	cache.Set("count", 42)

	value, ok := cache.Get("count")
	if !ok {
		t.Error("expected key to exist")
	}
	if value != 42 {
		t.Errorf("expected 42, got %d", value)
	}
}

func TestTTLCacheWithStruct(t *testing.T) {
	type TestData struct {
		Name  string
		Count int
	}

	cache := NewTTLCache[TestData](5 * time.Minute)

	data := TestData{Name: "test", Count: 5}
	cache.Set("data", data)

	value, ok := cache.Get("data")
	if !ok {
		t.Error("expected key to exist")
	}
	if value.Name != "test" || value.Count != 5 {
		t.Errorf("expected {test, 5}, got {%s, %d}", value.Name, value.Count)
	}
}

func TestClearAllCaches(t *testing.T) {
	PRCache.Set("test", nil)
	BranchCache.Set("test", nil)
	CommitCache.Set("test", nil)
	WorkflowCache.Set("test", nil)

	ClearAll()

	_, ok1 := PRCache.Get("test")
	_, ok2 := BranchCache.Get("test")
	_, ok3 := CommitCache.Get("test")
	_, ok4 := WorkflowCache.Get("test")

	if ok1 || ok2 || ok3 || ok4 {
		t.Error("expected all caches to be cleared")
	}
}
