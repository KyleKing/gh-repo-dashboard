package cache

import (
	"sync"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

type TTLCache[T any] struct {
	mu      sync.RWMutex
	entries map[string]entry[T]
	ttl     time.Duration
}

func NewTTLCache[T any](ttl time.Duration) *TTLCache[T] {
	return &TTLCache[T]{
		entries: make(map[string]entry[T]),
		ttl:     ttl,
	}
}

func (c *TTLCache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[key]
	if !ok {
		var zero T
		return zero, false
	}

	if time.Now().After(e.expiresAt) {
		var zero T
		return zero, false
	}

	return e.value, true
}

func (c *TTLCache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = entry[T]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *TTLCache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]entry[T])
}

func (c *TTLCache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

var (
	PRCache       = NewTTLCache[*models.PRInfo](5 * time.Minute)
	PRListCache   = NewTTLCache[[]models.PRInfo](5 * time.Minute)
	PRDetailCache = NewTTLCache[*models.PRDetail](5 * time.Minute)
	BranchCache   = NewTTLCache[[]models.BranchInfo](5 * time.Minute)
	CommitCache   = NewTTLCache[[]models.CommitInfo](5 * time.Minute)
	WorkflowCache = NewTTLCache[*models.WorkflowSummary](2 * time.Minute)
)

func ClearAll() {
	PRCache.Clear()
	PRListCache.Clear()
	PRDetailCache.Clear()
	BranchCache.Clear()
	CommitCache.Clear()
	WorkflowCache.Clear()
}
