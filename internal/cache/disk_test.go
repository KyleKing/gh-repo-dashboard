package cache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

const (
	upstream = "github.com/kyleking/gh-repo-dashboard"
	listKey  = "prs"
)

func samplePRs() []models.PRInfo {
	return []models.PRInfo{
		{Number: 7, Title: "Cache pull requests on disk", State: "OPEN", HeadRef: "cache", Checks: models.ChecksStatus{
			Total: 2, Passing: 2,
		}},
		{Number: 4, Title: "Stamp the checkout", State: "OPEN", HeadRef: "stamp"},
	}
}

// A second run of the app sees only what the file holds, so the round trip is
// exercised through a fresh store and a fresh memory cache.
func TestPersistedSurvivesAProcessBoundary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stamp := cache.Stamp{Scope: "/repo", Fingerprint: "head-a"}

	cache.PersistUsing(cache.NewDiskCache(dir), cache.NewTTLCache[[]models.PRInfo](time.Hour),
		upstream, listKey, stamp, samplePRs())

	got, ok := cache.PersistedUsing(cache.NewDiskCache(dir), cache.NewTTLCache[[]models.PRInfo](time.Hour),
		upstream, listKey, stamp)
	if !ok {
		t.Fatal("a persisted pull request list was not read back")
	}
	if len(got) != 2 || got[0].Number != 7 || got[0].Title != "Cache pull requests on disk" {
		t.Errorf("read back %+v", got)
	}

	if _, ok := cache.PersistedUsing(cache.NewDiskCache(dir), cache.NewTTLCache[[]models.PRInfo](time.Hour),
		"github.com/other/repo", listKey, stamp); ok {
		t.Error("another upstream read this one's file")
	}
}

// Whatever the file says, a value this build cannot trust is a miss and never
// an error: the entry is refetched rather than decoded into zero values.
func TestUnreadableFilesAreAMiss(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		corrupt func(t *testing.T, path string)
		dropped bool
	}{
		{
			name:    "missing",
			corrupt: func(t *testing.T, path string) { t.Helper(); mustRemove(t, path) },
		},
		{
			name: "version bumped",
			corrupt: func(t *testing.T, path string) {
				t.Helper()
				rewriteJSON(t, path, func(file map[string]any) { file["version"] = 99 })
			},
			dropped: true,
		},
		{
			name: "truncated",
			corrupt: func(t *testing.T, path string) {
				t.Helper()
				data := mustRead(t, path)
				mustWrite(t, path, data[:len(data)/2])
			},
			dropped: true,
		},
		{
			name:    "garbage",
			corrupt: func(t *testing.T, path string) { t.Helper(); mustWrite(t, path, []byte("\x00not json at all")) },
			dropped: true,
		},
		{
			name: "written for another upstream",
			corrupt: func(t *testing.T, path string) {
				t.Helper()
				rewriteJSON(t, path, func(file map[string]any) { file["upstream"] = "github.com/other/repo" })
			},
			dropped: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			store := cache.NewDiskCache(dir)
			path := cache.DiskPath(store, upstream)

			cache.PersistUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour),
				upstream, listKey, cache.NoStamp, samplePRs())
			tc.corrupt(t, path)

			got, ok := cache.PersistedUsing(cache.NewDiskCache(dir), cache.NewTTLCache[[]models.PRInfo](time.Hour),
				upstream, listKey, cache.NoStamp)
			if ok || got != nil {
				t.Fatalf("a %s file was served: %+v", tc.name, got)
			}

			if _, err := os.Stat(path); tc.dropped && err == nil {
				t.Error("the file was left in place for the next run to fail on again")
			}
		})
	}
}

// Titles from a private repo are the most the file may ever hold. Nothing that
// carries prose a reader wrote may reach it.
func TestNoBodyReachesDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := cache.NewDiskCache(dir)

	prs := samplePRs()
	prs[0].Title = "titles are fine"
	cache.PersistUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour), upstream, listKey, cache.NoStamp, prs)
	cache.PersistUsing(store, cache.NewTTLCache[map[string]string](time.Hour), upstream, "heads", cache.NoStamp,
		map[string]string{"feature": "deadbeef"})

	written := string(mustRead(t, cache.DiskPath(store, upstream)))
	if !strings.Contains(written, "titles are fine") {
		t.Fatal("the fixture never reached the file, so the rest of this proves nothing")
	}

	for _, persisted := range []any{[]models.PRInfo{}, map[string]string{}} {
		if field, found := prosePayload(reflect.TypeOf(persisted), nil); found {
			t.Errorf("%T can carry prose through %s", persisted, field)
		}
	}
}

