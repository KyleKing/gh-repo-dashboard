package app

import (
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

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
	PRInfo *forge.PullRequest
	Error  error
}

// WorkflowLoadedMsg reports a repo's default-branch CI. Stale is a cached value
// past its expiry, and Miss a cache read that found nothing.
type WorkflowLoadedMsg struct {
	Path   string
	Branch string
	CI     *forge.DefaultBranchCI
	Error  error
	Stale  bool
	Miss   bool
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
	Branches   []vcs.BranchInfo
	Stashes    []vcs.StashDetail
	Worktrees  []vcs.WorktreeInfo
	PRs        []forge.PullRequest
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

// PRListLoadedMsg reports the loaded list of pull requests for a repo.
type PRListLoadedMsg struct {
	Path  string
	PRs   []forge.PullRequest
	Error error
}

// PRDetailLoadedMsg reports one pull request's checks and description for the
// focused view's detail pane.
type PRDetailLoadedMsg struct {
	Path   string
	Detail forge.PRDetail
	Number int
	Error  error
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

// ActionResultMsg reports the outcome of a write action (branch switch, push,
// PR creation, or PR merge) run against a single repo.
type ActionResultMsg struct {
	Path    string
	Message string
	Success bool
}

// OpenPRsLoadedMsg reports a repo's open pull requests. Stale is a cached list
// past its expiry, and Miss a cache read that found nothing.
type OpenPRsLoadedMsg struct {
	Path  string
	PRs   []forge.PullRequest
	Error error
	Stale bool
	Miss  bool
}

// ViewerLoadedMsg reports the signed-in GitHub login, empty when gh cannot say.
type ViewerLoadedMsg struct {
	Login string
}

// DueTasksLoadedMsg reports which configured tasks are due on a repo.
type DueTasksLoadedMsg struct {
	Path  string
	Tasks []string
}

// ChangedPathsLoadedMsg reports a repo's uncommitted paths for the expanded region.
type ChangedPathsLoadedMsg struct {
	Path  string
	Paths []string
}

// BranchCountLoadedMsg reports the loaded local branch count for a repo.
type BranchCountLoadedMsg struct {
	Path  string
	Count int
}
