package app

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestWindowSizeMsg(t *testing.T) {
	m := New(nil, 1)

	updatedModel, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updatedModel.(Model)

	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
	if cmd != nil {
		t.Error("window resize should not return a command")
	}
}

func TestReposDiscoveredMsg(t *testing.T) {
	tests := []struct {
		name        string
		paths       []string
		wantLoading bool
		wantCmd     bool
	}{
		{"with repos", []string{"/repo1", "/repo2"}, true, true},
		{"empty list", nil, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(nil, 1)
			updatedModel, cmd := m.Update(ReposDiscoveredMsg{Paths: tt.paths})
			m = updatedModel.(Model)

			if len(m.repoPaths) != len(tt.paths) {
				t.Errorf("expected %d repo paths, got %d", len(tt.paths), len(m.repoPaths))
			}
			if m.loadingCount != len(tt.paths) {
				t.Errorf("expected loadingCount %d, got %d", len(tt.paths), m.loadingCount)
			}
			if m.loadedCount != 0 {
				t.Errorf("expected loadedCount 0, got %d", m.loadedCount)
			}
			if m.loading != tt.wantLoading {
				t.Errorf("expected loading=%v, got %v", tt.wantLoading, m.loading)
			}
			if (cmd != nil) != tt.wantCmd {
				t.Errorf("expected cmd non-nil=%v, got %v", tt.wantCmd, cmd != nil)
			}
		})
	}
}

func TestRepoSummaryLoadedSuccess(t *testing.T) {
	m := New(nil, 1)
	m.loadingCount = 2
	m.loadedCount = 0

	msg := RepoSummaryLoadedMsg{
		Path:    "/repo1",
		Summary: models.RepoSummary{Path: "/repo1", Branch: "main"},
	}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if m.loadedCount != 1 {
		t.Errorf("expected loadedCount 1, got %d", m.loadedCount)
	}
	if m.summaries["/repo1"].Branch != "main" {
		t.Errorf("expected summary stored, got %+v", m.summaries["/repo1"])
	}
	if !m.loading {
		t.Error("loading should remain true until all summaries load")
	}
}

func TestRepoSummaryLoadedError(t *testing.T) {
	m := New(nil, 1)
	m.loadingCount = 1
	loadErr := errors.New("boom")

	msg := RepoSummaryLoadedMsg{Path: "/repo1", Error: loadErr}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	summary, ok := m.summaries["/repo1"]
	if !ok {
		t.Fatal("error summary should still be stored")
	}
	if !errors.Is(summary.Error, loadErr) {
		t.Errorf("expected stored error %v, got %v", loadErr, summary.Error)
	}
	if summary.Path != "/repo1" {
		t.Errorf("expected path preserved, got %q", summary.Path)
	}
}

func TestRepoSummaryLoadingCompletion(t *testing.T) {
	m := New(nil, 1)
	m.repoPaths = []string{"/repo1", "/repo2"}
	m.loadingCount = 2
	m.loadedCount = 1
	m.summaries["/repo1"] = models.RepoSummary{Path: "/repo1"}

	msg := RepoSummaryLoadedMsg{
		Path:    "/repo2",
		Summary: models.RepoSummary{Path: "/repo2"},
	}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if m.loading {
		t.Error("loading should be false once loadedCount reaches loadingCount")
	}
	if len(m.filteredPaths) != 2 {
		t.Errorf("expected filteredPaths refreshed with 2 entries, got %d", len(m.filteredPaths))
	}
}

func TestPRLoadedMsg(t *testing.T) {
	m := New(nil, 1)
	m.summaries["/repo1"] = models.RepoSummary{Path: "/repo1"}

	prInfo := &models.PRInfo{Number: 7}
	updatedModel, cmd := m.Update(PRLoadedMsg{Path: "/repo1", PRInfo: prInfo})
	m = updatedModel.(Model)

	if m.summaries["/repo1"].PRInfo != prInfo {
		t.Error("PRInfo should be attached to the summary")
	}
	if cmd != nil {
		t.Error("PRLoadedMsg should not return a command")
	}

	updatedModel, _ = m.Update(PRLoadedMsg{Path: "/unknown", PRInfo: prInfo})
	m = updatedModel.(Model)
	if _, ok := m.summaries["/unknown"]; ok {
		t.Error("unknown path should not create a summary")
	}
}

func TestWorkflowLoadedMsg(t *testing.T) {
	m := New(nil, 1)
	m.summaries["/repo1"] = models.RepoSummary{Path: "/repo1"}

	workflow := &models.WorkflowSummary{}
	updatedModel, cmd := m.Update(WorkflowLoadedMsg{Path: "/repo1", Workflow: workflow})
	m = updatedModel.(Model)

	if m.summaries["/repo1"].WorkflowInfo != workflow {
		t.Error("WorkflowInfo should be attached to the summary")
	}
	if cmd != nil {
		t.Error("WorkflowLoadedMsg should not return a command")
	}

	updatedModel, _ = m.Update(WorkflowLoadedMsg{Path: "/unknown", Workflow: workflow})
	m = updatedModel.(Model)
	if _, ok := m.summaries["/unknown"]; ok {
		t.Error("unknown path should not create a summary")
	}
}

