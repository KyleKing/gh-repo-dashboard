//go:build golden

package app

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

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
			Branch:       "main",
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
				BaseRef: "main",
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
	m.detailTab = DetailTabBranches
	m.branches = []models.BranchInfo{
		{Name: "main", Upstream: "origin/main", LastCommit: time.Now().Add(-2 * time.Hour)},
		{Name: "feature/login", Upstream: "origin/feature/login", Ahead: 2, LastCommit: time.Now().Add(-10 * time.Minute), IsCurrent: true},
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
