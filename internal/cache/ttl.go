package cache

import (
	"strconv"
	"time"

	acache "github.com/kyleking/aragonite/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
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
	DefaultBranchCICache = newRegisteredTTLCache[*models.DefaultBranchCI](workflowTTL)
	PRCache              = newRegisteredTTLCache[*models.PRInfo](defaultTTL)
	PRListCache          = newRegisteredTTLCache[[]models.PRInfo](defaultTTL)
	PRSearchCache        = newRegisteredTTLCache[[]models.PRInfo](defaultTTL)
	PRDetailCache        = newRegisteredTTLCache[*models.PRDetail](defaultTTL)
	PRPreviewCache       = newRegisteredTTLCache[*models.PRPreview](defaultTTL)
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
