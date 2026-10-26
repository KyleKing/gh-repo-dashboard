//go:build golden

package app

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
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
		{"200x50", 200, 50},
	}
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

// fleetGoldenModel is twenty repos, three of which need the viewer: one with
// failing CI and unpushed commits, one dirty, one with a pull request waiting on
// the viewer. Four quiet repos carry signals that show but never promote: a
// stash, a note, template drift, and a due task.
func fleetGoldenModel() Model {
	m := New([]string{"/Users/dev"}, 1)
	m.loading = false
	m.viewer = "kyle"

	names := []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet",
		"kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo", "sierra", "tango",
	}

	for i, name := range names {
		path := "/Users/dev/" + name
		m.repoPaths = append(m.repoPaths, path)
		m.summaries[path] = models.RepoSummary{
			RepoSummary: vcs.RepoSummary{
				Path:         path,
				VCSType:      vcs.TypeGit,
				Branch:       mainBranchName,
				Upstream:     "origin/main",
				LastModified: time.Now().Add(-time.Duration(i+1) * 26 * time.Hour),
			},
		}
	}

	edit := func(name string, mutate func(*models.RepoSummary)) {
		path := "/Users/dev/" + name
		summary := m.summaries[path]
		mutate(&summary)
		m.summaries[path] = summary
	}

	edit("delta", func(s *models.RepoSummary) {
		s.Ahead = 2
		s.CI = &forge.DefaultBranchCI{Workflows: []forge.CIWorkflowRun{
			{Workflow: "test", Status: forge.StatusCompleted, Conclusion: models.CIFailureConclusion},
			{Workflow: "lint", Status: forge.StatusCompleted, Conclusion: "success"},
		}}
	})
	edit("juliet", func(s *models.RepoSummary) {
		s.Branch = "feature/login"
		s.Staged, s.Unstaged, s.Untracked = 1, 2, 1
		s.OpenPRs = []forge.PullRequest{{Number: 9, Author: "kyle", ReviewDecision: "REVIEW_REQUIRED"}}
	})
	edit("mike", func(s *models.RepoSummary) {
		s.OpenPRs = []forge.PullRequest{
			{Number: 31, Author: "kyle", ReviewDecision: "CHANGES_REQUESTED"},
			{Number: 30, Author: "bot"},
		}
	})
	edit("bravo", func(s *models.RepoSummary) { s.StashCount = 1 })
	edit("echo", func(s *models.RepoSummary) { s.NotesFiles = []models.NoteFile{{Name: ".doing"}} })
	edit("kilo", func(s *models.RepoSummary) {
		s.TemplateInfo = &models.CopierTemplateInfo{Commit: "v0.15.1", IsTag: true, Behind: true, LatestTag: "v0.16.0"}
	})
	edit("papa", func(s *models.RepoSummary) { s.Behind = 3 })

	m.due["/Users/dev/oscar"] = []string{"gc"}
	m.updateFilteredPaths()

	return m
}

