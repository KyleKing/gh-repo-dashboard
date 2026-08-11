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

// WorkflowLoadedMsg reports a loaded workflow summary or its load error, along
// with the branch the runs belong to.
type WorkflowLoadedMsg struct {
	Path     string
	Branch   string
	Workflow *models.WorkflowSummary
	Error    error
}

// CopierInfoLoadedMsg reports a repo's loaded copier template info, or nil
// when the repo isn't copier-generated.
type CopierInfoLoadedMsg struct {
	Path string
	Info *models.CopierTemplateInfo
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

// DetailLoadedMsg reports the branches, stashes, worktrees, PRs, and notes files loaded for a repo's detail view.
type DetailLoadedMsg struct {
	Path       string
	Branches   []models.BranchInfo
	Stashes    []models.StashDetail
	Worktrees  []models.WorktreeInfo
	PRs        []models.PRInfo
	NotesFiles []models.NoteFileContent
	// DeletableBranches names local, non-current branches whose tip matches a
	// merged pull request's head OID (safe-to-delete detection). It's
	// best-effort: a missing gh yields an empty set rather than failing the load.
	DeletableBranches map[string]bool
}

// NotesContentLoadedMsg reports a repo's notes files read in full, for the
// repo list's notes preview.
type NotesContentLoadedMsg struct {
	Path  string
	Files []models.NoteFileContent
}

// BranchDetailLoadedMsg reports the loaded detail for a single branch.
type BranchDetailLoadedMsg struct {
	Path   string
	Detail models.BranchDetail
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

// StashDiffstatLoadedMsg reports one stash's diffstat for the focused view's
// detail pane. A failed read arrives as an empty diffstat so the pane settles
// instead of waiting forever.
type StashDiffstatLoadedMsg struct {
	Path     string
	Index    int
	Diffstat string
}

// PRDetailLoadedMsg reports the loaded detail for a single pull request.
type PRDetailLoadedMsg struct {
	Path     string
	PRNumber int
	Detail   models.PRDetail
	Error    error
}

// ActionResultMsg reports the outcome of a write action (branch switch, push,
// PR creation, or PR merge) run against a single repo.
type ActionResultMsg struct {
	Path    string
	Message string
	Success bool
}

// PRCountLoadedMsg reports the loaded pull request count for a repo.
type PRCountLoadedMsg struct {
	Path  string
	Count int
}
