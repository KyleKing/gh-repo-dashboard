//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// mapFleet returns a model with the fleet map loaded for two repos: one whose
// PR head ref is checked out here, one whose head ref lives in a peer, plus a
// local branch with commits and no PR and a branch level with its remote.
func mapFleet() Model {
	m := New([]string{"/dev"}, 1)
	m.width, m.height = 160, 40
	m.loading = false
	m.viewMode = ViewModePRMap
	m.summaries = map[string]models.RepoSummary{
		"/dev/app":    {Path: "/dev/app", Branch: "main", RemoteRepo: "acme/app", Upstream: "origin/main"},
		"/dev/app-wt": {Path: "/dev/app-wt", Branch: "feature/peer", RemoteRepo: "acme/app"},
		"/dev/lib":    {Path: "/dev/lib", Branch: "main", RemoteRepo: "acme/lib", Upstream: "origin/main"},
	}
	m.repoPaths = []string{"/dev/app", "/dev/app-wt", "/dev/lib"}
	m.updateFilteredPaths()

	m.prMap = map[string]PRMapLoadedMsg{
		"/dev/app": {
			Path: "/dev/app",
			PRs: []models.PRInfo{
				{Number: 7, Title: "Add the login flow", State: "OPEN", HeadRef: "feature/login"},
				{Number: 9, Title: "Bump a dependency", State: "OPEN", HeadRef: "dependabot/bump"},
				{Number: 8, Title: "Wire the peer branch", State: "OPEN", HeadRef: "feature/peer"},
			},
			Branches: []models.BranchInfo{
				{Name: "main"},
				{Name: "feature/login", Ahead: 2},
				{Name: "feature/orphan", Ahead: 3},
			},
		},
		"/dev/app-wt": {Path: "/dev/app-wt"},
		"/dev/lib":    {Path: "/dev/lib", Branches: []models.BranchInfo{{Name: "main"}}},
	}

	// app-wt is app's peer; pre-warm its branch list so reopening the region
	// costs no fetch.
	m.peerBranches = map[string][]models.BranchInfo{
		"/dev/app-wt": {{Name: "feature/peer"}},
	}

	return m
}

func TestPRMapLocatesHeadRefs(t *testing.T) {
	t.Parallel()

	located := map[int]string{}
	orphans := []string{}

	fleet := mapFleet()
	for _, entry := range fleet.buildPRMap() {
		if entry.HasPR() {
			located[entry.PR.Number] = entry.Location
			continue
		}

		orphans = append(orphans, entry.Branch)
	}

	tests := map[int]string{
		7: "here: feature/login",
		8: "peer: app-wt",
		9: emDash,
	}

	for number, want := range tests {
		if got := located[number]; got != want {
			t.Errorf("PR #%d located at %q, want %q", number, got, want)
		}
	}

	if len(orphans) != 1 || orphans[0] != "feature/orphan" {
		t.Errorf("local-only branches = %v, want just feature/orphan; a branch level with its remote "+
			"has nothing to push and a branch with a PR is not orphaned", orphans)
	}
}

func TestPRMapForkPRDoesNotClaimALocalBranch(t *testing.T) {
	t.Parallel()

	m := mapFleet()
	m.prMap["/dev/lib"] = PRMapLoadedMsg{
		Path: "/dev/lib",
		PRs: []models.PRInfo{
			{Number: 63, Title: "Fix the parser", State: "OPEN", HeadRef: "patch-1", HeadRepoOwner: "camoz"},
		},
		Branches: []models.BranchInfo{{Name: "patch-1", Ahead: 4}},
	}

	var pr, local prMapEntry
	for _, entry := range m.buildPRMap() {
		if entry.Repo != "lib" {
			continue
		}
		if entry.HasPR() {
			pr = entry
		} else {
			local = entry
		}
	}

	if pr.Location != "fork: camoz" || pr.HasLocal {
		t.Errorf("fork PR located at %q (local=%v), want the fork named and no local claim",
			pr.Location, pr.HasLocal)
	}
	if pr.Branch != "camoz:patch-1" {
		t.Errorf("fork PR branch = %q, want it qualified by the fork owner", pr.Branch)
	}
	if local.Branch != "patch-1" {
		t.Errorf("local patch-1 = %q, want the fork's PR not to have claimed it", local.Branch)
	}
}

