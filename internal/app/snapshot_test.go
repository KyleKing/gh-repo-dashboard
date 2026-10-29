//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/snapshot"
)

// writeRepo fabricates the minimum .git a stamp and a remote identity need:
// HEAD on a branch, the branch's OID, and an origin remote.
func writeRepo(t *testing.T, oid string) string {
	t.Helper()

	path := t.TempDir()
	gitDir := filepath.Join(path, ".git")

	for _, dir := range []string{filepath.Join(gitDir, "refs", "heads")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	write := func(name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(gitDir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("HEAD", "ref: refs/heads/main\n")
	write(filepath.Join("refs", "heads", "main"), oid+"\n")
	write("config", "[remote \"origin\"]\n\turl = git@github.com:test/app.git\n")

	return path
}

func savedModel(t *testing.T, path string) Model {
	t.Helper()

	row := snapshot.Row{
		Summary: models.RepoSummary{
			RepoSummary: vcs.RepoSummary{Branch: "main", Path: path, RemoteID: "github.com/test/app"},
		},
		BranchCount: 4,
	}
	m := New(nil, 0).WithSnapshots(cache.NewDiskCache(t.TempDir()))
	m.snapshots.Save("github.com/test/app", path, vcs.Stamp(path), row)

	return m
}

// A kept row paints again at the next discovery, marked stale where the
// fresh reads will overtake it.
func TestSeedSnapshotPaintsTheLastRow(t *testing.T) {
	t.Parallel()

	path := writeRepo(t, "1111111111111111111111111111111111111111")
	m := savedModel(t, path)

	m.seedSnapshot(path)

	seeded, ok := m.summaries[path]
	if !ok || seeded.Branch != "main" {
		t.Fatalf("summary = %+v, %v", seeded, ok)
	}
	if m.branchCount[path] != 4 {
		t.Errorf("branchCount = %d", m.branchCount[path])
	}
	if m.freshness[fetchKey{path, fetchOpenPRs}] != freshnessStale {
		t.Error("a seeded row did not mark its fetches stale")
	}
}

// A checkout that committed since the row was written is not painted from it.
func TestSeedSnapshotMissesAfterACommit(t *testing.T) {
	t.Parallel()

	path := writeRepo(t, "1111111111111111111111111111111111111111")
	m := savedModel(t, path)

	if err := os.WriteFile(filepath.Join(path, ".git", "refs", "heads", "main"),
		[]byte("2222222222222222222222222222222222222222\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m.seedSnapshot(path)

	if _, ok := m.summaries[path]; ok {
		t.Error("a moved checkout was painted from the row written before it moved")
	}
}

// A checkout with no remote never had a row kept, so nothing paints early.
func TestSeedSnapshotSkipsRemoteless(t *testing.T) {
	t.Parallel()

	path := t.TempDir()
	gitDir := filepath.Join(path, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]\n\tbare = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := New(nil, 0).WithSnapshots(cache.NewDiskCache(t.TempDir()))
	m.seedSnapshot(path)

	if _, ok := m.summaries[path]; ok {
		t.Error("a remoteless checkout painted a row it never had")
	}
}
