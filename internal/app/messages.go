package app

import "github.com/kyleking/gh-repo-dashboard/internal/models"

// ReposDiscoveredMsg reports the repo paths found during discovery.
type ReposDiscoveredMsg struct {
	Paths []string
}

// RepoSummaryLoadedMsg reports a loaded repo summary or its load error.
type RepoSummaryLoadedMsg struct {
	Path    string
	Summary models.RepoSummary
	Error   error
}

// PRLoadedMsg reports a loaded pull request summary or its load error.
type PRLoadedMsg struct {
	Path   string
	PRInfo *models.PRInfo
	Error  error
}

// WorkflowLoadedMsg reports a loaded workflow summary or its load error.
type WorkflowLoadedMsg struct {
	Path     string
	Workflow *models.WorkflowSummary
	Error    error
}

// ErrorMsg carries an error to be displayed as a status message.
type ErrorMsg struct {
	Error error
}

// TickMsg triggers a periodic UI refresh.
type TickMsg struct{}

// WindowSizeMsg reports the terminal's current dimensions.
type WindowSizeMsg struct {
	Width  int
	Height int
}

// DetailLoadedMsg reports the branches, stashes, worktrees, PRs, and notes file loaded for a repo's detail view.
type DetailLoadedMsg struct {
	Path         string
	Branches     []models.BranchInfo
	Stashes      []models.StashDetail
	Worktrees    []models.WorktreeInfo
	PRs          []models.PRInfo
	NotesFile    string
	NotesContent string
}

// BranchDetailLoadedMsg reports the loaded detail for a single branch.
type BranchDetailLoadedMsg struct {
	Path   string
	Detail models.BranchDetail
}

// PRCreatedMsg reports the result of creating or opening a pull request.
type PRCreatedMsg struct {
	URL   string
	Error error
}

// CopySuccessMsg reports text successfully copied to the clipboard.
type CopySuccessMsg struct {
	Text string
}

// URLOpenedMsg reports a URL opened in the default browser.
type URLOpenedMsg struct {
	URL string
}

// StatusMsg sets the transient status bar message.
type StatusMsg struct {
	Message string
}

// ClearStatusMsg clears the status bar message.
type ClearStatusMsg struct{}

// RefreshCompleteMsg reports that a refresh finished and which view mode to restore.
type RefreshCompleteMsg struct {
	ViewMode ViewMode
}

// BatchResult reports the outcome of a batch operation on a single repo.
type BatchResult struct {
	Path    string
	Success bool
	Message string
}

// BatchStartMsg reports the start of a batch operation over the given paths.
type BatchStartMsg struct {
	TaskName string
	Paths    []string
}

// BatchProgressMsg reports the result of one repo completing within a batch operation.
type BatchProgressMsg struct {
	Result BatchResult
}

// BatchCompleteMsg reports that a batch operation finished with the given results.
type BatchCompleteMsg struct {
	TaskName string
	Results  []BatchResult
}

// PRListLoadedMsg reports the loaded list of pull requests for a repo.
type PRListLoadedMsg struct {
	Path  string
	PRs   []models.PRInfo
	Error error
}

// PRDetailLoadedMsg reports the loaded detail for a single pull request.
type PRDetailLoadedMsg struct {
	Path     string
	PRNumber int
	Detail   models.PRDetail
	Error    error
}

// PRCountLoadedMsg reports the loaded pull request count for a repo.
type PRCountLoadedMsg struct {
	Path  string
	Count int
}
