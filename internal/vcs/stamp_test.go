package vcs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

const (
	oidMain    = "1111111111111111111111111111111111111111"
	oidPushed  = "2222222222222222222222222222222222222222"
	oidFeature = "3333333333333333333333333333333333333333"
)

// writeGitRepo lays out the parts of a .git directory a stamp reads: HEAD, the
// branch it names, the remote-tracking ref of its upstream, and the config
// that ties the two together.
func writeGitRepo(tb testing.TB, path string) {
	tb.Helper()

	gitDir := filepath.Join(path, ".git")
	mkdir(tb, filepath.Join(gitDir, "refs", "heads"))
	mkdir(tb, filepath.Join(gitDir, "refs", "remotes", "origin"))

	writeFile(tb, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/main\n")
	writeFile(tb, filepath.Join(gitDir, "refs", "heads", "main"), oidMain+"\n")
	writeFile(tb, filepath.Join(gitDir, "refs", "remotes", "origin", "main"), oidMain+"\n")
	writeFile(tb, filepath.Join(gitDir, "config"), strings.Join([]string{
		"[remote \"origin\"]",
		"\turl = git@github.com:acme/app.git",
		"[branch \"main\"]",
		"\tremote = origin",
		"\tmerge = refs/heads/main",
		"",
	}, "\n"))
}

func write(path, contents string) error {
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

func TestStampMovesWithEveryLocalChangeThatMattersToACache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		change  func(repo string) error
		differs bool
	}{
		{"an untouched checkout stamps the same", func(string) error { return nil }, false},
		{"a write outside the git dir changes nothing", func(repo string) error {
			return write(filepath.Join(repo, "notes.txt"), "scratch")
		}, false},
		{"a commit moves HEAD's OID", func(repo string) error {
			return write(filepath.Join(repo, ".git", "refs", "heads", "main"), oidFeature+"\n")
		}, true},
		{"a push moves the remote-tracking ref", func(repo string) error {
			return write(filepath.Join(repo, ".git", "refs", "remotes", "origin", "main"), oidPushed+"\n")
		}, true},
		{"a switch changes the branch HEAD names", func(repo string) error {
			if err := write(filepath.Join(repo, ".git", "refs", "heads", "feature"), oidFeature+"\n"); err != nil {
				return err
			}

			return write(filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature\n")
		}, true},
		{"a new branch shows in the refs tree", func(repo string) error {
			return write(filepath.Join(repo, ".git", "refs", "heads", "spike"), oidFeature+"\n")
		}, true},
		{"a fetch writes FETCH_HEAD", func(repo string) error {
			return write(filepath.Join(repo, ".git", "FETCH_HEAD"), oidPushed+"\tbranch 'main' of acme/app\n")
		}, true},
		{"a pack of the refs replaces them", func(repo string) error {
			return write(filepath.Join(repo, ".git", "packed-refs"),
				"# pack-refs with: peeled fully-peeled sorted \n"+oidMain+" refs/heads/main\n")
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := filepath.Join(t.TempDir(), "app")
			mkdir(t, repo)
			writeGitRepo(t, repo)

			before := vcs.Stamp(repo)
			if before == cache.NoStamp {
				t.Fatal("a git checkout produced no stamp")
			}

			if err := tt.change(repo); err != nil {
				t.Fatal(err)
			}

			after := vcs.Stamp(repo)
			if (after != before) != tt.differs {
				t.Errorf("stamp differs = %v, want %v", after != before, tt.differs)
			}
			if after.Scope != repo {
				t.Errorf("stamp scope = %q, want the checkout identity %q", after.Scope, repo)
			}
		})
	}
}

// packed-refs is the only place a ref lives once it has been packed, so HEAD
// has to resolve through it rather than reporting nothing.
func TestStampResolvesHeadThroughPackedRefs(t *testing.T) {
	t.Parallel()

	repo := filepath.Join(t.TempDir(), "app")
	mkdir(t, repo)
	writeGitRepo(t, repo)

	if err := os.Remove(filepath.Join(repo, ".git", "refs", "heads", "main")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, ".git", "packed-refs"),
		"# pack-refs with: peeled fully-peeled sorted \n"+oidPushed+" refs/heads/main\n")

	if got := vcs.Stamp(repo); !strings.Contains(got.Fingerprint, oidPushed) {
		t.Errorf("packed HEAD OID missing from the fingerprint %q", got.Fingerprint)
	}
}

func TestStampReadsAWorktreesOwnHeadAndItsParentsRefs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parent := filepath.Join(root, "app")
	worktree := filepath.Join(root, "app-wt")

	mkdir(t, parent)
	writeGitRepo(t, parent)

	wtDir := filepath.Join(parent, ".git", "worktrees", "app-wt")
	mkdir(t, wtDir)
	mkdir(t, worktree)
	writeFile(t, filepath.Join(worktree, ".git"), "gitdir: "+wtDir+"\n")
	writeFile(t, filepath.Join(wtDir, "HEAD"), "ref: refs/heads/feature\n")
	writeFile(t, filepath.Join(parent, ".git", "refs", "heads", "feature"), oidFeature+"\n")

	got := vcs.Stamp(worktree)
	if got.Scope != parent {
		t.Errorf("worktree scope = %q, want its parent %q", got.Scope, parent)
	}
	if !strings.Contains(got.Fingerprint, oidFeature) {
		t.Errorf("worktree's own HEAD missing from the fingerprint %q", got.Fingerprint)
	}
	if got == vcs.Stamp(parent) {
		t.Error("a worktree on another branch stamped the same as its parent")
	}
}

// A jj workspace with no colocated .git exposes no git refs, and a stamp that
// cannot see a commit land must not claim the checkout is unchanged.
func TestStampIsEmptyWithoutGitRefsToRead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspace := filepath.Join(root, "app-ws")
	mkdir(t, filepath.Join(workspace, ".jj", "repo", "store", "git"))

	for _, path := range []string{workspace, filepath.Join(root, "gone")} {
		if got := vcs.Stamp(path); got != cache.NoStamp {
			t.Errorf("Stamp(%q) = %+v, want NoStamp", path, got)
		}
	}
}

func BenchmarkStamp(b *testing.B) {
	repo := filepath.Join(b.TempDir(), "app")
	mkdir(b, repo)
	writeGitRepo(b, repo)

	b.ResetTimer()

	for range b.N {
		vcs.Stamp(repo)
	}
}
