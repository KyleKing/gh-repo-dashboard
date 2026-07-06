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

// clearer is satisfied by any TTLCache, letting the registry hold caches of
// differing type parameters.
type clearer interface {
	Clear()
}

var (
	registryMu sync.Mutex
	registry   []clearer
)

// newRegisteredTTLCache builds a TTLCache like NewTTLCache and appends it to
// the package-level registry that ClearAll drains. Reserved for the
// package-level cache variables below; tests wanting a throwaway cache should
// use NewTTLCache directly so they don't accumulate in the registry.
func newRegisteredTTLCache[T any](ttl time.Duration) *TTLCache[T] {
	c := NewTTLCache[T](ttl)

	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, c)

	return c
}

// Get returns the cached value for key and whether it was present and unexpired.
//
//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
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

const (
	defaultTTL  = 5 * time.Minute
	workflowTTL = 2 * time.Minute
)

// Package-level caches shared across the app, keyed by repo path (or "path#N" for PR-numbered lookups).
var (
	PRCache            = newRegisteredTTLCache[*models.PRInfo](defaultTTL)
	PRListCache        = newRegisteredTTLCache[[]models.PRInfo](defaultTTL)
	PRDetailCache      = newRegisteredTTLCache[*models.PRDetail](defaultTTL)
	BranchCache        = newRegisteredTTLCache[[]models.BranchInfo](defaultTTL)
	CommitCache        = newRegisteredTTLCache[[]models.CommitInfo](defaultTTL)
	WorkflowCache      = newRegisteredTTLCache[*models.WorkflowSummary](workflowTTL)
	MergedPRHeadsCache = newRegisteredTTLCache[map[string]string](defaultTTL)
)

// ClearAll clears every registered package-level cache.
func ClearAll() {
	registryMu.Lock()
	defer registryMu.Unlock()

	for _, c := range registry {
		c.Clear()
	}
}
