//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

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

	if opened.prMap != nil {
		t.Error("leaving the map should release its per-repo data")
	}
}