// TestGoldenRepoListFleetBreakpoints pins the case the hierarchy has to get
// right: a fleet where most rows are quiet and three ask for the viewer.
func TestGoldenRepoListFleetBreakpoints(t *testing.T) {
	for _, size := range breakpointSizes() {
		t.Run(size.name, func(t *testing.T) {
			m := fleetGoldenModel()
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
		{"ci", panelCI},
	}

	for _, size := range breakpointSizes() {
		for _, tab := range tabs {
			t.Run(tab.name+"-"+size.name, func(t *testing.T) {
				m := goldenModel()
				m.width, m.height = size.width, size.height
				m.viewMode = ViewModeRepoDetail
				m.selectedRepo = "/Users/dev/bravo"
				m.focusedPanel = tab.tab
				summary := m.summaries[m.selectedRepo]
				summary.CI = &forge.DefaultBranchCI{Branch: mainBranchName, Workflows: []forge.CIWorkflowRun{
					{Workflow: "docs", Status: forge.StatusCompleted, Conclusion: "success", UpdatedAt: time.Now().Add(-time.Hour)},
					{
						Workflow: "test", Status: forge.StatusCompleted, Conclusion: models.CIFailureConclusion,
						UpdatedAt: time.Now().Add(-48 * time.Hour), FailingJobs: []string{"py3.12", "lint"},
					},
				}}
				m.summaries[m.selectedRepo] = summary
				m.branches = []vcs.BranchInfo{
					{Name: mainBranchName, Upstream: "origin/main", LastCommit: time.Now().Add(-2 * time.Hour)},
					{
						Name: "feature/login", Upstream: "origin/feature/login", Ahead: 2,
						LastCommit: time.Now().Add(-10 * time.Minute), IsCurrent: true,
					},
				}
				m.prs = []forge.PullRequest{
					{Number: 42, Title: "Add login flow", State: "OPEN", HeadRef: "feature/login"},
					{
						Number:  7,
						Title:   "Bump the template to v0.10.0",
						State:   "OPEN",
						IsDraft: true,
						HeadRef: "chore/template",
					},
				}
				golden.RequireEqual(t, []byte(m.renderScreen()))
			})
		}
	}
}

// TestGoldenRepoListExpanded pins the region `v` opens under the table at each
// breakpoint: the table keeps its rows above it, and the region's four sections
// hold their order and their share of the height whatever the note says.
func TestGoldenRepoListExpanded(t *testing.T) {
	for _, size := range breakpointSizes() {
		t.Run(size.name, func(t *testing.T) {
			m := goldenModel()
			m.width, m.height = size.width, size.height
			m.cursor = 1
			m.expandOpen = true

			bravo := m.summaries["/Users/dev/bravo"]
			bravo.NotesFiles = []models.NoteFile{{Name: ".doing", FirstLine: "wire up the session store"}}
			m.summaries["/Users/dev/bravo"] = bravo

			m.notesPreview = map[string][]models.NoteFileContent{
				"/Users/dev/bravo": {{Name: ".doing", Content: "wire up the session store\nthen the logout flow"}},
			}
			m.prMap = map[string]PRMapLoadedMsg{
				"/Users/dev/bravo": {
					Path: "/Users/dev/bravo",
					PRs: []forge.PullRequest{
						{Number: 42, Title: "Add login flow", State: "OPEN", HeadRef: "feature/login"},
					},
					Branches: []vcs.BranchInfo{
						{Name: mainBranchName, Upstream: "origin/main"},
						{Name: "feature/login", Upstream: "origin/feature/login", Ahead: 2, IsCurrent: true},
					},
				},
			}
			golden.RequireEqual(t, []byte(m.renderScreen()))
		})
	}
}

// TestGoldenRepoListLoading pins the list mid-load: charlie's summary has not
// been read, alpha's summary is in but every fetch it started is still out, and
// bravo has settled. A pending cell that quietly renders as an absent one shows
// up here as a frame diff.
func TestGoldenRepoListLoading(t *testing.T) {
	m := goldenModel()
	m.loading = true
	m.loadingCount, m.loadedCount = 3, 2
	delete(m.summaries, "/Users/dev/charlie")
	for _, kind := range []fetchKind{fetchPR, fetchOpenPRs, fetchTemplate, fetchCI} {
		m.startFetch("/Users/dev/alpha", kind)
	}
	m.freshness[fetchKey{"/Users/dev/alpha", fetchOpenPRs}] = freshnessMiss
	m.freshness[fetchKey{"/Users/dev/alpha", fetchCI}] = freshnessMiss
	golden.RequireEqual(t, []byte(m.renderScreen()))
}

// TestGoldenListSizes pins the filter panel with its chip line, and the preview
// pane, which only the wide size has room for.
func TestGoldenListSizes(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Model)
	}{
		{"filter-panel", func(m *Model) {
			m.CycleFilterState(models.FilterModeDirty)
			m.updateFilteredPaths()
			m.viewMode = ViewModeFilter
		}},
		{"preview", func(m *Model) {
			path := "/Users/dev/delta"
			m.cursor = slices.Index(m.filteredPaths, path)
			s := m.summaries[path]
			s.OpenPRs = []forge.PullRequest{
				{Number: 12, Title: "Retry the flaky upload", Author: "kyle", ReviewDecision: "APPROVED"},
				{Number: 14, Title: "Bump the template", Author: "bot", IsDraft: true},
			}
			m.summaries[path] = s
			m.previewCommits[path] = []vcs.CommitInfo{
				{ShortHash: "a1b2c3d", Subject: "Retry uploads on a 502", Date: time.Now().Add(-3 * time.Hour)},
				{ShortHash: "9f8e7d6", Subject: "Log the upload size", Date: time.Now().Add(-26 * time.Hour)},
				{ShortHash: "5c4b3a2", Subject: "Pin the test image", Date: time.Now().Add(-5 * 24 * time.Hour)},
			}
		}},
	}

	for _, c := range cases {
		for _, size := range breakpointSizes() {
			t.Run(c.name+"/"+size.name, func(t *testing.T) {
				m := fleetGoldenModel()
				m.width, m.height = size.width, size.height
				c.setup(&m)
				golden.RequireEqual(t, []byte(m.renderScreen()))
			})
		}
	}
}

