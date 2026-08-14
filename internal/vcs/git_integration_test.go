package vcs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func gitInit(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()

		cmd := exec.CommandContext(t.Context(), "git", args...) // #nosec G204 -- args are literals from this test
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	write := func(name, body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	run("init", "--initial-branch=main")
	write("file.txt", "one\n")
	run("add", "file.txt")
	run("commit", "-m", "initial")
	write("file.txt", "two\n")
	run("stash", "push", "-m", "first stash")
	write("file.txt", "three\n")
	run("stash", "push", "-m", "second stash")

	return dir
}

func TestGitGetStashListAgainstRealGit(t *testing.T) {
	t.Parallel()

	dir := gitInit(t)

	stashes, err := vcs.NewGitOperations().GetStashList(t.Context(), dir)
	if err != nil {
		t.Fatalf("GetStashList: %v", err)
	}

	if len(stashes) != 2 {
		t.Fatalf("got %d stashes, want 2: %+v", len(stashes), stashes)
	}

	wantMessages := []string{"On main: second stash", "On main: first stash"}
	for i, want := range wantMessages {
		if stashes[i].Index != i {
			t.Errorf("stash %d: index = %d, want %d", i, stashes[i].Index, i)
		}

		if stashes[i].Message != want {
			t.Errorf("stash %d: message = %q, want %q", i, stashes[i].Message, want)
		}

		if stashes[i].Date.IsZero() {
			t.Errorf("stash %d: date is zero", i)
		}
	}
}

func TestGitGetNewestModifiedFileAgainstRealGit(t *testing.T) {
	t.Parallel()

	dir := gitInit(t)

	// The clean tree left by gitInit has nothing uncommitted to report.
	name, _, err := vcs.NewGitOperations().GetNewestModifiedFile(t.Context(), dir)
	if err != nil {
		t.Fatalf("GetNewestModifiedFile on a clean tree: %v", err)
	}

	if name != "" {
		t.Errorf("clean tree reported newest file %q, want none", name)
	}

	older := time.Now().Add(-time.Hour)
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("edited\n"), 0o600); err != nil {
		t.Fatalf("writing file.txt: %v", err)
	}

	if err := os.Chtimes(filepath.Join(dir, "file.txt"), older, older); err != nil {
		t.Fatalf("backdating file.txt: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "newer.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatalf("writing newer.txt: %v", err)
	}

	name, modTime, err := vcs.NewGitOperations().GetNewestModifiedFile(t.Context(), dir)
	if err != nil {
		t.Fatalf("GetNewestModifiedFile on a dirty tree: %v", err)
	}

	if name != "newer.txt" {
		t.Errorf("newest file = %q, want %q (the untracked file touched after file.txt was backdated)",
			name, "newer.txt")
	}

	if modTime.Before(older) {
		t.Errorf("newest file's mod time %v is not after the backdated file.txt's %v", modTime, older)
	}
}
