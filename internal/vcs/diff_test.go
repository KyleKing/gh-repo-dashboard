package vcs_test

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// A patch rendered by a viewer has to go through git's own external-diff
// machinery, since that is what hands the viewer the two blobs to compare.
func TestGitStashDiffExternalAsksGitForAnExtDiff(t *testing.T) {
	t.Parallel()

	const key = "git stash show --patch --ext-diff stash@{2}"

	ctx := stubCommands(t, map[string]string{key: "rendered patch\n"}, nil)

	got, err := vcs.NewGitOperations().StashDiffExternal(ctx, testRepoPath, 2, 60, "difft")
	if err != nil {
		t.Fatal(err)
	}
	if want := "rendered patch"; got != want {
		t.Errorf("patch = %q; want %q", got, want)
	}
}

//nolint:paralleltest // the override is process-wide, set once at startup
func TestGitExternalDiffCommandPrefersTheOverride(t *testing.T) {
	const key = "git config --get diff.external"

	ctx := stubCommands(t, map[string]string{key: "difft"}, nil)
	g := vcs.NewGitOperations()

	if got := g.ExternalDiffCommand(ctx, testRepoPath); got != "difft" {
		t.Errorf("command = %q; want the one git config names", got)
	}

	vcs.SetExternalDiffCommand("delta --paging=never")
	t.Cleanup(func() { vcs.SetExternalDiffCommand("") })

	if want := "delta --paging=never"; g.ExternalDiffCommand(ctx, testRepoPath) != want {
		t.Errorf("the configured override lost to git config, want %q", want)
	}
}
