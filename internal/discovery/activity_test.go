package discovery_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
)

func commitAs(t *testing.T, dir, email string, when time.Time) {
	t.Helper()

	stamp := when.Format(time.RFC3339)
	run := func(args ...string) {
		t.Helper()

		//nolint:gosec // #nosec G204 -- fixed binary, arguments written in this test
		cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Dev", "GIT_AUTHOR_EMAIL="+email, "GIT_AUTHOR_DATE="+stamp,
			"GIT_COMMITTER_NAME=Dev", "GIT_COMMITTER_EMAIL="+email, "GIT_COMMITTER_DATE="+stamp,
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "work")
}

func TestFilterActive(t *testing.T) {
	t.Parallel()

	now := time.Now()
	activity := discovery.Activity{
		Since:   now.Add(-180 * 24 * time.Hour),
		Authors: []string{"Kyle.King+work@Example.com", "kyle@example.com"},
	}

	repos := []struct {
		name   string
		email  string
		age    time.Duration
		active bool
	}{
		{"inside", "kyle@example.com", 24 * time.Hour, true},
		{"outside", "kyle@example.com", 400 * 24 * time.Hour, false},
		{"other-author", "someone@example.com", 24 * time.Hour, false},
		{"superstring-email", "notkyle@example.com", 24 * time.Hour, false},
		{"mixed-case-email", "KYLE@EXAMPLE.COM", 24 * time.Hour, true},
		{"plus-address", "kyle.king+work@example.com", 24 * time.Hour, true},
	}

	root := t.TempDir()
	paths := make([]string, len(repos))

	for i, repo := range repos {
		paths[i] = filepath.Join(root, repo.name)
		if err := os.Mkdir(paths[i], 0o750); err != nil {
			t.Fatal(err)
		}

		commitAs(t, paths[i], repo.email, now.Add(-repo.age))
	}

	kept, hidden := discovery.FilterActive(context.Background(), paths, activity, 3)

	var want []string

	for i, repo := range repos {
		if repo.active {
			want = append(want, paths[i])
		}
	}

	if !slices.Equal(kept, want) {
		t.Errorf("kept %v, want %v", kept, want)
	}

	if hidden != len(repos)-len(want) {
		t.Errorf("hidden = %d, want %d", hidden, len(repos)-len(want))
	}
}
