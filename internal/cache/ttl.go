// Package cache provides a generic in-memory TTL cache used to avoid
// redundant gh/git/jj calls across TUI refreshes.
package cache

import (
	"strconv"
	"sync"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// Stamp is what a checkout looked like when a value was read from it. The
// cache compares Fingerprint for equality and never interprets it; Scope names
// the checkout it came from, so an entry several checkouts of one remote share
// is only evicted for the checkout that actually changed. An empty
// Fingerprint proves nothing: it is what a caller passes for a value no
// local state can invalidate, and what vcs returns for a checkout it could
// not read.
type Stamp struct {
	Scope       string
	Fingerprint string
}

// NoStamp is the stamp for a value no local state can invalidate.
var NoStamp = Stamp{} //nolint:gochecknoglobals // an empty-value constant, never assigned to

type entry[T any] struct {
	value     T
	expiresAt time.Time
	seen      map[string]string
}

// TTLCache is a generic in-memory cache whose entries expire after a fixed duration.
type TTLCache[T any] struct {
	mu      sync.RWMutex
	entries map[string]entry[T]
	ttl     time.Duration
	now     func() time.Time
}

// NewTTLCache returns an empty TTLCache with the given entry lifetime.
func NewTTLCache[T any](ttl time.Duration) *TTLCache[T] {
	return &TTLCache[T]{
		entries: make(map[string]entry[T]),
		ttl:     ttl,
		now:     time.Now,
	}
}

// clearer is satisfied by any TTLCache, letting the registry hold caches of
// differing type parameters.
type clearer interface {
	Clear()
	setTTL(ttl time.Duration)
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

// Get returns the cached value for key, for a value the checkout cannot prove
// still correct. The TTL is the ceiling and stamp only lowers it: a checkout
// whose stamp moved since it last touched this entry evicts it early, because
// a local change (a push above all) is exactly when a remote-derived value
// stops matching. Pass NoStamp for a value no local state bears on.
//
//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
func (c *TTLCache[T]) Get(key string, stamp Stamp) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var zero T

	e, ok := c.entries[key]
	if !ok {
		return zero, false
	}

	if c.now().After(e.expiresAt) {
		delete(c.entries, key)

		return zero, false
	}

	if stamp.Fingerprint != "" {
		prev, recorded := e.seen[stamp.Scope]
		switch {
		case recorded && prev != stamp.Fingerprint:
			delete(c.entries, key)

			return zero, false
		case !recorded:
			e.seen[stamp.Scope] = stamp.Fingerprint
		}
	}

	return e.value, true
}

// Fresh returns the cached value for key when stamp matches the one it was
// written under, for a value derived from local state alone. An unchanged
// checkout cannot have changed the answer, so the entry stays correct however
// old it is and the TTL never evicts it. A checkout that could not be stamped
// always misses.
//
//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
func (c *TTLCache[T]) Fresh(key string, stamp Stamp) (T, bool) {
	var zero T

	if stamp.Fingerprint == "" {
		return zero, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	e, ok := c.entries[key]
	if !ok || e.seen[stamp.Scope] != stamp.Fingerprint {
		return zero, false
	}

	return e.value, true
}

// Set stores value under key as read from the checkout stamp describes,
// expiring after the cache's configured TTL.
func (c *TTLCache[T]) Set(key string, stamp Stamp, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	seen := make(map[string]string, 1)
	if stamp.Fingerprint != "" {
		seen[stamp.Scope] = stamp.Fingerprint
	}

	c.entries[key] = entry[T]{
		value:     value,
		expiresAt: c.now().Add(c.ttl),
		seen:      seen,
	}
}

// restore seeds an entry read back from disk, keeping the expiry and the
// fingerprints it was written under so a persisted value neither restarts its
// TTL nor forgets which checkouts have already read it.
func (c *TTLCache[T]) restore(key string, value T, expiresAt time.Time, seen map[string]string, stamp Stamp) {
	c.mu.Lock()
	defer c.mu.Unlock()

	merged := make(map[string]string, len(seen)+1)
	for scope, fingerprint := range seen {
		merged[scope] = fingerprint
	}

	if stamp.Fingerprint != "" {
		merged[stamp.Scope] = stamp.Fingerprint
	}

	c.entries[key] = entry[T]{value: value, expiresAt: expiresAt, seen: merged}
}

// deadline is when an entry written now would expire.
func (c *TTLCache[T]) deadline() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.now().Add(c.ttl)
}

func (c *TTLCache[T]) clock() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.now()
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

func (c *TTLCache[T]) setTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ttl = ttl
}

const (
	defaultTTL   = 5 * time.Minute
	workflowTTL  = 2 * time.Minute
	copierTagTTL = 30 * time.Minute
)

// Package-level caches shared across the app. A cache key names who else may
// read the value: RemoteScope for anything read off the remote, a checkout
// identity for anything read from the object store, and the repo path itself
// only for values that are genuinely per-directory.
var (
	DefaultBranchCICache = newRegisteredTTLCache[*models.DefaultBranchCI](workflowTTL)
	PRCache              = newRegisteredTTLCache[*models.PRInfo](defaultTTL)
	PRListCache          = newRegisteredTTLCache[[]models.PRInfo](defaultTTL)
	PRSearchCache        = newRegisteredTTLCache[[]models.PRInfo](defaultTTL)
	PRDetailCache        = newRegisteredTTLCache[*models.PRDetail](defaultTTL)
	BranchCache          = newRegisteredTTLCache[[]models.BranchInfo](defaultTTL)
	CommitCache          = newRegisteredTTLCache[[]models.CommitInfo](defaultTTL)
	WorkflowCache        = newRegisteredTTLCache[*models.WorkflowSummary](workflowTTL)
	MergedPRHeadsCache   = newRegisteredTTLCache[map[string]string](defaultTTL)
	// CopierLatestTagCache is keyed by a template's _src_path rather than by
	// repo path, so every repo generated from the same upstream template
	// shares one lookup instead of each repo hitting the network on its own.
	CopierLatestTagCache = newRegisteredTTLCache[string](copierTagTTL)
)

// BranchCacheKey and CommitCacheKey key on the checkout rather than on the
// object store it borrows. A worktree and its parent read the same refs, but
// both values are relative to whichever HEAD asked: the branch list carries the
// current-branch marker and the log starts at HEAD. Sharing one entry between
// them would either serve one checkout the other's answer or, as the stamp
// makes it, miss every time the cursor alternates.
func BranchCacheKey(repoPath string) string {
	return repoPath + "\x00branches"
}

// CommitCacheKey builds the commit log cache key for a checkout, keyed by depth
// because a deeper log is a different value.
func CommitCacheKey(repoPath string, count int) string {
	return repoPath + "\x00commits:" + strconv.Itoa(count)
}

// ClearAll clears every registered package-level cache, and the installed disk
// store with them.
func ClearAll() {
	registryMu.Lock()
	defer registryMu.Unlock()

	for _, c := range registry {
		c.Clear()
	}

	if d := installedDiskCache(); d != nil {
		d.Clear()
	}
}

// SetAllTTLs overrides every registered cache's entry lifetime. Intended for
// startup config application; existing entries keep their original expiry.
func SetAllTTLs(ttl time.Duration) {
	registryMu.Lock()
	defer registryMu.Unlock()

	for _, c := range registry {
		c.setTTL(ttl)
	}
}
