package app

import (
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// ReposDiscoveredMsg reports the repo paths found during discovery.
type ReposDiscoveredMsg struct {
	Paths []string
}

// GHAuthCheckedMsg reports what, if anything, is wrong with the gh CLI setup.
// Message is empty when gh is present and authenticated.
type GHAuthCheckedMsg struct {
	Message string
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
	// NewestFile and NewestFileTime name the working tree's most recently
	// modified uncommitted file. NewestFile is empty for a clean tree or a jj
	// repo, where there is nothing to report.
	NewestFile     string
	NewestFileTime time.Time
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

// StatusMsg sets the transient status bar message. IsError marks a message the
// operator asked for something the app could not do, which headless script mode
// turns into a nonzero exit.
type StatusMsg struct {
	Message string
	IsError bool
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

// StashDiffLoadedMsg reports one stash's full patch for the focused view's
// detail pane (empty on a failed read, so the pane settles instead of
// waiting forever).
type StashDiffLoadedMsg struct {
	Path  string
	Index int
	Diff  string
}

// StashDiffstatLoadedMsg reports one stash's diffstat for the focused view's
// detail pane (empty on a failed read).
type StashDiffstatLoadedMsg struct {
	Path     string
	Index    int
	Diffstat string
}

// UncommittedDiffLoadedMsg reports the working tree's full patch for a repo
// path (the current repo for the Status panel, a peer checkout for the Peers
// panel; empty on a failed read).
type UncommittedDiffLoadedMsg struct {
	Path string
	Diff string
}

// UncommittedDiffstatLoadedMsg reports the working tree's diffstat for a repo
// path (empty on a failed read).
type UncommittedDiffstatLoadedMsg struct {
	Path     string
	Diffstat string
}

// PRDetailLoadedMsg reports the loaded detail for a single pull request.
type PRDetailLoadedMsg struct {
	Path     string
	PRNumber int
	Detail   models.PRDetail
	Error    error
}

// PRPreviewLoadedMsg reports the loaded detail for one PRs tab row's inline
// preview, keyed by prPreviewKey so a stale in-flight answer can be told apart
// from the row the cursor is on now.
type PRPreviewLoadedMsg struct {
	Key    string
	Detail models.PRDetail
	Error  error
}

// PRSearchLoadedMsg reports the pull requests one saved view returned. Query
// and fleet identify which read this was, so an answer that arrives after the
// view or the scope moved on is discarded rather than shown under the wrong
// heading.
type PRSearchLoadedMsg struct {
	Query string
	Fleet bool
	PRs   []models.PRInfo
	Error error
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

// BranchCountLoadedMsg reports the loaded local branch count for a repo.
type BranchCountLoadedMsg struct {
	Path  string
	Count int
}
