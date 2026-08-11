package cache

import "time"

// NewTTLCacheWithClock returns a TTLCache reading the current time from now,
// so a test can age an entry past its TTL without sleeping.
func NewTTLCacheWithClock[T any](ttl time.Duration, now func() time.Time) *TTLCache[T] {
	c := NewTTLCache[T](ttl)
	c.now = now

	return c
}

// NewSizedDiskCache returns a disk store with a small byte budget, so a test
// can overflow it without writing megabytes.
func NewSizedDiskCache(dir string, maxBytes int64) *DiskCache {
	d := NewDiskCache(dir)
	d.maxBytes = maxBytes

	return d
}

// DiskPath is where store d keeps upstream's cache file.
func DiskPath(d *DiskCache, upstream string) string {
	return d.path(upstream)
}

// PersistUsing and PersistedUsing drive Persist and Persisted against an explicit
// store, so a test works in its own directory instead of the installed one and
// stays parallel-safe.
func PersistUsing[T any](d *DiskCache, c *TTLCache[T], upstream, key string, stamp Stamp, value T) {
	persistTo(d, c, upstream, key, stamp, value)
}

//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
func PersistedUsing[T any](d *DiskCache, c *TTLCache[T], upstream, key string, stamp Stamp) (T, bool) {
	return persistedFrom(d, c, upstream, key, stamp)
}
