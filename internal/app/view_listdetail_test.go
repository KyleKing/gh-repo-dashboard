//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestRepoLabelNamesTheLowercasedRemoteOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		remote string
		want   string
	}{
		{"ssh", "git@github.com:KyleKing/proj.git", "kyleking/proj"},
		{"https", "https://github.com/coverbasedev/proj", "coverbasedev/proj"},
		{"no remote", "", "work/proj"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "work", "proj")
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}

			git := func(args ...string) {
				t.Helper()

				//nolint:gosec // #nosec G204 -- fixed binary, arguments written in this test
				cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", path}, args...)...)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}

			git("init", "-q")
			git("-c", "user.name=Dev", "-c", "user.email=dev@example.com", "commit", "-q", "--allow-empty", "-m", "x")

			if tc.remote != "" {
				git("remote", "add", "origin", tc.remote)
			}

			summary, err := models.ReadSummary(context.Background(), vcs.GetOperations(path), path)
			if err != nil {
				t.Fatal(err)
			}

			m := New(nil, 1)
			m.summaries[path] = summary

			if got := m.repoLabel(path); got != tc.want {
				t.Errorf("repoLabel = %q, want %q (RemoteRepo %q)", got, tc.want, summary.RemoteRepo)
			}
		})
	}
}
