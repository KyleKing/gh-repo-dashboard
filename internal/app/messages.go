package app

import "github.com/kyleking/gh-repo-dashboard/internal/models"

type ReposDiscoveredMsg struct {
	Paths []string
}

type RepoSummaryLoadedMsg struct {
	Path    string
	Summary models.RepoSummary
	Error   error
}

type PRLoadedMsg struct {
	Path   string
	PRInfo *models.PRInfo
	Error  error
}

type WorkflowLoadedMsg struct {
	Path     string
	Workflow *models.WorkflowSummary
	Error    error
}

type ErrorMsg struct {
	Error error
}

type TickMsg struct{}

type WindowSizeMsg struct {
	Width  int
	Height int
}

type DetailLoadedMsg struct {
	Path      string
	Branches  []models.BranchInfo
	Stashes   []models.StashDetail
	Worktrees []models.WorktreeInfo
	PRs       []models.PRInfo
}

type BranchDetailLoadedMsg struct {
	Path   string
	Detail models.BranchDetail
}

type PRCreatedMsg struct {
	URL   string
	Error error
}

type CopySuccessMsg struct {
	Text string
}

type URLOpenedMsg struct {
	URL string
}

type StatusMsg struct {
	Message string
}

type ClearStatusMsg struct{}

type RefreshCompleteMsg struct {
	ViewMode ViewMode
}

type BatchResult struct {
	Path    string
	Success bool
	Message string
}

type BatchStartMsg struct {
	TaskName string
	Paths    []string
}

type BatchProgressMsg struct {
	Result BatchResult
}

type BatchCompleteMsg struct {
	TaskName string
	Results  []BatchResult
}

type PRListLoadedMsg struct {
	Path  string
	PRs   []models.PRInfo
	Error error
}

type PRDetailLoadedMsg struct {
	Path     string
	PRNumber int
	Detail   models.PRDetail
	Error    error
}

type PRCountLoadedMsg struct {
	Path  string
	Count int
}