func TestDetailLoadedMsg(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/repo1"

	msg := DetailLoadedMsg{
		Path:      "/repo1",
		Branches:  []models.BranchInfo{{Name: "main"}},
		Stashes:   []models.StashDetail{{Index: 0}},
		Worktrees: []models.WorktreeInfo{{Path: "/wt"}},
	}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if len(m.branches) != 1 || len(m.stashes) != 1 || len(m.worktrees) != 1 {
		t.Error("detail data should be stored for the selected repo")
	}
	if cmd != nil {
		t.Error("no PRs means no prefetch command")
	}
}

func TestDetailLoadedMsgPathMismatch(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/repo1"

	msg := DetailLoadedMsg{
		Path:     "/other",
		Branches: []models.BranchInfo{{Name: "main"}},
	}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.branches != nil {
		t.Error("mismatched path should not update branches")
	}
	if cmd != nil {
		t.Error("mismatched path should not return a command")
	}
}

func TestBranchDetailLoadedMsg(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/repo1"

	detail := models.BranchDetail{Branch: models.BranchInfo{Name: "feature"}}
	updatedModel, _ := m.Update(BranchDetailLoadedMsg{Path: "/repo1", Detail: detail})
	m = updatedModel.(Model)

	if m.branchDetail.Branch.Name != "feature" {
		t.Errorf("expected branch detail stored, got %q", m.branchDetail.Branch.Name)
	}

	other := models.BranchDetail{Branch: models.BranchInfo{Name: "stale"}}
	updatedModel, _ = m.Update(BranchDetailLoadedMsg{Path: "/other", Detail: other})
	m = updatedModel.(Model)

	if m.branchDetail.Branch.Name != "feature" {
		t.Error("mismatched path should not overwrite branch detail")
	}
}

func TestPRListLoadedMsg(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/repo1"

	updatedModel, _ := m.Update(PRListLoadedMsg{Path: "/repo1", PRs: []models.PRInfo{{Number: 1}, {Number: 2}}})
	m = updatedModel.(Model)

	if len(m.prs) != 2 {
		t.Errorf("expected 2 PRs, got %d", len(m.prs))
	}

	updatedModel, _ = m.Update(PRListLoadedMsg{Path: "/other", PRs: []models.PRInfo{{Number: 9}}})
	m = updatedModel.(Model)

	if len(m.prs) != 2 {
		t.Error("mismatched path should not overwrite PR list")
	}
}

func TestPRDetailLoadedMsgSuccess(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/repo1"
	m.selectedPR = models.PRInfo{Number: 42}

	detail := models.PRDetail{PRInfo: models.PRInfo{Number: 42, Title: "Add feature"}}
	updatedModel, _ := m.Update(PRDetailLoadedMsg{Path: "/repo1", PRNumber: 42, Detail: detail})
	m = updatedModel.(Model)

	if m.prDetail.Title != "Add feature" {
		t.Errorf("expected PR detail stored, got %q", m.prDetail.Title)
	}
}

func TestPRDetailLoadedMsgError(t *testing.T) {
	m := New(nil, 1)
	m.selectedRepo = "/repo1"
	m.selectedPR = models.PRInfo{Number: 42}
	m.prDetail = models.PRDetail{PRInfo: models.PRInfo{Number: 42, Title: "Existing"}}

	updatedModel, cmd := m.Update(PRDetailLoadedMsg{Path: "/repo1", PRNumber: 42, Error: errors.New("gh failed")})
	m = updatedModel.(Model)

	if m.statusMessage == "" {
		t.Error("error should set a status message")
	}
	if m.prDetail.Title != "Existing" {
		t.Error("error should preserve existing PR detail")
	}
	if cmd == nil {
		t.Error("error should return a clear-status command")
	}
}

func TestPRDetailLoadedMsgMismatch(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		prNumber int
	}{
		{"path mismatch", "/other", 42},
		{"PR number mismatch", "/repo1", 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(nil, 1)
			m.selectedRepo = "/repo1"
			m.selectedPR = models.PRInfo{Number: 42}

			detail := models.PRDetail{PRInfo: models.PRInfo{Number: tt.prNumber, Title: "stale"}}
			updatedModel, cmd := m.Update(PRDetailLoadedMsg{Path: tt.path, PRNumber: tt.prNumber, Detail: detail})
			m2 := updatedModel.(Model)

			if m2.prDetail.Title != "" {
				t.Error("mismatched message should not update PR detail")
			}
			if cmd != nil {
				t.Error("mismatched message should not return a command")
			}
		})
	}
}