func TestPRMapSkipsTheDefaultBranchAndSharedCheckouts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parent := filepath.Join(root, "app")
	worktree := filepath.Join(root, "app-wt")
	for _, dir := range []string{filepath.Join(parent, ".git"), worktree} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	pointer := "gitdir: " + filepath.Join(parent, ".git", "worktrees", "app-wt") + "\n"
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o600); err != nil {
		t.Fatal(err)
	}

	shared := PRMapLoadedMsg{Branches: []models.BranchInfo{
		{Name: mainBranchName, Ahead: 3},
		{Name: "feature/shared", Ahead: 1},
	}}

	m := New([]string{root}, 1)
	m.loading = false
	m.repoPaths = []string{parent, worktree}
	m.summaries = map[string]models.RepoSummary{
		parent:   {Path: parent, RemoteRepo: "acme/app"},
		worktree: {Path: worktree, RemoteRepo: "acme/app"},
	}
	m.updateFilteredPaths()
	m.prMap = map[string]PRMapLoadedMsg{
		parent:   {Path: parent, Branches: shared.Branches},
		worktree: {Path: worktree, Branches: shared.Branches},
	}

	branches := make([]string, 0, 2)
	for _, entry := range m.buildPRMap() {
		branches = append(branches, entry.Repo+"/"+entry.Branch)
	}

	want := []string{"app/feature/shared"}
	if strings.Join(branches, " ") != strings.Join(want, " ") {
		t.Errorf("map rows = %v, want %v; a worktree sees its parent's refs and main ahead of "+
			"origin is not a local-only branch", branches, want)
	}
}

func TestPRMapSortsByRepoThenDescendingPR(t *testing.T) {
	t.Parallel()

	fleet := mapFleet()
	entries := fleet.buildPRMap()

	order := make([]string, 0, len(entries))
	for _, entry := range entries {
		label := entry.Repo + "/" + entry.Branch
		if entry.HasPR() {
			label = entry.Repo + "#" + entry.PR.HeadRef
		}

		order = append(order, label)
	}

	want := []string{
		"app#dependabot/bump",
		"app#feature/peer",
		"app#feature/login",
		"app/feature/orphan",
	}

	if got := strings.Join(order, " "); got != strings.Join(want, " ") {
		t.Errorf("map order = %q, want %q", got, strings.Join(want, " "))
	}
}

func TestPRMapSummaryCounts(t *testing.T) {
	t.Parallel()

	fleet := mapFleet()
	summary := prMapSummary(fleet.buildPRMap())
	for _, want := range []string{"3 open", "2 with local branch", "1 local-only branches"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q is missing %q", summary, want)
		}
	}
}

func TestPRMapRendersEveryColumn(t *testing.T) {
	t.Parallel()

	rendered := plainText(mapFleet().renderPRMap())
	want := []string{"REPO", "#7", "Add the login flow", localOnlyLabel, "peer: app-wt", "feature/orphan"}
	for _, want := range want {
		if !strings.Contains(rendered, want) {
			t.Errorf("fleet map is missing %q:\n%s", want, rendered)
		}
	}
}

func TestPRMapEnterOpensTheRepoBehindTheRow(t *testing.T) {
	t.Parallel()

	m := mapFleet()
	m.prMapCursor = 0

	next, _ := m.openPRMapRepo(m.buildPRMap())
	opened, ok := next.(Model)
	if !ok {
		t.Fatalf("enter returned %T, want Model", next)
	}

	if opened.selectedRepo != "/dev/app" || opened.viewMode != ViewModeRepoDetail {
		t.Errorf("enter landed on %q in %v, want /dev/app in the focused view",
			opened.selectedRepo, opened.viewMode)
	}

	if _, ok := opened.prMap["/dev/app"]; !ok {
		t.Error("drilling into a repo should keep what the map read, for the region to reuse")
	}
}

// TestExpandCacheSurvivesThePRMap pins the interaction between the expanded
// region and the fleet map, which share m.prMap: the region caches a repo's
// branches and pull requests there so reopening it costs nothing, and the map
// used to throw that away on both entry and exit.
func TestExpandCacheSurvivesThePRMap(t *testing.T) {
	t.Parallel()
	m := mapFleet()
	m.viewMode = ViewModeRepoList
	cached := m.prMap["/dev/app"]

	m, _ = m.openPRMap()
	if _, ok := m.prMap["/dev/app"]; !ok {
		t.Error("opening the fleet map dropped what the region had already read")
	}

	m = afterUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.viewMode != ViewModeRepoList {
		t.Fatalf("esc should return to the repo list, got %v", m.viewMode)
	}

	got, ok := m.prMap["/dev/app"]
	if !ok {
		t.Fatal("leaving the fleet map dropped the region's cache")
	}
	if len(got.PRs) != len(cached.PRs) {
		t.Errorf("expected %d cached PRs to survive, got %d", len(cached.PRs), len(got.PRs))
	}

	m.expandOpen = true
	m.cursor = 0
	if cmd := m.expandCmd(); cmd != nil {
		t.Error("reopening the region on a cached repo should cost no fetch")
	}
}
