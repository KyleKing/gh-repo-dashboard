// Package cache provides a generic in-memory TTL cache used to avoid
// redundant gh/git/jj calls across TUI refreshes.
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

// TTLCache is a generic in-memory cache whose entries expire after a fixed duration.
type TTLCache[T any] struct {
	mu      sync.RWMutex
	entries map[string]entry[T]
	ttl     time.Duration
}

// NewTTLCache returns an empty TTLCache with the given entry lifetime.
func NewTTLCache[T any](ttl time.Duration) *TTLCache[T] {
	return &TTLCache[T]{
		entries: make(map[string]entry[T]),
		ttl:     ttl,
	}
}

// Get returns the cached value for key and whether it was present and unexpired.
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

// Set stores value under key, expiring after the cache's configured TTL.
func (c *TTLCache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = entry[T]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Clear removes all entries from the cache.
func (c *TTLCache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]entry[T])
}

// Delete removes the entry for key, if any.
func (c *TTLCache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, key)
}

// Package-level caches shared across the app, keyed by repo path (or "path#N" for PR-numbered lookups).
var (
	PRCache       = NewTTLCache[*models.PRInfo](5 * time.Minute)
	PRListCache   = NewTTLCache[[]models.PRInfo](5 * time.Minute)
	PRDetailCache = NewTTLCache[*models.PRDetail](5 * time.Minute)
	BranchCache   = NewTTLCache[[]models.BranchInfo](5 * time.Minute)
	CommitCache   = NewTTLCache[[]models.CommitInfo](5 * time.Minute)
	WorkflowCache = NewTTLCache[*models.WorkflowSummary](2 * time.Minute)
)

// ClearAll clears every package-level cache.
func ClearAll() {
	PRCache.Clear()
	PRListCache.Clear()
	PRDetailCache.Clear()
	BranchCache.Clear()
	CommitCache.Clear()
	WorkflowCache.Clear()
}