// prosePayload walks a persisted type for a field holding text a person wrote
// rather than a name, a state, or a count.
func prosePayload(typ reflect.Type, seen []reflect.Type) (string, bool) {
	for _, visited := range seen {
		if visited == typ {
			return "", false
		}
	}

	seen = append(seen, typ)

	switch typ.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return prosePayload(typ.Elem(), seen)
	case reflect.Struct:
		for i := range typ.NumField() {
			field := typ.Field(i)
			if strings.Contains(strings.ToLower(field.Name), "body") {
				return typ.Name() + "." + field.Name, true
			}

			if nested, found := prosePayload(field.Type, seen); found {
				return typ.Name() + "." + nested, true
			}
		}
	default:
	}

	return "", false
}

// The files hold what a private repository shows its collaborators, so nobody
// else on the machine reads them.
func TestModesMatchThePosture(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "cache")
	if err := os.MkdirAll(dir, 0o777); err != nil { //nolint:gosec // the wide mode is the case under test
		t.Fatal(err)
	}

	store := cache.NewDiskCache(dir)
	cache.PersistUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour),
		upstream, listKey, cache.NoStamp, samplePRs())

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("directory mode = %o; want 700", perm)
	}

	fileInfo, err := os.Stat(cache.DiskPath(store, upstream))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %o; want 600", perm)
	}
}

// watchForTornFiles fails the test if the file at path is ever readable and not
// decodable, which is what a writer that does not publish by rename leaves for
// a concurrent reader to find. It returns the wait for its own goroutine.
func watchForTornFiles(t *testing.T, path string, done <-chan struct{}) func() {
	t.Helper()

	var reader sync.WaitGroup

	reader.Add(1)

	go func() {
		defer reader.Done()

		for {
			select {
			case <-done:
				return
			default:
			}

			data, err := os.ReadFile(path) //nolint:gosec // the file the writers are publishing
			if err != nil {
				continue
			}

			var file map[string]json.RawMessage
			if err := json.Unmarshal(data, &file); err != nil {
				t.Errorf("read a half-written file of %d bytes: %v", len(data), err)

				return
			}
		}
	}()

	return reader.Wait
}

// Two sessions write the same upstream at once. Rename publishes a whole file
// or none, so every read is either a miss or a value that decodes.
func TestConcurrentWritersLeaveNoHalfWrittenFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := cache.DiskPath(cache.NewDiskCache(dir), upstream)

	var writers sync.WaitGroup

	done := make(chan struct{})
	awaitReader := watchForTornFiles(t, path, done)

	for writer := range 8 {
		writers.Add(1)

		go func() {
			defer writers.Done()

			store := cache.NewDiskCache(dir)
			memory := cache.NewTTLCache[[]models.PRInfo](time.Hour)
			prs := []models.PRInfo{{Number: writer, Title: strings.Repeat("t", 512)}}

			for round := range 20 {
				cache.PersistUsing(store, memory, upstream, "prs:"+strconv.Itoa(round), cache.NoStamp, prs)

				got, ok := cache.PersistedUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour),
					upstream, "prs:"+strconv.Itoa(round), cache.NoStamp)
				if ok && (len(got) != 1 || got[0].Title != strings.Repeat("t", 512)) {
					t.Errorf("read a partial value: %+v", got)
				}
			}
		}()
	}

	writers.Wait()
	close(done)
	awaitReader()

	got, ok := cache.PersistedUsing(cache.NewDiskCache(dir), cache.NewTTLCache[[]models.PRInfo](time.Hour),
		upstream, "prs:19", cache.NoStamp)
	if !ok || len(got) != 1 {
		t.Fatalf("the surviving file does not decode: %+v, hit=%v", got, ok)
	}
}

// cache_to_disk = false installs no store, and a run with no store must not
// create so much as a directory.
func TestNoStoreWritesNothing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	memory := cache.NewTTLCache[[]models.PRInfo](time.Hour)

	cache.PersistUsing(nil, memory, upstream, listKey, cache.NoStamp, samplePRs())

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("wrote %d files with persistence off", len(entries))
	}

	if _, ok := memory.Get(listKey, cache.NoStamp); !ok {
		t.Error("persistence off cost the memory cache its entry")
	}
	if _, ok := cache.PersistedUsing(nil, cache.NewTTLCache[[]models.PRInfo](time.Hour),
		upstream, listKey, cache.NoStamp); ok {
		t.Error("a value came back with no store to hold it")
	}
}

// A remoteless checkout shares its values with nobody, so a file per path would
// grow with every repo ever scanned.
func TestAnEmptyUpstreamStaysInMemory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := cache.NewDiskCache(dir)

	cache.PersistUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour), "", listKey, cache.NoStamp, samplePRs())

	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Errorf("wrote %d files for a repo with no upstream (err=%v)", len(entries), err)
	}
}

