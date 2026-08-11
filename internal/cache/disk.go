package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// diskSchemaVersion guards the shape of what is written. Bump it whenever a
// persisted struct gains, loses, or renames a field: a file written under
// another version is dropped rather than decoded into zero values.
const diskSchemaVersion = 1

const (
	diskFileMode   = 0o600
	diskDirMode    = 0o700
	diskMaxBytes   = 4 << 20
	diskFileSuffix = ".json"
)

// DiskCache persists remote-derived cache values between runs, one file per
// upstream identity, so a cold start on a large fleet does not re-issue a
// network call per repo.
//
// What it holds is deliberately narrow: counts, states, numbers, and pull
// request titles. Bodies and comment text stay in memory. Titles from a private
// repository are already more than the app has otherwise put on disk, so the
// files are mode 0600 under a 0700 directory.
//
// Every failure is a miss. A corrupt, truncated, or unreadable file is dropped
// and refetched rather than reported.
type DiskCache struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
}

// NewDiskCache returns a store writing under dir, which is created on first
// write.
func NewDiskCache(dir string) *DiskCache {
	return &DiskCache{dir: dir, maxBytes: diskMaxBytes}
}

// UserDiskCache returns the store under os.UserCacheDir(), which is where a
// cache belongs on every platform the app builds for.
func UserDiskCache() (*DiskCache, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("resolving the user cache directory: %w", err)
	}

	return NewDiskCache(filepath.Join(base, "gh-repo-dashboard")), nil
}

//nolint:gochecknoglobals // the installed store, like the caches it backs
var (
	diskMu    sync.RWMutex
	diskStore *DiskCache
)

// SetDiskCache installs the store Persist writes through and Persisted reads
// back. Nothing is installed until a caller opts in, so a `cache_to_disk =
// false` run (and every test) touches no files at all.
func SetDiskCache(store *DiskCache) {
	diskMu.Lock()
	defer diskMu.Unlock()

	diskStore = store
}

func installedDiskCache() *DiskCache {
	diskMu.RLock()
	defer diskMu.RUnlock()

	return diskStore
}

// diskFile is one upstream's whole cache. The version and the upstream are read
// back before the entries, so a file from another schema or another key is
// dropped instead of decoded.
type diskFile struct {
	Version  int                  `json:"version"`
	Upstream string               `json:"upstream"`
	Entries  map[string]diskEntry `json:"entries"`
}

type diskEntry struct {
	Value     json.RawMessage   `json:"value"`
	ExpiresAt time.Time         `json:"expires_at"`
	Seen      map[string]string `json:"seen"`
}

// Persisted returns the value for key, reading the upstream's cache file when
// the memory cache misses and seeding memory from what it finds. The file is
// read on the miss that needs it rather than at startup, so landing on one row
// costs one small read instead of one per repo on the roster.
//
// The disk entry keeps its own expiry and the fingerprints it was written
// under, so reading it back neither restarts the TTL nor loses the stamp rule.
//
//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
func Persisted[T any](c *TTLCache[T], upstream, key string, stamp Stamp) (T, bool) {
	return persistedFrom(installedDiskCache(), c, upstream, key, stamp)
}

//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
func persistedFrom[T any](d *DiskCache, c *TTLCache[T], upstream, key string, stamp Stamp) (T, bool) {
	if value, ok := c.Get(key, stamp); ok {
		return value, true
	}

	var zero T

	if d == nil || upstream == "" {
		return zero, false
	}

	e, ok := d.read(upstream, key, stamp, c.clock())
	if !ok {
		return zero, false
	}

	var value T
	if err := json.Unmarshal(e.Value, &value); err != nil {
		return zero, false
	}

	c.restore(key, value, e.ExpiresAt, e.Seen, stamp)

	return value, true
}

// Persist stores value in c and, when a store is installed, in upstream's cache
// file. An empty upstream is memory-only: a checkout with no resolvable remote
// shares its values with nobody, and a file per path is the unbounded growth
// the size budget exists to prevent.
func Persist[T any](c *TTLCache[T], upstream, key string, stamp Stamp, value T) {
	persistTo(installedDiskCache(), c, upstream, key, stamp, value)
}

func persistTo[T any](d *DiskCache, c *TTLCache[T], upstream, key string, stamp Stamp, value T) {
	c.Set(key, stamp, value)

	if d == nil || upstream == "" {
		return
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}

	seen := make(map[string]string, 1)
	if stamp.Fingerprint != "" {
		seen[stamp.Scope] = stamp.Fingerprint
	}

	d.write(upstream, key, diskEntry{Value: encoded, ExpiresAt: c.deadline(), Seen: seen}, c.clock())
}

func (d *DiskCache) read(upstream, key string, stamp Stamp, now time.Time) (diskEntry, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	file, ok := d.load(upstream)
	if !ok {
		return diskEntry{}, false
	}

	e, ok := file.Entries[key]
	if !ok || now.After(e.ExpiresAt) {
		return diskEntry{}, false
	}

	if prev, recorded := e.Seen[stamp.Scope]; recorded && stamp.Fingerprint != "" && prev != stamp.Fingerprint {
		return diskEntry{}, false
	}

	d.touch(upstream)

	return e, true
}