func TestPRCountLoadedMsg(t *testing.T) {
	m := New(nil, 1)
	m.prCount = nil

	updatedModel, cmd := m.Update(PRCountLoadedMsg{Path: "/repo1", Count: 3})
	m = updatedModel.(Model)

	if m.prCount == nil {
		t.Fatal("prCount map should be initialized")
	}
	if m.prCount["/repo1"] != 3 {
		t.Errorf("expected count 3, got %d", m.prCount["/repo1"])
	}
	if cmd != nil {
		t.Error("PRCountLoadedMsg should not return a command")
	}
}

func TestBatchTaskProgressMsg(t *testing.T) {
	m := New(nil, 1)
	m.batchRunning = true

	msg := batch.TaskProgressMsg{
		Result: batch.TaskResult{Path: "/repo1", Success: true, Message: "fetched"},
	}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if len(m.batchResults) != 1 {
		t.Fatalf("expected 1 batch result, got %d", len(m.batchResults))
	}
	if m.batchResults[0].Path != "/repo1" || !m.batchResults[0].Success || m.batchResults[0].Message != "fetched" {
		t.Errorf("unexpected batch result: %+v", m.batchResults[0])
	}
	if m.batchProgress != 1 {
		t.Errorf("expected batchProgress 1, got %d", m.batchProgress)
	}
	if cmd != nil {
		t.Error("TaskProgressMsg should not return a command")
	}
}

func TestBatchTaskCompleteMsg(t *testing.T) {
	m := New(nil, 1)
	m.batchRunning = true

	msg := batch.TaskCompleteMsg{
		TaskName: "Fetch All",
		Results: []batch.TaskResult{
			{Path: "/repo1", Success: true, Message: "ok"},
			{Path: "/repo2", Success: false, Message: "failed"},
		},
	}
	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(Model)

	if m.batchRunning {
		t.Error("batchRunning should be false after completion")
	}
	if len(m.batchResults) != 2 {
		t.Fatalf("expected 2 batch results, got %d", len(m.batchResults))
	}
	if m.batchResults[1].Success {
		t.Error("second result should record failure")
	}
	if m.batchProgress != 2 {
		t.Errorf("expected batchProgress 2, got %d", m.batchProgress)
	}
	if cmd != nil {
		t.Error("TaskCompleteMsg should not return a command")
	}
}

func TestUpdateFilteredPathsClampsCursor(t *testing.T) {
	m := New(nil, 1)
	m.repoPaths = []string{"/alpha", "/beta", "/gamma"}
	m.summaries = map[string]models.RepoSummary{
		"/alpha": {Path: "/alpha"},
		"/beta":  {Path: "/beta"},
		"/gamma": {Path: "/gamma"},
	}
	m.updateFilteredPaths()
	m.cursor = 2

	m.searchText = "alpha"
	m.updateFilteredPaths()

	if len(m.filteredPaths) != 1 {
		t.Fatalf("expected 1 filtered path, got %d", len(m.filteredPaths))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should clamp to last index 0, got %d", m.cursor)
	}
}

func TestUpdateFilteredPathsEmptyResetsCursor(t *testing.T) {
	m := New(nil, 1)
	m.repoPaths = []string{"/alpha"}
	m.summaries = map[string]models.RepoSummary{"/alpha": {Path: "/alpha"}}
	m.updateFilteredPaths()
	m.cursor = 0

	m.searchText = "no-match"
	m.updateFilteredPaths()

	if len(m.filteredPaths) != 0 {
		t.Fatalf("expected empty filtered paths, got %d", len(m.filteredPaths))
	}
	if m.cursor != 0 {
		t.Errorf("cursor should reset to 0, got %d", m.cursor)
	}
}

func TestStartBatchTaskEmptyIsNoop(t *testing.T) {
	m := New(nil, 1)
	m.filteredPaths = nil

	intermediate, _ := m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	m = intermediate.(Model)
	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	m = updatedModel.(Model)

	if m.viewMode != ViewModeRepoList {
		t.Errorf("empty batch should not change view mode, got %v", m.viewMode)
	}
	if m.batchRunning {
		t.Error("empty batch should not start running")
	}
	if cmd != nil {
		t.Error("empty batch should not return a command")
	}
}

func TestStartBatchTaskWithRepos(t *testing.T) {
	m := New(nil, 1)
	m.filteredPaths = []string{"/repo1", "/repo2"}

	intermediate, _ := m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	m = intermediate.(Model)
	updatedModel, cmd := m.Update(tea.KeyPressMsg{Code: 'F', Text: "F"})
	m = updatedModel.(Model)

	if m.viewMode != ViewModeBatchProgress {
		t.Errorf("expected ViewModeBatchProgress, got %v", m.viewMode)
	}
	if !m.batchRunning {
		t.Error("batch should be running")
	}
	if m.batchTask != "Fetch All" {
		t.Errorf("expected task 'Fetch All', got %q", m.batchTask)
	}
	if m.batchTotal != 2 {
		t.Errorf("expected batchTotal 2, got %d", m.batchTotal)
	}
	if m.batchProgress != 0 || m.batchResults != nil {
		t.Error("batch progress and results should be reset")
	}
	if cmd == nil {
		t.Error("batch start should return a command")
	}
}
