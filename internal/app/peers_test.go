//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// peerFleet returns two clones of one remote sharing a branch, plus a worktree
// of the first on a branch of its own.
func peerFleet(width, height int) Model {
	now := time.Now()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = width, height
	m.loading = false
	m.summaries = map[string]models.RepoSummary{
		"/dev/app-a": {
			RepoSummary: vcs.RepoSummary{
				Path:         "/dev/app-a",
				VCSType:      vcs.TypeGit,
				Branch:       "main",
				RemoteRepo:   "acme/app",
				Upstream:     "origin/main",
				LastModified: now,
			},
		},
		"/dev/app-b": {
			RepoSummary: vcs.RepoSummary{
				Path:         "/dev/app-b",
				VCSType:      vcs.TypeGit,
				Branch:       "main",
				RemoteRepo:   "acme/app",
				Upstream:     "origin/main",
				Unstaged:     1,
				LastModified: now,
			},
		},
		"/dev/solo": {
			RepoSummary: vcs.RepoSummary{
				Path:         "/dev/solo",
				VCSType:      vcs.TypeGit,
				Branch:       "main",
				RemoteRepo:   "acme/solo",
				LastModified: now,
			},
		},
	}
	m.repoPaths = []string{"/dev/app-a", "/dev/app-b", "/dev/solo"}
	m.selectedRepo = "/dev/app-a"
	m.worktrees = []vcs.WorktreeInfo{
		{Path: "/dev/app-a", Branch: "main"},
		{Path: "/dev/app-a-wt", Branch: "feature/side"},
	}
	m.updateFilteredPaths()

	// app-a has an open PR on "feature-x"; app-b holds it locally under a
	// different name, tracking the same upstream ref, so it counts as a
	// relevant peer regardless of the branch name it checked it out under.
	m.prMap = map[string]PRMapLoadedMsg{
		"/dev/app-a": {
			Path: "/dev/app-a",
			PRs:  []forge.PullRequest{{Number: 1, HeadRef: "feature-x"}},
		},
	}
	m.peerBranches = map[string][]vcs.BranchInfo{
		"/dev/app-b": {{Name: "renamed-branch", Upstream: "origin/feature-x"}},
	}

	return m
}

func TestConflictingBranchesCountsTheRepoItself(t *testing.T) {
	t.Parallel()

	own := models.PeerCheckout{Path: "/dev/app-a", Branch: "feature/side"}
	peers := []models.PeerCheckout{
		{Path: "/dev/app-b", Branch: "main"},
		{Path: "/dev/app-a-wt", Branch: "feature/side"},
	}

	conflicts := models.ConflictingBranches(&own, peers)
	if !conflicts["feature/side"] {
		t.Error("two checkouts on one feature branch is the conflict this flag exists for")
	}

	if conflicts["main"] {
		t.Error("a branch held by exactly one checkout is not a conflict")
	}

	if got := len(models.ConflictingBranches(nil, peers)); got != 0 {
		t.Errorf("an unknown own branch produced %d conflicts, want 0", got)
	}
}

func TestConflictingBranchesExemptsAnIdleDefaultBranch(t *testing.T) {
	t.Parallel()

	idle := models.PeerCheckout{Path: "/dev/app-b", Branch: mainBranchName}
	own := models.PeerCheckout{Path: "/dev/app-a", Branch: mainBranchName}

	if models.ConflictingBranches(&own, []models.PeerCheckout{idle})[mainBranchName] {
		t.Error("backup clones parked on main are ordinary, not a conflict")
	}

	own.Ahead = 1
	if !models.ConflictingBranches(&own, []models.PeerCheckout{idle})[mainBranchName] {
		t.Error("unpushed commits on a shared main are invisible to the other checkout")
	}

	own.Ahead = 0
	idle.Dirty = true
	if !models.ConflictingBranches(&own, []models.PeerCheckout{idle})[mainBranchName] {
		t.Error("uncommitted work on a shared main is the same hazard")
	}
}

func TestPeersCellFlagsSharedBranches(t *testing.T) {
	t.Parallel()

	m := peerFleet(140, 35)

	shared, _ := m.peersCell("/dev/app-a", plainStyle, false)
	if shared != "⧉ 1"+conflictMark {
		t.Errorf("peers cell = %q, want %q", shared, "⧉ 1"+conflictMark)
	}

	alone, _ := m.peersCell("/dev/solo", plainStyle, false)
	if alone != emDash {
		t.Errorf("a repo with no peers renders %q, want %q", alone, emDash)
	}
}