func (d *DiskCache) write(upstream, key string, e diskEntry, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	file, ok := d.load(upstream)
	if !ok {
		file = diskFile{Version: diskSchemaVersion, Upstream: upstream, Entries: map[string]diskEntry{}}
	}

	for existing, held := range file.Entries {
		if now.After(held.ExpiresAt) {
			delete(file.Entries, existing)
		}
	}

	file.Entries[key] = e

	if err := d.save(upstream, file); err != nil {
		return
	}

	d.evict()
}

// Clear removes every cache file, leaving the directory in place. Refresh drops
// the disk copy along with the memory one: it is pressed because something
// looks wrong, and a refresh that leaves a stale pull request state behind
// cannot fix it.
func (d *DiskCache) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, f := range d.stat() {
		removeQuietly(f.path)
	}
}

// load reads upstream's file, dropping one this build cannot trust. The caller
// holds d.mu.
func (d *DiskCache) load(upstream string) (diskFile, bool) {
	path := d.path(upstream)

	data, err := os.ReadFile(path) //nolint:gosec // a hashed name under the store's own directory
	if err != nil {
		return diskFile{}, false
	}

	var file diskFile
	if err := json.Unmarshal(data, &file); err != nil ||
		file.Version != diskSchemaVersion || file.Upstream != upstream {
		removeQuietly(path)

		return diskFile{}, false
	}

	if file.Entries == nil {
		file.Entries = map[string]diskEntry{}
	}

	return file, true
}

func (d *DiskCache) save(upstream string, file diskFile) error {
	if err := d.ensureDir(); err != nil {
		return err
	}

	data, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("encoding the cache for %s: %w", upstream, err)
	}

	return writeAtomic(d.dir, d.path(upstream), data)
}

// writeAtomic publishes data by renaming a temp file from the same directory,
// so a reader sees either the old file or the whole new one however many
// sessions write at once. The mode is set before the rename, never after, so
// the contents are never readable at the temp file's own mode.
func writeAtomic(dir, path string, data []byte) error {
	tmp, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return fmt.Errorf("creating a temporary cache file in %s: %w", dir, err)
	}

	name := tmp.Name()

	if err := fillFile(tmp, data); err != nil {
		removeQuietly(name)

		return err
	}

	if err := os.Rename(name, path); err != nil {
		removeQuietly(name)

		return fmt.Errorf("replacing %s: %w", path, err)
	}

	return nil
}

func fillFile(file *os.File, data []byte) error {
	if err := file.Chmod(diskFileMode); err != nil {
		closeQuietly(file)

		return fmt.Errorf("securing %s: %w", file.Name(), err)
	}

	if _, err := file.Write(data); err != nil {
		closeQuietly(file)

		return fmt.Errorf("writing %s: %w", file.Name(), err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", file.Name(), err)
	}

	return nil
}

func closeQuietly(file *os.File) {
	_ = file.Close() //nolint:errcheck // the caller is already returning the failure that matters
}

func (d *DiskCache) ensureDir() error {
	if err := os.MkdirAll(d.dir, diskDirMode); err != nil {
		return fmt.Errorf("creating %s: %w", d.dir, err)
	}

	// MkdirAll leaves an existing directory's mode alone, and a cache directory
	// inherited from an earlier, wider umask must not stay group-readable.
	if err := os.Chmod(d.dir, diskDirMode); err != nil {
		return fmt.Errorf("securing %s: %w", d.dir, err)
	}

	return nil
}

func (d *DiskCache) path(upstream string) string {
	sum := sha256.Sum256([]byte(upstream))

	return filepath.Join(d.dir, hex.EncodeToString(sum[:])+diskFileSuffix)
}

// touch records that the file was used, so eviction orders by last read rather
// than by last write.
func (d *DiskCache) touch(upstream string) {
	now := time.Now()
	//nolint:errcheck // an unrecorded read costs eviction order, nothing a caller can act on
	_ = os.Chtimes(d.path(upstream), now, now)
}

type diskFileInfo struct {
	path string
	size int64
	used time.Time
}

// evict trims the store to its byte budget, least recently used first, so a
// fleet scanned once does not keep its cache forever.
func (d *DiskCache) evict() {
	files := d.stat()

	total := totalBytes(files)
	if total <= d.maxBytes {
		return
	}

	slices.SortFunc(files, func(a, b diskFileInfo) int { return a.used.Compare(b.used) })

	for _, f := range files {
		if total <= d.maxBytes {
			return
		}

		removeQuietly(f.path)
		total -= f.size
	}
}

// stat returns every cache file with its size and last use. Temp files are
// skipped: one belongs to a write still in flight, here or in another session.
func (d *DiskCache) stat() []diskFileInfo {
	dirEntries, err := os.ReadDir(d.dir)
	if err != nil {
		return nil
	}

	var files []diskFileInfo

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || filepath.Ext(dirEntry.Name()) != diskFileSuffix {
			continue
		}

		info, err := dirEntry.Info()
		if err != nil {
			continue
		}

		files = append(files, diskFileInfo{
			path: filepath.Join(d.dir, dirEntry.Name()),
			size: info.Size(),
			used: info.ModTime(),
		})
	}

	return files
}

func totalBytes(files []diskFileInfo) int64 {
	var total int64
	for _, f := range files {
		total += f.size
	}

	return total
}

func removeQuietly(path string) {
	_ = os.Remove(path) //nolint:errcheck // a file the store cannot delete is a file it stops reading
}
