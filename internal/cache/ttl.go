package cache

import (
	"time"

	acache "github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/forge"
)

type (
	Stamp     = acache.Stamp
	DiskCache = acache.DiskCache
)

type TTLCache[T any] = acache.TTLCache[T]

var NoStamp = acache.NoStamp //nolint:gochecknoglobals // an empty-value constant, never assigned to

var (
	RemoteScope       = acache.RemoteScope
	NewDiskCache      = acache.NewDiskCache
	NewSizedDiskCache = acache.NewSizedDiskCache
	DiskPath          = acache.DiskPath
	UserDiskCache     = acache.UserDiskCache
	SetDiskCache      = acache.SetDiskCache
	ClearAll          = acache.ClearAll
	SetAllTTLs        = acache.SetAllTTLs
)

func NewTTLCache[T any](ttl time.Duration) *acache.TTLCache[T] {
	return acache.NewTTLCache[T](ttl)
}

func NewTTLCacheWithClock[T any](ttl time.Duration, now func() time.Time) *acache.TTLCache[T] {
	return acache.NewTTLCacheWithClock[T](ttl, now)
}

func newRegisteredTTLCache[T any](ttl time.Duration) *acache.TTLCache[T] {
	return acache.NewRegistered[T](ttl)
}

func Persist[T any](c *acache.TTLCache[T], upstream, key string, stamp Stamp, value T) {
	acache.Persist(c, upstream, key, stamp, value)
}

func Persisted[T any](c *acache.TTLCache[T], upstream, key string, stamp Stamp) (T, bool) {
	return acache.Persisted[T](c, upstream, key, stamp)
}

func PersistUsing[T any](d *DiskCache, c *acache.TTLCache[T], upstream, key string, stamp Stamp, value T) {
	acache.PersistUsing(d, c, upstream, key, stamp, value)
}

func PersistedUsing[T any](d *DiskCache, c *acache.TTLCache[T], upstream, key string, stamp Stamp) (T, bool) {
	return acache.PersistedUsing(d, c, upstream, key, stamp)
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
	DefaultBranchCICache = newRegisteredTTLCache[*forge.DefaultBranchCI](workflowTTL)
	PRCache              = newRegisteredTTLCache[*forge.PullRequest](defaultTTL)
	PRListCache          = newRegisteredTTLCache[[]forge.PullRequest](defaultTTL)
	PRSearchCache        = newRegisteredTTLCache[[]forge.PullRequest](defaultTTL)
	PRDetailCache        = newRegisteredTTLCache[*forge.PRDetail](defaultTTL)
	PRPreviewCache       = newRegisteredTTLCache[*forge.PRPreview](defaultTTL)
	WorkflowCache        = newRegisteredTTLCache[*forge.WorkflowSummary](workflowTTL)
	MergedPRHeadsCache   = newRegisteredTTLCache[map[string]string](defaultTTL)
	// CopierLatestTagCache is keyed by a template's _src_path rather than by
	// repo path, so every repo generated from the same upstream template
	// shares one lookup instead of each repo hitting the network on its own.
	CopierLatestTagCache = newRegisteredTTLCache[string](copierTagTTL)
)
