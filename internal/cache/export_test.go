package cache

import "time"

// NewTTLCacheWithClock returns a TTLCache reading the current time from now,
// so a test can age an entry past its TTL without sleeping.
func NewTTLCacheWithClock[T any](ttl time.Duration, now func() time.Time) *TTLCache[T] {
	c := NewTTLCache[T](ttl)
	c.now = now

	return c
}