func TestGoldenRepoDetail(t *testing.T) {
	m := goldenModel()
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/Users/dev/bravo"
	m.focusedPanel = panelBranches
	m.branches = []vcs.BranchInfo{
		{Name: mainBranchName, Upstream: "origin/main", LastCommit: time.Now().Add(-2 * time.Hour)},
		{
			Name:       "feature/login",
			Upstream:   "origin/feature/login",
			Ahead:      2,
			LastCommit: time.Now().Add(-10 * time.Minute),
			IsCurrent:  true,
		},
	}
	golden.RequireEqual(t, []byte(m.renderScreen()))
}

func TestGoldenBranchDetail(t *testing.T) {
	m := goldenModel()
	m.viewMode = ViewModeBranchDetail
	m.selectedRepo = "/Users/dev/bravo"
	m.branches = []vcs.BranchInfo{
		{Name: mainBranchName, Upstream: "origin/main", LastCommit: time.Now().Add(-2 * time.Hour)},
	}
	m.branchDetail = models.BranchDetail{
		Branch: vcs.BranchInfo{
			Name:      "feature/login",
			Upstream:  "origin/feature/login",
			Ahead:     2,
			IsCurrent: true,
		},
		Commits: []vcs.CommitInfo{
			{
				Hash:      "abc1234def",
				ShortHash: "abc1234",
				Subject:   "Add login flow",
				Author:    "dev",
				Date:      time.Now().Add(-10 * time.Minute),
			},
			{
				Hash:      "9876543abc",
				ShortHash: "9876543",
				Subject:   "Wire up session store",
				Author:    "dev",
				Date:      time.Now().Add(-1 * time.Hour),
			},
		},
		DefaultBranch: mainBranchName,
		DefaultAhead:  2,
		DefaultBehind: 1,
		Staged:        1,
		Unstaged:      2,
		PRInfo: &forge.PullRequest{
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

func TestGoldenTaskRun(t *testing.T) {
	m := goldenModel()
	m.viewMode = ViewModeTaskRun
	m.run = taskRun{label: "fetch", paths: m.repoPaths}
	for i, path := range m.repoPaths {
		result := tasks.Result{
			Path: path, Repo: filepath.Base(path), Status: tasks.StatusOK, Duration: time.Duration(i+1) * 300 * time.Millisecond,
		}
		switch i {
		case 1:
			result.Message = "fetched 2 remotes"
		case 2:
			result.Status, result.ExitCode, result.Message = tasks.StatusFailed, 1, "no remote configured"
		}
		m.run.rows = append(m.run.rows, runRow{path: path, done: true, result: result})
	}
	m.run.rows[0].done = false
	golden.RequireEqual(t, []byte(m.renderScreen()))
}

func TestGoldenTasksBreakpoints(t *testing.T) {
	for _, size := range breakpointSizes() {
		t.Run(size.name, func(t *testing.T) {
			m := tasksGoldenModel()
			m.width, m.height = size.width, size.height
			golden.RequireEqual(t, []byte(m.renderScreen()))
		})
	}
}

func TestGoldenTaskPreview(t *testing.T) {
	m := tasksGoldenModel()
	next, _ := m.openPreview(config.Task{
		Name: "tidy", Desc: "remove untracked build output", Run: "git clean -fdx", Where: "dirty", Confirm: true,
	})
	m = mustAppModel(next)
	golden.RequireEqual(t, []byte(m.renderScreen()))
}
