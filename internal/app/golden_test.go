//go:build golden

package app

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// breakpointSizes are the three terminal sizes the layout is designed against:
// one per breakpoint, each named for the golden file it produces.
func breakpointSizes() []struct {
	name          string
	width, height int
} {
	return []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"120x35", 120, 35},
		{"220x50", 220, 50},
	}
}

func goldenModel() Model {
	m := New([]string{"/Users/dev"}, 1)
	m.width = 100
	m.height = 30
	m.loading = false

	m.repoPaths = []string{"/Users/dev/alpha", "/Users/dev/bravo", "/Users/dev/charlie"}
	m.summaries = map[string]models.RepoSummary{
		"/Users/dev/alpha": {
			Path:         "/Users/dev/alpha",
			VCSType:      models.VCSTypeGit,
			Branch:       mainBranchName,
			Upstream:     "origin/main",
			LastModified: time.Now().Add(-5 * time.Minute),
		},
		"/Users/dev/bravo": {
			Path:         "/Users/dev/bravo",
			VCSType:      models.VCSTypeGit,
			Branch:       "feature/login",
			Upstream:     "origin/feature/login",
			Ahead:        2,
			Staged:       1,
			Unstaged:     3,
			LastModified: time.Now().Add(-2 * time.Hour),
			PRInfo: &models.PRInfo{
				Number:  42,
				Title:   "Add login flow",
				State:   "OPEN",
				URL:     "https://github.com/dev/bravo/pull/42",
				HeadRef: "feature/login",
				BaseRef: mainBranchName,
			},
		},
		"/Users/dev/charlie": {
			Path:         "/Users/dev/charlie",
			VCSType:      models.VCSTypeJJ,
			Branch:       "trunk",
			Untracked:    1,
			LastModified: time.Now().Add(-3 * 24 * time.Hour),
		},
	}
	m.prCount = map[string]int{"/Users/dev/bravo": 1}
	m.updateFilteredPaths()
	return m
}

func TestGoldenRepoList(t *testing.T) {
	m := goldenModel()
	golden.RequireEqual(t, []byte(m.renderScreen()))
}

// TestGoldenRepoListBreakpoints pins the repo list at the three terminal sizes
// the layout is designed against, so a column-engine change that only shows up
// under collapse or under surplus width still fails here.
func TestGoldenRepoListBreakpoints(t *testing.T) {
	for _, size := range breakpointSizes() {
		t.Run(size.name, func(t *testing.T) {
			m := goldenModel()
			m.width, m.height = size.width, size.height
			golden.RequireEqual(t, []byte(m.renderScreen()))
		})
	}
}

// TestGoldenPanelGridBreakpoints pins the panel grid at each breakpoint with
// focus on each of the two busiest panels, so a sizing or collapse change
// shows up as a frame diff.
func TestGoldenPanelGridBreakpoints(t *testing.T) {
	tabs := []struct {
		name string
		tab  panelID
	}{
		{"branches", panelBranches},
		{"prs", panelPRs},
	}

	for _, size := range breakpointSizes() {
		for _, tab := range tabs {
			t.Run(tab.name+"-"+size.name, func(t *testing.T) {
				m := goldenModel()
				m.width, m.height = size.width, size.height
				m.viewMode = ViewModeRepoDetail
				m.selectedRepo = "/Users/dev/bravo"
				m.focusedPanel = tab.tab
				m.branches = []models.BranchInfo{
					{Name: mainBranchName, Upstream: "origin/main", LastCommit: time.Now().Add(-2 * time.Hour)},
					{
						Name: "feature/login", Upstream: "origin/feature/login", Ahead: 2,
						LastCommit: time.Now().Add(-10 * time.Minute), IsCurrent: true,
					},
				}
				m.prs = []models.PRInfo{
					{Number: 42, Title: "Add login flow", State: "OPEN", HeadRef: "feature/login"},
					{Number: 7, Title: "Bump the template to v0.10.0", State: "OPEN", IsDraft: true, HeadRef: "chore/template"},
				}
				golden.RequireEqual(t, []byte(m.renderScreen()))
			})
		}
	}
}

func TestGoldenFilterModal(t *testing.T) {
	m := goldenModel()
	m.viewMode = ViewModeFilter
	m.filterCursor = 1
	golden.RequireEqual(t, []byte(m.renderScreen()))
}

func TestGoldenRepoDetail(t *testing.T) {
	m := goldenModel()
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/Users/dev/bravo"
	m.focusedPanel = panelBranches
	m.branches = []models.BranchInfo{
		{Name: mainBranchName, Upstream: "origin/main", LastCommit: time.Now().Add(-2 * time.Hour)},
		{Name: "feature/login", Upstream: "origin/feature/login", Ahead: 2, LastCommit: time.Now().Add(-10 * time.Minute), IsCurrent: true},
	}
	golden.RequireEqual(t, []byte(m.renderScreen()))
}

func TestGoldenBranchDetail(t *testing.T) {
	m := goldenModel()
	m.viewMode = ViewModeBranchDetail
	m.selectedRepo = "/Users/dev/bravo"
	m.branches = []models.BranchInfo{
		{Name: mainBranchName, Upstream: "origin/main", LastCommit: time.Now().Add(-2 * time.Hour)},
	}
	m.branchDetail = models.BranchDetail{
		Branch: models.BranchInfo{
			Name:      "feature/login",
			Upstream:  "origin/feature/login",
			Ahead:     2,
			IsCurrent: true,
		},
		Commits: []models.CommitInfo{
			{Hash: "abc1234def", ShortHash: "abc1234", Subject: "Add login flow", Author: "dev", Date: time.Now().Add(-10 * time.Minute)},
			{Hash: "9876543abc", ShortHash: "9876543", Subject: "Wire up session store", Author: "dev", Date: time.Now().Add(-1 * time.Hour)},
		},
		DefaultBranch: mainBranchName,
		DefaultAhead:  2,
		DefaultBehind: 1,
		Staged:        1,
		Unstaged:      2,
		PRInfo: &models.PRInfo{
			Number:  42,
			Title:   "Add login flow",
			State:   "OPEN",
			URL:     "https://github.com/dev/bravo/pull/42",
			HeadRef: "feature/login",
			BaseRef: mainBranchName,
		},
	}
	golden.RequireEqual(t, []byte(m.renderScreen()))
}

func TestGoldenBatchProgress(t *testing.T) {
	m := goldenModel()
	m.viewMode = ViewModeBatchProgress
	m.batchTask = "Fetch All"
	m.batchRunning = false
	m.batchTotal = 3
	m.batchProgress = 3
	m.batchResults = []BatchResult{
		{Path: "/Users/dev/alpha", Success: true, Message: "fetched"},
		{Path: "/Users/dev/bravo", Success: true, Message: "fetched"},
		{Path: "/Users/dev/charlie", Success: false, Message: "no remote configured"},
	}
	golden.RequireEqual(t, []byte(m.renderScreen()))
}
