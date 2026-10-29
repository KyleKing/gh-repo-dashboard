// Package snapshot keeps the last row the list painted for each checkout, so
// a relaunch draws it at once and refreshes behind it.
//
// A row lives in its upstream's own cache file under the checkout's key, so
// two checkouts of one remote keep sharing every remote-derived read while
// each holds the local half of its own row. A checkout with no resolvable
// remote gets no record: its whole row reads back in the time a disk lookup
// would take anyway. Rows hold pull request titles and repo paths, never
// bodies, which is what the disk store already keeps at 0600 under 0700.
package snapshot

import (
	"time"

	"github.com/kyleking/aragonite/cache"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// schema is bumped whenever Row gains, loses, or renames a field: a record
// written under another version reads as absent.
const schema = 1

// memoryTTL only decides what counts as recent in memory. Disk keeps a record
// for the store's own stale retention, since the point of a snapshot is to
// paint whatever was last known however long ago.
const memoryTTL = 24 * time.Hour

// Row is one list row as it last painted: the summary the checkout read of
// itself with the remote-derived fields the fetchers filled in, plus the
// counts the list keeps beside the summary.
type Row struct {
	Version      int                `json:"version"`
	Summary      models.RepoSummary `json:"summary"`
	Due          []string           `json:"due,omitempty"`
	ChangedPaths []string           `json:"changed_paths,omitempty"`
	BranchCount  int                `json:"branch_count,omitempty"`
}

// Store reads and writes rows through the disk cache it is opened with.
type Store struct {
	disk *cache.DiskCache
	mem  *cache.TTLCache[Row]
}

// Open returns the store writing through disk, which may be nil to keep rows
// for this run alone. The memory half registers so a hard refresh's ClearAll
// drops rows along with every value cache.
func Open(disk *cache.DiskCache) *Store {
	return &Store{disk: disk, mem: cache.NewRegistered[Row](memoryTTL)}
}

// slotKey names a path's entry. The upstream is part of it because the memory
// cache is shared across upstreams: a checkout re-pointed at another remote
// must not read the record the old remote's file never held.
func slotKey(remoteID, path string) string {
	return remoteID + "\x00row\x00" + path
}

// Load returns path's last row under the upstream remoteID names and how long
// ago it was written. A record written under another schema, or under a
// checkout whose stamp has since moved, reads as absent.
func (s *Store) Load(remoteID, path string, stamp cache.Stamp) (Row, time.Duration, bool) {
	if s == nil || remoteID == "" {
		return Row{}, 0, false
	}

	row, age, ok := cache.NewSnapshot(s.mem, s.disk, remoteID, slotKey(remoteID, path)).Load(stamp)
	if !ok || row.Version != schema {
		return Row{}, 0, false
	}

	return row, age, true
}

// Save records path's row under the upstream remoteID names, stamped with
// what the checkout looked like.
func (s *Store) Save(remoteID, path string, stamp cache.Stamp, row Row) {
	if s == nil || remoteID == "" {
		return
	}

	row.Version = schema
	cache.NewSnapshot(s.mem, s.disk, remoteID, slotKey(remoteID, path)).Save(stamp, row)
}