// A fleet scanned once must not keep its cache forever, and the file the cursor
// keeps landing on is the last one to go.
func TestEvictionDropsTheLeastRecentlyUsed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := cache.NewSizedDiskCache(dir, 2048)
	kept := "github.com/kyleking/kept"

	cache.PersistUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour), kept, listKey, cache.NoStamp, samplePRs())
	ageFile(t, cache.DiskPath(store, kept), time.Now())

	for i := range 12 {
		other := "github.com/kyleking/repo" + strconv.Itoa(i)
		cache.PersistUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour),
			other, listKey, cache.NoStamp, samplePRs())
		ageFile(t, cache.DiskPath(store, other), time.Now().Add(-time.Duration(i+2)*time.Hour))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 13 {
		t.Error("nothing was evicted")
	}

	if _, err := os.Stat(cache.DiskPath(store, kept)); err != nil {
		t.Errorf("the most recently used file was evicted first: %v", err)
	}
}

// A checkout whose stamp moved since it wrote the entry pushed, and a pushed
// branch is exactly when the remote's answer is stale.
func TestAMovedStampMissesOnDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := cache.NewDiskCache(dir)
	written := cache.Stamp{Scope: "/repo", Fingerprint: "head-a"}

	cache.PersistUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour), upstream, listKey, written, samplePRs())

	moved := cache.Stamp{Scope: "/repo", Fingerprint: "head-b"}
	if _, ok := cache.PersistedUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour),
		upstream, listKey, moved); ok {
		t.Error("a checkout that pushed was served the pull request list it wrote before")
	}

	peer := cache.Stamp{Scope: "/peer", Fingerprint: "head-c"}
	if _, ok := cache.PersistedUsing(store, cache.NewTTLCache[[]models.PRInfo](time.Hour),
		upstream, listKey, peer); !ok {
		t.Error("another checkout of the same remote missed on an entry it had never read")
	}
}

// An expired entry is not served, and writing next to it clears it out.
func TestExpiredEntriesAreNeitherServedNorKept(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := cache.NewDiskCache(dir)
	clock := newFakeClock()

	cache.PersistUsing(store, cache.NewTTLCacheWithClock[[]models.PRInfo](5*time.Minute, clock.now),
		upstream, listKey, cache.NoStamp, samplePRs())

	clock.advance(time.Hour)

	if _, ok := cache.PersistedUsing(store, cache.NewTTLCacheWithClock[[]models.PRInfo](5*time.Minute, clock.now),
		upstream, listKey, cache.NoStamp); ok {
		t.Error("an expired entry was served from disk")
	}

	cache.PersistUsing(store, cache.NewTTLCacheWithClock[[]models.PRInfo](5*time.Minute, clock.now),
		upstream, "other", cache.NoStamp, samplePRs())

	var file struct {
		Entries map[string]json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(mustRead(t, cache.DiskPath(store, upstream)), &file); err != nil {
		t.Fatal(err)
	}
	if _, held := file.Entries[listKey]; held {
		t.Error("the expired entry outlived the write that replaced its neighbor")
	}
}

// Refresh is pressed because something looks wrong; a stale pull request state
// left on disk would survive it.
//
//nolint:paralleltest // installs the process-wide store ClearAll drains
func TestClearAllDropsTheInstalledDiskCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewDiskCache(dir)

	cache.SetDiskCache(store)
	t.Cleanup(func() { cache.SetDiskCache(nil) })

	cache.Persist(cache.PRListCache, upstream, listKey, cache.NoStamp, samplePRs())
	if _, err := os.Stat(cache.DiskPath(store, upstream)); err != nil {
		t.Fatalf("the installed store never wrote the file: %v", err)
	}

	cache.ClearAll()

	if _, err := os.Stat(cache.DiskPath(store, upstream)); !os.IsNotExist(err) {
		t.Errorf("refresh left the file on disk: %v", err)
	}
	if _, ok := cache.Persisted(cache.PRListCache, upstream, listKey, cache.NoStamp); ok {
		t.Error("refresh served a value from the cache it just cleared")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // a path this test just wrote
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func rewriteJSON(t *testing.T, path string, edit func(file map[string]any)) {
	t.Helper()

	var file map[string]any
	if err := json.Unmarshal(mustRead(t, path), &file); err != nil {
		t.Fatal(err)
	}

	edit(file)

	data, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}

	mustWrite(t, path, data)
}

func ageFile(t *testing.T, path string, used time.Time) {
	t.Helper()

	if err := os.Chtimes(path, used, used); err != nil {
		t.Fatal(err)
	}
}