func TestCompactRecordCarriesTheConflictMark(t *testing.T) {
	t.Parallel()

	m := peerFleet(70, 24)
	layout := table.Fit(compactColSpecs, 70)

	shared := plainText(m.renderCompactRow(m.summaries["/dev/app-a"], false, layout))
	if !strings.Contains(shared, peerPrefix+"1 "+conflictMark) {
		t.Errorf("compact record drops the conflict the wide PEERS cell shows:\n%s", shared)
	}

	alone := plainText(m.renderCompactRow(m.summaries["/dev/solo"], false, layout))
	if strings.Contains(alone, conflictMark) {
		t.Errorf("a repo with no peers has nothing to conflict with:\n%s", alone)
	}
}

func TestCheckoutKindDoesNotDependOnWhoIsLooking(t *testing.T) {
	t.Parallel()

	worktree := models.RepoSummary{
		RepoSummary: vcs.RepoSummary{Path: "/dev/app-wt", RemoteRepo: "acme/app", ParentPath: "/dev/app"},
	}
	clone := models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: "/dev/app-b", RemoteRepo: "acme/app"}}
	unrelated := models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: "/dev/app-c", RemoteRepo: "acme/app"}}

	peers := models.FindPeerCheckouts(&unrelated, []models.RepoSummary{worktree, clone})
	kinds := map[string]string{}
	for i := range peers {
		kinds[peers[i].Folder()] = peers[i].Kind()
	}

	if kinds["app-wt"] != "worktree" {
		t.Errorf("app-wt reads as %q from an unrelated clone, want worktree", kinds["app-wt"])
	}
	if kinds["app-b"] != "clone" {
		t.Errorf("app-b reads as %q, want clone", kinds["app-b"])
	}
}

func TestFleetHeaderCountsBranchConflicts(t *testing.T) {
	t.Parallel()

	m := peerFleet(140, 35)
	if got := m.BranchConflictCount(); got != 2 {
		t.Errorf("counted %d repos in conflict, want 2 (app-a and app-b share main)", got)
	}

	header := plainText(m.renderRepoListBreadcrumbs())
	if !strings.Contains(header, conflictMark+" 2 branch conflicts") {
		t.Errorf("fleet header does not report the hazard: %q", header)
	}
}

func TestCheckoutListNamesKindAndConflict(t *testing.T) {
	t.Parallel()

	m := peerFleet(160, 40)
	rendered := plainText(renderPanel(m, panelPeers))

	for _, want := range []string{"app-a-wt", "worktree", "feature/side", "app-b", "clone", "main " + conflictMark} {
		if !strings.Contains(rendered, want) {
			t.Errorf("checkout list is missing %q:\n%s", want, rendered)
		}
	}

	if strings.Contains(rendered, "/dev/app-a  ") {
		t.Error("the repo's own working directory is not one of its parallel checkouts")
	}
}

func TestEnterJumpsToACheckoutAndEscReturns(t *testing.T) {
	t.Parallel()

	m := peerFleet(140, 35)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelPeers

	// The checkouts sort by folder name: app-a-wt, then app-b.
	m.detailCursor = 1

	jumped, _ := m.handleDetailOpenKey()
	after, ok := jumped.(Model)
	if !ok {
		t.Fatalf("jump returned %T, want Model", jumped)
	}

	if after.selectedRepo != "/dev/app-b" {
		t.Fatalf("jumped to %q, want /dev/app-b", after.selectedRepo)
	}

	if len(after.repoStack) != 1 || after.repoStack[0] != "/dev/app-a" {
		t.Fatalf("repo stack = %v, want the repo the jump came from", after.repoStack)
	}

	back, _ := after.handleBackKey()
	returned, ok := back.(Model)
	if !ok {
		t.Fatalf("back returned %T, want Model", back)
	}

	if returned.selectedRepo != "/dev/app-a" || returned.viewMode != ViewModeRepoDetail {
		t.Errorf("esc landed on %q in %v, want /dev/app-a still in the focused view",
			returned.selectedRepo, returned.viewMode)
	}

	out, _ := returned.handleBackKey()
	final, ok := out.(Model)
	if !ok {
		t.Fatalf("back returned %T, want Model", out)
	}

	if final.viewMode != ViewModeRepoList {
		t.Errorf("a second esc landed in %v, want the fleet list", final.viewMode)
	}
}

func TestBracketOpensTheCheckoutsTab(t *testing.T) {
	t.Parallel()

	m := peerFleet(140, 35)
	m.cursor = 0

	next, _ := m.handleKey(tea.KeyPressMsg{Code: ']', Text: "]"})
	opened, ok := next.(Model)
	if !ok {
		t.Fatalf("] returned %T, want Model", next)
	}

	if opened.viewMode != ViewModeRepoDetail || opened.focusedPanel != panelPeers {
		t.Errorf("] landed in %v on tab %v, want the focused view's checkouts tab",
			opened.viewMode, opened.focusedPanel)
	}
}
