//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"testing"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func relevantPeersFleet() Model {
	m := New([]string{"/dev/one", "/dev/two"}, 1)
	m.width, m.height = 140, 35
	m.loading = false
	m.summaries = map[string]models.RepoSummary{
		"/dev/one/app": {
			RepoSummary: vcs.RepoSummary{
				Path:       "/dev/one/app",
				Branch:     "main",
				RemoteRepo: "acme/app",
				Upstream:   "origin/main",
			},
		},
		"/dev/one/app-unrelated": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/one/app-unrelated", Branch: "chore", RemoteRepo: "acme/app"},
		},
		"/dev/one/app-current": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/one/app-current", Branch: "feature-a", RemoteRepo: "acme/app"},
		},
		"/dev/one/app-other-branch": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/one/app-other-branch", Branch: "main", RemoteRepo: "acme/app"},
		},
		"/dev/two/app-fork": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/two/app-fork", Branch: "feature-b", RemoteRepo: "acme/app"},
		},
		"/dev/two/app-elsewhere": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/two/app-elsewhere", Branch: "main", RemoteRepo: "acme/app"},
		},
	}
	m.repoPaths = []string{
		"/dev/one/app", "/dev/one/app-unrelated", "/dev/one/app-current",
		"/dev/one/app-other-branch", "/dev/two/app-fork", "/dev/two/app-elsewhere",
	}
	m.updateFilteredPaths()

	m.prMap = map[string]PRMapLoadedMsg{
		"/dev/one/app": {
			Path: "/dev/one/app",
			PRs: []forge.PullRequest{
				{Number: 1, HeadRef: "feature-a"},
				{Number: 2, HeadRef: "feature-b", HeadRepoOwner: "someone-else"},
			},
		},
	}
	m.peerBranches = map[string][]vcs.BranchInfo{
		// only unrelated local branches: not relevant
		"/dev/one/app-unrelated": {{Name: "chore", Upstream: "origin/chore"}},
		// the PR's branch exists locally but is not the current checkout: still relevant
		"/dev/one/app-current": {
			{Name: "feature-a", Upstream: "origin/feature-a"},
			{Name: "main", Upstream: "origin/main"},
		},
		// branch name matches PR #2's head ref, but that PR is from a fork
		"/dev/two/app-fork": {{Name: "feature-b", Upstream: "origin/feature-b"}},
		// discovered under a different scan root, but genuinely relevant
		"/dev/two/app-elsewhere": {{Name: "renamed", Upstream: "origin/feature-a"}},
	}

	return m
}

func TestRelevantPeers_IncludesNonCurrentLocalBranchMatch(t *testing.T) {
	t.Parallel()

	m := relevantPeersFleet()
	peers := m.relevantPeers("/dev/one/app")

	found := false
	for _, peer := range peers {
		if peer.Path == "/dev/one/app-current" {
			found = true
			if peer.PR.Number != 1 {
				t.Errorf("expected match against PR #1, got #%d", peer.PR.Number)
			}
		}
	}
	if !found {
		t.Error("expected app-current to be relevant via its non-current feature-a branch")
	}
}

func TestRelevantPeers_ExcludesUnrelatedAndForkBranches(t *testing.T) {
	t.Parallel()

	m := relevantPeersFleet()
	peers := m.relevantPeers("/dev/one/app")

	for _, peer := range peers {
		if peer.Path == "/dev/one/app-unrelated" {
			t.Error("app-unrelated has no branch tracking an open PR and should not be relevant")
		}
		if peer.Path == "/dev/two/app-fork" {
			t.Error("app-fork's branch matches a fork PR by name only and should not be relevant")
		}
	}
}

func TestRelevantPeers_MissingBranchListIsSkippedNotIrrelevant(t *testing.T) {
	t.Parallel()

	m := relevantPeersFleet()
	peers := m.relevantPeers("/dev/one/app")

	for _, peer := range peers {
		if peer.Path == "/dev/one/app-other-branch" {
			t.Error("app-other-branch's branch list never loaded and should be left out, not flagged relevant")
		}
	}
}

func TestRelevantPeers_FlagsAPeerFoundUnderADifferentScanRoot(t *testing.T) {
	t.Parallel()

	m := relevantPeersFleet()
	peers := m.relevantPeers("/dev/one/app")

	for _, peer := range peers {
		want := peer.Path == "/dev/two/app-elsewhere"
		if peer.OtherScanRoot != want {
			t.Errorf("peer %s: OtherScanRoot = %v, want %v", peer.Path, peer.OtherScanRoot, want)
		}
	}
}
