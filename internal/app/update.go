package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

const (
	textObjectKeyLen      = 2
	branchDetailLogLimit  = 20
	statusClearDelay      = 3 * time.Second
	prDetailPrefetchCount = 3
)

// Update dispatches an incoming tea.Msg to the handler for its message type.
//
//nolint:gocyclo,cyclop,funlen // flat message-type dispatch; complexity is the case count, not nesting
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)

		return m, tea.Batch(m.visibleCICmds()...)

	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)

	case tea.KeyMsg:
		return m.routeKeyMsg(msg)

	case ReposDiscoveredMsg:
		return m.handleReposDiscovered(msg)

	case RepoSummaryLoadedMsg:
		return m.handleRepoSummaryLoaded(msg)

	case PRLoadedMsg:
		m.settleSummary(msg.Path, fetchPR, func(s *models.RepoSummary) { s.PRInfo = msg.PRInfo })

		return m, nil

	case WorkflowLoadedMsg:
		return m.handleWorkflowLoaded(msg)

	case CopierInfoLoadedMsg:
		m.settleSummary(msg.Path, fetchTemplate, func(s *models.RepoSummary) { s.TemplateInfo = msg.Info })

		return m, nil

	case DetailLoadedMsg:
		return m.handleDetailLoaded(msg)

	case NotesContentLoadedMsg:
		return m.handleNotesContentLoaded(msg)

	case BranchDetailLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.branchDetailLoading = false
			m.branchDetail = msg.Detail
		}

		return m, nil

	case PRListLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.prs = msg.PRs
			m = m.snapFocusToShownPanel()
		}

		return m, nil

	case PRDetailLoadedMsg:
		return m.handlePRDetailLoaded(msg)

	case StashDiffLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.stashDiff = withStashText(m.stashDiff, msg.Index, msg.Diff)
		}

		return m, nil

	case StashDiffstatLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.stashDiffstat = withStashText(m.stashDiffstat, msg.Index, msg.Diffstat)
		}

		return m, nil

	case ActionResultMsg:
		return m.handleActionResult(msg)

	case PRMapLoadedMsg:
		m.prMap[msg.Path] = msg
		m.finishFetch(msg.Path, fetchExpand)

		return m, nil

	case PRCountLoadedMsg:
		m.prCount[msg.Path] = msg.Count
		m.finishFetch(msg.Path, fetchPRCount)

		return m, nil

	case CopySuccessMsg:
		return m.withFadingStatus("Copied to clipboard: " + msg.Text)

	case URLOpenedMsg:
		return m.withFadingStatus("Opened in browser: " + msg.URL)

	case StatusMsg:
		m.statusMessage = msg.Message
		return m, nil

	case ClearStatusMsg:
		m.statusMessage = ""
		return m, nil

	case RefreshCompleteMsg:
		return m.withFadingStatus("Data refreshed")

	case batch.TaskProgressMsg:
		return m.recordBatchResults(msg.Result), nil

	case batch.TaskCompleteMsg:
		m.batchRunning = false

		return m.recordBatchResults(msg.Results...), nil

	case ErrorMsg:
		return m, nil
	}

	return m, nil
}

// settleSummary applies a fetch's result to path's summary when the list still
// holds one, and marks the fetch settled either way, so a value arriving for a
// repo that has since gone stops reading as in flight.
func (m *Model) settleSummary(path string, kind fetchKind, apply func(*models.RepoSummary)) {
	if summary, ok := m.summaries[path]; ok {
		apply(&summary)
		m.summaries[path] = summary
	}

	m.finishFetch(path, kind)
}

// withFadingStatus shows text and schedules its own removal, which is what
// every message reporting a finished action wants.
func (m Model) withFadingStatus(text string) (tea.Model, tea.Cmd) {
	m.statusMessage = text

	return m, clearStatusAfterDelay()
}

func (m Model) recordBatchResults(results ...batch.TaskResult) Model {
	for _, r := range results {
		m.batchResults = append(m.batchResults, BatchResult{
			Path:    r.Path,
			Success: r.Success,
			Message: r.Message,
		})
	}
	m.batchProgress = len(m.batchResults)

	return m
}

// routeKeyMsg dispatches a key press to the handler for the current mode
// (search, command-bar, or one of the view modes).
func (m Model) routeKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.searching {
		return m.handleSearchKey(msg)
	}
	if m.commandMode {
		return m.handleCommandKey(msg)
	}
	if newM, cmd, handled := m.handleChordKey(msg); handled {
		return newM, cmd
	}
	switch m.viewMode {
	case ViewModeFilter:
		return m.handleFilterKey(msg)
	case ViewModeSort:
		return m.handleSortKey(msg)
	case ViewModeRepoDetail:
		return m.handleDetailKey(msg)
	case ViewModeBranchDetail:
		return m.handleBranchDetailKey(msg)
	case ViewModePRDetail:
		return m.handlePRDetailKey(msg)
	case ViewModeBatchProgress:
		return m.handleBatchKey(msg)
	case ViewModeConfirm:
		return m.handleConfirmKey(msg)
	case ViewModePRMap:
		return m.handlePRMapKey(msg)
	case ViewModePalette:
		return m.handlePaletteKey(msg)
	default:
		return m.handleKey(msg)
	}
}

// handleReposDiscovered records the discovered repo paths and kicks off a
// summary load for each.
func (m Model) handleReposDiscovered(msg ReposDiscoveredMsg) (tea.Model, tea.Cmd) {
	m.repoPaths = msg.Paths
	m.loadingCount = len(msg.Paths)
	m.loadedCount = 0

	if len(msg.Paths) == 0 {
		m.loading = false
	}

	m.updateFilteredPaths()

	cmds := make([]tea.Cmd, 0, len(msg.Paths)+1)
	for _, path := range msg.Paths {
		cmds = append(cmds, loadRepoSummaryCmd(path))
	}

	// Scanning from inside a repo has nothing to list, so open it directly.
	// esc still falls back to the one-row list.
	if len(msg.Paths) == 1 {
		m.selectedRepo = msg.Paths[0]
		m.viewMode = ViewModeRepoDetail
		m.focusedPanel = panelBranches
		m.detailCursor = 0
		cmds = append(cmds, loadDetailCmd(m.selectedRepo))
	}

	return m, tea.Batch(cmds...)
}

// handleRepoSummaryLoaded merges a loaded repo summary and, once every repo
// has reported in, ends the initial loading state.
func (m Model) handleRepoSummaryLoaded(msg RepoSummaryLoadedMsg) (tea.Model, tea.Cmd) {
	m.loadedCount++

	m.summaries[msg.Path] = msg.Summary

	var cmds []tea.Cmd
	if msg.Error == nil {
		cmds = append(cmds, m.followUpCmds(msg.Summary)...)
	}

	if m.loadedCount >= m.loadingCount {
		m.loading = false
		m.updateFilteredPaths()
		cmds = append(cmds, m.visibleCICmds()...)
	}

	return m, tea.Batch(cmds...)
}

// followUpCmds starts the per-repo fetches a loaded summary enables, recording
// each as in flight so the cell it fills reads as pending until it lands. A
// repo with no upstream has no pull request to ask about, and its commands are
// nil, so nothing is recorded for it.
func (m *Model) followUpCmds(summary models.RepoSummary) []tea.Cmd {
	started := []struct {
		kind fetchKind
		cmd  tea.Cmd
	}{
		{fetchPR, loadPRCmd(summary.Path, summary.Branch, summary.Upstream)},
		{fetchPRCount, loadPRCountCmd(summary.Path, summary.RemoteID, summary.Upstream)},
		{fetchTemplate, loadCopierInfoCmd(summary.Path)},
	}

	cmds := make([]tea.Cmd, 0, len(started))
	for _, start := range started {
		if start.cmd == nil {
			continue
		}
		m.startFetch(summary.Path, start.kind)
		cmds = append(cmds, start.cmd)
	}

	return cmds
}

// handleWorkflowLoaded records a CI fetch's outcome. A repo with no GitHub
// remote, or one whose runs cannot be read, arrives with no workflow and is
// still marked settled so its cell stops showing the in-flight placeholder.
func (m Model) handleWorkflowLoaded(msg WorkflowLoadedMsg) (tea.Model, tea.Cmd) {
	m.settleSummary(msg.Path, fetchCI, func(s *models.RepoSummary) { s.WorkflowInfo = msg.Workflow })

	if msg.Branch != "" {
		if m.ciBranch == nil {
			m.ciBranch = make(map[string]string)
		}
		m.ciBranch[msg.Path] = msg.Branch
	}

	return m, nil
}

// handlePRDetailLoaded stores a loaded PR detail if it's still for the
// currently selected repo/PR, preserving prior info on error.
func (m Model) handlePRDetailLoaded(msg PRDetailLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Path != m.selectedRepo || !m.showingPR(msg.PRNumber) {
		return m, nil
	}
	if msg.Error != nil {
		// Don't clear basic info on error - preserve what we already have
		m.statusMessage = fmt.Sprintf("Failed to load PR details: %v", msg.Error)
		return m, clearStatusAfterDelay()
	}
	m.prDetail = msg.Detail

	return m, nil
}

// showingPR reports whether the pull request is the one on screen. The
// focused view tracks its selection with the panel cursor and never sets
// selectedPR, which only the PR-detail view owns.
func (m Model) showingPR(number int) bool {
	if m.selectedPR.Number == number {
		return true
	}

	pr, ok := m.selectedPanelPR()

	return ok && pr.Number == number
}

// handleSpinnerTick advances the loading spinner. The tick chain stops once
// nothing is loading, so an idle dashboard issues no further redraws.
func (m Model) handleSpinnerTick(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	if !m.anyLoading() {
		return m, nil
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)

	return m, cmd
}

// handleDetailLoaded stores the loaded repo detail (branches, stashes,
// worktrees, PRs, notes), fills the detail pane for whatever the cursor
// already sits on, and prefetches the first few PR details.
func (m Model) handleDetailLoaded(msg DetailLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Path != m.selectedRepo {
		return m, nil
	}

	m.detailLoading = false
	m.branches = msg.Branches
	m.deletableBranches = msg.DeletableBranches
	m.stashes = msg.Stashes
	m.worktrees = msg.Worktrees
	m.prs = msg.PRs
	m.notesFiles = msg.NotesFiles
	m = m.snapFocusToShownPanel()

	prefetchCount := min(prDetailPrefetchCount, len(msg.PRs))

	cmds := []tea.Cmd{m.panelDetailCmd()}
	for i := range prefetchCount {
		cmds = append(cmds, prefetchPRDetailCmd(msg.Path, m.summaries[msg.Path].RemoteID, msg.PRs[i].Number))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingOperator != "" {
		return m.handleOperatorPendingKey(msg)
	}

	if key.Matches(msg, m.keys.Repeat) {
		m.pendingRepeat = true
		return m, nil
	}

	if newM, handled := m.handleCursorKey(msg); handled {
		cmds := append(newM.visibleCICmds(), newM.expandCmd())

		return newM, tea.Batch(cmds...)
	}

	if newM, handled := m.handleModeKey(msg); handled {
		return newM, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Enter):
		return m.handleEnterKey()

	case key.Matches(msg, m.keys.Find):
		return m.openPalette()

	case key.Matches(msg, m.keys.Back):
		return m.handleBackKey()

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Search):
		return m.openSearch()

	case key.Matches(msg, m.keys.Expand):
		return m.toggleExpand()

	case key.Matches(msg, m.keys.Peers):
		return m.openCheckouts()

	case key.Matches(msg, m.keys.FetchAll),
		key.Matches(msg, m.keys.PruneRemote),
		key.Matches(msg, m.keys.CleanupMerged),
		key.Matches(msg, m.keys.RefreshPRs):
		m.pendingOperator = msg.String()
		m.pendingObject = ""

		return m, nil
	}

	return m, nil
}

// handleModeKey handles the keys that only swap which full-screen mode is
// showing. Handled is false if msg didn't match any of them.
func (m Model) handleModeKey(msg tea.KeyMsg) (Model, bool) {
	switch {
	case key.Matches(msg, m.keys.Help):
		if m.viewMode == ViewModeHelp {
			m.viewMode = ViewModeRepoList
		} else {
			m.viewMode = ViewModeHelp
		}

		return m, true

	case key.Matches(msg, m.keys.Filter):
		m.viewMode = ViewModeFilter
		return m, true

	case key.Matches(msg, m.keys.Sort):
		m.viewMode = ViewModeSort
		m.sortCursor = 0

		return m, true
	}

	return m, false
}

// handlePRMapKey drives the fleet map: movement, opening the repo behind a
// row, and opening a pull request in the browser.
func (m Model) handlePRMapKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := m.buildPRMap()

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoList
		m.prMapCursor = 0

		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.prMapCursor = max(m.prMapCursor-1, 0)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.prMapCursor = min(m.prMapCursor+1, max(len(entries)-1, 0))
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.prMapCursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.prMapCursor = max(len(entries)-1, 0)
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		return m.openPRMapRepo(entries)

	case key.Matches(msg, m.keys.OpenURL):
		if m.prMapCursor < len(entries) && entries[m.prMapCursor].HasPR() {
			return m, openURLCmd(entries[m.prMapCursor].PR.URL)
		}

		return m, nil
	}

	return m, nil
}

func (m Model) openPRMapRepo(entries []prMapEntry) (tea.Model, tea.Cmd) {
	if m.prMapCursor >= len(entries) {
		return m, nil
	}

	repo := entries[m.prMapCursor].Repo
	for _, path := range m.filteredPaths {
		if filepath.Base(path) == repo {
			m.prMapCursor = 0

			return m.openRepo(path)
		}
	}

	return m, nil
}

// openPRMap loads every visible repo's pull requests and branch list, then
// shows the fleet map. The pull request call is the per-repo list the detail
// view already makes; the branch list is local git.
func (m Model) openPRMap() (Model, tea.Cmd) {
	m.viewMode = ViewModePRMap
	m.prMapCursor = 0

	cmds := make([]tea.Cmd, 0, len(m.filteredPaths)+1)
	for _, path := range m.filteredPaths {
		m.startFetch(path, fetchExpand)
		cmds = append(cmds, loadPRMapCmd(path, m.summaries[path].RemoteID, m.summaries[path].Upstream))
	}
	cmds = append(cmds, m.spinner.Tick)

	return m, tea.Batch(cmds...)
}

// handleChordKey resolves the keys that either complete a pending two-key
// chord or start one: "@:" to repeat a command, "gg" to jump to the top, and
// ":" to open the command bar.
func (m Model) handleChordKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case m.pendingRepeat:
		m.pendingRepeat = false
		if key.Matches(msg, m.keys.Command) {
			model, cmd := m.repeatLastCommand()

			return model, cmd, true
		}

		return m, nil, true

	case m.pendingTop:
		m.pendingTop = false
		if key.Matches(msg, m.keys.TopPrefix) {
			return m.moveToTop(), nil, true
		}

		return m, nil, true

	case key.Matches(msg, m.keys.TopPrefix) && !m.acceptsCheckoutPR():
		m.pendingTop = true
		return m, nil, true

	case key.Matches(msg, m.keys.Command):
		m.commandMode = true
		m.commandInput.Reset()
		m.completionCandidates = nil
		m.commandInput.Focus()

		return m, nil, true
	}

	return m, nil, false
}

// acceptsCheckoutPR reports whether the current view has a pull request for
// "g" to check out. Elsewhere "g" opens the "gg" chord for jumping to the top.
// The panel grid is not one of them: its verbs live behind the leader key, so
// "gg" means the same thing in every panel.
func (m Model) acceptsCheckoutPR() bool {
	return m.viewMode == ViewModePRDetail
}

func (m Model) moveToTop() Model {
	switch m.viewMode {
	case ViewModeRepoDetail:
		m.detailCursor = 0
	case ViewModePRMap:
		m.prMapCursor = 0
	default:
		m.cursor = 0
	}

	return m
}

// openSearch starts a fresh search. The buffer and the committed query are
// both cleared, so the prompt and the filtered list agree from the first
// keystroke instead of appending to whatever was searched last.
func (m Model) openSearch() (tea.Model, tea.Cmd) {
	m.searching = true
	m.searchInput.SetValue("")
	m.searchText = ""
	m.searchInput.Focus()
	m.updateFilteredPaths()
	m.cursor = 0

	return m, nil
}

// visibleCICmds requests CI for the repos currently on screen that have not
// been asked for yet. Nothing is fetched for rows the user has never seen,
// which keeps a 62-repo fleet from spending 62 calls to draw one screen.
func (m *Model) visibleCICmds() []tea.Cmd {
	rowHeight := 1
	if m.isCompact() {
		rowHeight = compactRowHeight
	}

	window := m.visibleRepoRange(rowHeight)

	var cmds []tea.Cmd
	for i := window.start; i < window.end; i++ {
		path := m.filteredPaths[i]
		if m.ciRequested[path] {
			continue
		}

		if m.ciRequested == nil {
			m.ciRequested = make(map[string]bool)
		}
		m.ciRequested[path] = true
		m.startFetch(path, fetchCI)
		cmds = append(cmds, loadDefaultBranchCICmd(path, m.summaries[path].RemoteID))
	}

	return cmds
}

func (m Model) handleNotesContentLoaded(msg NotesContentLoadedMsg) (tea.Model, tea.Cmd) {
	if m.notesPreview == nil {
		m.notesPreview = make(map[string][]models.NoteFileContent)
	}
	m.notesPreview[msg.Path] = msg.Files

	return m, nil
}

// toggleExpand opens or closes the list's expanded region. A terminal too short
// to seat one would swallow the keypress with nothing on screen to explain it,
// so it says so instead.
func (m Model) toggleExpand() (tea.Model, tea.Cmd) {
	m.expandOpen = !m.expandOpen

	if m.expandOpen && m.expandHeight(m.listBodyHeight()) == 0 {
		m.expandOpen = false
		m.statusMessage = "Terminal too short for the expanded region"

		return m, clearStatusAfterDelay()
	}

	cmd := m.expandCmd()

	return m, cmd
}

// expandCmd reads what the region shows for the repo under the cursor and is
// not holding yet: its notes, and the branches and pull requests the fleet map
// caches beside them.
func (m *Model) expandCmd() tea.Cmd {
	if !m.expandOpen || m.cursor >= len(m.filteredPaths) {
		return nil
	}

	path := m.filteredPaths[m.cursor]

	summary, ok := m.summaries[path]
	if !ok {
		return nil
	}

	var cmds []tea.Cmd

	if _, cached := m.notesPreview[path]; !cached && len(summary.NotesFiles) > 0 {
		cmds = append(cmds, loadNotesContentCmd(path, summary.NotesFiles))
	}

	if _, cached := m.prMap[path]; !cached && !m.fetchPending(path, fetchExpand) {
		m.startFetch(path, fetchExpand)
		cmds = append(cmds, loadPRMapCmd(path, summary.RemoteID, summary.Upstream))
	}

	return tea.Batch(cmds...)
}

// handleCursorKey handles the repo-list cursor movement keys (up/down/top/
// bottom). Handled is false if msg didn't match any of them.
func (m Model) handleCursorKey(msg tea.KeyMsg) (Model, bool) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.filteredPaths)-1 {
			m.cursor++
		}

	case key.Matches(msg, m.keys.Top):
		m.cursor = 0

	case key.Matches(msg, m.keys.Bottom):
		if len(m.filteredPaths) > 0 {
			m.cursor = len(m.filteredPaths) - 1
		}

	default:
		return m, false
	}

	return m, true
}

// handleEnterKey opens the detail view for the selected repo, from the repo list.
func (m Model) handleEnterKey() (tea.Model, tea.Cmd) {
	if m.viewMode != ViewModeRepoList || m.cursor >= len(m.filteredPaths) {
		return m, nil
	}

	m.selectedRepo = m.filteredPaths[m.cursor]
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelBranches
	m.detailCursor = 0
	m.branches = nil
	m.stashes = nil
	m.worktrees = nil
	m.prs = nil
	m.notesFiles = nil
	m.stashDiffstat = nil
	m.stashDiff = nil
	m.stashFullDiff = false
	m.branchDetail = models.BranchDetail{}
	m.prDetail = models.PRDetail{}
	m.detailLoading = true

	return m, tea.Batch(loadDetailCmd(m.selectedRepo), m.spinner.Tick)
}

// handleBackKey pops the current view back to its parent, if it has one.
func (m Model) handleBackKey() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeRepoDetail:
		if last := len(m.repoStack) - 1; last >= 0 {
			previous := m.repoStack[last]
			m.repoStack = m.repoStack[:last]

			return m.openRepo(previous)
		}

		m.viewMode = ViewModeRepoList
	case ViewModeBranchDetail:
		m.viewMode = ViewModeRepoDetail
	case ViewModeHelp:
		m.viewMode = ViewModeRepoList
	case ViewModeFilter:
		m.viewMode = ViewModeRepoList
	case ViewModeSort:
		m.viewMode = ViewModeRepoList
	case ViewModeRepoList:
		return m.dismissListLayer()
	default:
		// no back transition from this view
	}

	return m, nil
}

// dismissListLayer closes one layer of the repo list, most recently opened
// first: the expanded region, then the search text, then the filters. The list
// itself has no parent to pop to, so a back key with nothing open does
// nothing rather than leaving the fleet.
func (m Model) dismissListLayer() (tea.Model, tea.Cmd) {
	switch {
	case m.expandOpen:
		m.expandOpen = false

		return m, nil

	case m.searchText != "":
		m.searchText = ""
		m.searchInput.SetValue("")
		m.updateFilteredPaths()
		m.cursor = 0

		return m, statusCmd("Search cleared")

	case m.filtersActive():
		m.ResetFilters()
		m.updateFilteredPaths()
		m.cursor = 0

		return m, statusCmd("Filters cleared")
	}

	return m, nil
}

func (m Model) handleOperatorPendingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	op, ok := lookupOperator(m.pendingOperator)
	if !ok {
		m.pendingOperator = ""
		m.pendingObject = ""

		return m, nil
	}

	keyStr := msg.String()
	switch {
	case keyStr == keyEsc:
		m.pendingOperator = ""
		m.pendingObject = ""

		return m, nil

	case keyStr == m.pendingOperator && m.pendingObject == "":
		m.pendingOperator = ""
		return m.confirmBatchTask(op.TaskName, op.Destructive, m.filteredPaths, op.Cmd)
	}

	m.pendingObject += keyStr
	if len(m.pendingObject) < textObjectKeyLen {
		if hasTextObjectPrefix(m.pendingObject) {
			return m, nil
		}
		m.pendingOperator = ""
		m.pendingObject = ""

		return m, statusCmd("Unknown text object: " + keyStr)
	}

	objKey := m.pendingObject
	m.pendingOperator = ""
	m.pendingObject = ""

	obj, found := lookupTextObject(objKey)
	if !found {
		return m, statusCmd("Unknown text object: " + objKey)
	}

	paths := m.resolveTextObject(obj)
	if len(paths) == 0 {
		return m, statusCmd("No repos match " + obj.Name)
	}

	return m.confirmBatchTask(fmt.Sprintf("%s (%s)", op.TaskName, obj.Name), op.Destructive, paths, op.Cmd)
}

func hasTextObjectPrefix(prefix string) bool {
	for _, obj := range textObjects() {
		if strings.HasPrefix(obj.Key, prefix) {
			return true
		}
	}

	return false
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.panelActions {
		return m.handlePanelActionKey(msg)
	}

	if msg.String() == panelActionLeader {
		m.panelActions = true
		return m, nil
	}

	panels := m.panelSet(m.gridWidth())
	if p, ok := panelForKey(panels, msg.String()); ok {
		return m.focusPanel(p.id)
	}

	if m.detailFocused {
		return m.handleDetailPaneKey(msg)
	}

	if newM, cmd, handled := m.handlePanelMoveKey(msg, panels); handled {
		return newM, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		return m.handleBackKey()

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Find):
		return m.openPalette()

	case key.Matches(msg, m.keys.Enter):
		m.detailFocused = true
		m.detailScroll = 0

		return m, nil

	case key.Matches(msg, m.keys.OpenDetail):
		return m.handleDetailOpenKey()

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m, nil
}

// handlePanelMoveKey handles movement within the panel column: between panels
// and between the rows of the focused one. Handled is false if msg didn't
// match any of them.
func (m Model) handlePanelMoveKey(msg tea.KeyMsg, panels []panelContent) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Right):
		model, cmd := m.cyclePanel(panels, 1)
		return model, cmd, true

	case key.Matches(msg, m.keys.Left):
		model, cmd := m.cyclePanel(panels, -1)
		return model, cmd, true

	case key.Matches(msg, m.keys.Up):
		model, cmd := m.moveDetailCursor(panels, -1)
		return model, cmd, true

	case key.Matches(msg, m.keys.Down):
		model, cmd := m.moveDetailCursor(panels, 1)
		return model, cmd, true

	case key.Matches(msg, m.keys.Top):
		m.detailCursor = 0
		return m, nil, true

	case key.Matches(msg, m.keys.Bottom):
		if maxIdx := m.detailListLen() - 1; maxIdx >= 0 {
			m.detailCursor = maxIdx
		}

		return m, nil, true
	}

	return m, nil, false
}

// handleDetailPaneKey drives the detail pane once enter has moved focus into
// it: movement scrolls the text rather than changing what is selected, and the
// write actions still apply to the item the pane is describing.
func (m Model) handleDetailPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Left):
		m.detailFocused = false
		m.detailScroll = 0

		return m, nil

	case key.Matches(msg, m.keys.Up):
		return m.scrollDetailPane(-1), nil

	case key.Matches(msg, m.keys.Down):
		return m.scrollDetailPane(1), nil

	case key.Matches(msg, m.keys.Top):
		m.detailScroll = 0
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.detailScroll = m.maxDetailScroll()
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Find):
		return m.openPalette()

	case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.OpenDetail):
		return m.handleDetailOpenKey()

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m, nil
}

// scrollDetailPane moves the detail pane by delta lines, clamped so the last
// line of text never scrolls past the top of the pane.
func (m Model) scrollDetailPane(delta int) Model {
	m.detailScroll = min(max(m.detailScroll+delta, 0), m.maxDetailScroll())

	return m
}

func (m Model) maxDetailScroll() int {
	pane := m.detailPaneSize()

	return max(len(m.panelDetailLines(pane.width))-pane.height, 0)
}

// handleActionKey routes the write-action keys shared by the repo-detail,
// branch-detail, and PR-detail views.
func (m Model) handleActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.CheckoutPR):
		return m.startCheckoutPR()

	case key.Matches(msg, m.keys.SwitchBranch):
		return m.startSwitchBranch()

	case key.Matches(msg, m.keys.PushBranch):
		return m.startPushBranch()

	case key.Matches(msg, m.keys.CreatePR):
		return m.startCreatePR()

	case key.Matches(msg, m.keys.MergePR):
		return m.startSquashMergePR()
	}

	return m, nil
}

// snapFocusToShownPanel moves focus off a panel the grid has dropped for being
// empty and clamps the row cursor to what the focused panel still lists, so
// the cursor is never parked on a box that is not drawn or a row that is gone.
// Call it wherever a message replaces the focused repo's lists.
func (m Model) snapFocusToShownPanel() Model {
	panels := m.panelSet(m.gridWidth())
	if len(panels) == 0 {
		return m
	}

	for _, p := range panels {
		if p.id == m.focusedPanel {
			m.detailCursor = min(m.detailCursor, max(m.panelRowCount(p.id)-1, 0))

			return m
		}
	}

	m.focusedPanel = panels[0].id
	m.detailCursor = 0
	m.detailScroll = 0

	return m
}

// focusPanel moves the cursor to a panel's first row.
func (m Model) focusPanel(id panelID) (tea.Model, tea.Cmd) {
	return m.selectPanelRow(id, 0)
}

// selectPanelRow puts the cursor on one row of one panel and loads whatever the
// detail pane now needs. It also hands focus back to the panel column, because
// a key naming a panel is a request to be in that panel, not in the pane
// describing its old selection.
func (m Model) selectPanelRow(id panelID, cursor int) (tea.Model, tea.Cmd) {
	m.focusedPanel = id
	m.detailCursor = max(cursor, 0)
	m.detailFocused = false
	m.detailScroll = 0

	return m, m.panelDetailCmd()
}

// cyclePanel moves focus delta panels along the grid, wrapping around.
func (m Model) cyclePanel(panels []panelContent, delta int) (tea.Model, tea.Cmd) {
	if len(panels) == 0 {
		return m, nil
	}

	next := (panelIndex(panels, m.focusedPanel) + delta + len(panels)) % len(panels)

	return m.focusPanel(panels[next].id)
}

// moveDetailCursor moves the row cursor by delta. The panel column reads as one
// vertical list, so a move off either end of a panel carries into its
// neighbor: a panel holding one row, or none at all, would otherwise swallow
// the key and leave the movement keys looking dead.
func (m Model) moveDetailCursor(panels []panelContent, delta int) (tea.Model, tea.Cmd) {
	next := m.detailCursor + delta
	if next >= 0 && next < m.detailListLen() {
		m.detailCursor = next
		m.detailScroll = 0

		return m, m.panelDetailCmd()
	}

	return m.crossPanel(panels, delta)
}

// crossPanel carries a row move off the end of one panel into the next, landing
// on the row nearest the boundary just crossed. It stops at the two ends of the
// column rather than wrapping, since tab is the key that cycles.
func (m Model) crossPanel(panels []panelContent, delta int) (tea.Model, tea.Cmd) {
	next := panelIndex(panels, m.focusedPanel) + delta
	if next < 0 || next >= len(panels) {
		return m, nil
	}

	cursor := 0
	if delta < 0 {
		cursor = m.panelRowCount(panels[next].id) - 1
	}

	return m.selectPanelRow(panels[next].id, cursor)
}

// panelDetailCmd fetches whatever the detail pane needs for the newly
// selected item, and nothing when the pane can render from cached data.
func (m Model) panelDetailCmd() tea.Cmd {
	switch {
	case m.focusedPanel == panelPRs && m.detailCursor < len(m.prs):
		pr := m.prs[m.detailCursor]
		if m.prDetail.Number == pr.Number {
			return nil
		}

		return loadPRDetailCmd(m.selectedRepo, m.summaries[m.selectedRepo].RemoteID, pr.Number)

	case m.focusedPanel == panelBranches && m.detailCursor < len(m.branches):
		branch := m.branches[m.detailCursor]
		if m.branchDetail.Branch.Name == branch.Name {
			return nil
		}

		return loadBranchDetailCmd(m.selectedRepo, branch.Name)

	case m.focusedPanel == panelStashes && m.detailCursor < len(m.stashes):
		return m.stashDetailCmd(m.stashes[m.detailCursor].Index)
	}

	return nil
}

func withStashText(texts map[int]string, index int, text string) map[int]string {
	if texts == nil {
		texts = make(map[int]string)
	}
	texts[index] = text

	return texts
}

// stashDetailCmd fetches whichever of the stash's diffstat or full patch the
// detail pane is showing, and nothing when that one is already cached.
func (m Model) stashDetailCmd(index int) tea.Cmd {
	if m.stashFullDiff {
		if _, loaded := m.stashDiff[index]; loaded {
			return nil
		}

		return loadStashDiffCmd(m.selectedRepo, index, m.detailPaneSize().width-stashBodyIndent)
	}

	if _, loaded := m.stashDiffstat[index]; loaded {
		return nil
	}

	return loadStashDiffstatCmd(m.selectedRepo, index)
}

// handleDetailOpenKey opens the branch-detail or PR-detail view for the
// currently selected row.
func (m Model) handleDetailOpenKey() (tea.Model, tea.Cmd) {
	switch {
	case m.focusedPanel == panelBranches && m.detailCursor < len(m.branches):
		m.selectedBranch = m.branches[m.detailCursor]
		m.branchDetail = models.BranchDetail{} // Clear previous detail
		m.branchDetailLoading = true
		m.viewMode = ViewModeBranchDetail

		return m, tea.Batch(loadBranchDetailCmd(m.selectedRepo, m.selectedBranch.Name), m.spinner.Tick)

	case m.focusedPanel == panelPeers:
		return m.jumpToCheckout()

	case m.focusedPanel == panelPRs && m.detailCursor < len(m.prs):
		m.selectedPR = m.prs[m.detailCursor]
		// Progressive loading: show basic info from the list immediately,
		// full details (author, assignees, etc.) load async.
		m.prDetail = models.PRDetail{PRInfo: m.selectedPR}
		m.viewMode = ViewModePRDetail

		return m, loadPRDetailCmd(m.selectedRepo, m.summaries[m.selectedRepo].RemoteID, m.selectedPR.Number)

	default:
		return m, nil
	}
}

// openCheckouts drills into the repo under the cursor and lands on its
// parallel-checkout tab, so the peer set is one key away from the fleet list.
func (m Model) openCheckouts() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.filteredPaths) {
		return m, nil
	}

	next, cmd := m.openRepo(m.filteredPaths[m.cursor])

	opened, ok := next.(Model)
	if !ok {
		return next, cmd
	}
	opened.focusedPanel = panelPeers

	return opened, cmd
}

// jumpToCheckout re-roots the focused view on the parallel checkout under the
// cursor, pushing the current repo so esc walks back out. Checkouts that
// discovery never scanned (a worktree outside the scan paths) have no summary
// to show, so they are left alone.
func (m Model) jumpToCheckout() (tea.Model, tea.Cmd) {
	checkouts := m.RepoCheckouts()
	if m.detailCursor >= len(checkouts) {
		return m, nil
	}

	target := checkouts[m.detailCursor].Path
	if _, known := m.summaries[target]; !known {
		m.statusMessage = "No scanned repo at " + target
		return m, nil
	}

	m.repoStack = append(m.repoStack, m.selectedRepo)

	return m.openRepo(target)
}

// openRepo points the focused view at a repo and clears the tab data the
// previous one left behind.
func (m Model) openRepo(path string) (tea.Model, tea.Cmd) {
	m.selectedRepo = path
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelBranches
	m.detailCursor = 0
	m.branches = nil
	m.stashes = nil
	m.worktrees = nil
	m.prs = nil
	m.notesFiles = nil
	m.stashDiffstat = nil
	m.stashDiff = nil
	m.stashFullDiff = false
	m.branchDetail = models.BranchDetail{}
	m.prDetail = models.PRDetail{}
	m.detailLoading = true

	return m, tea.Batch(loadDetailCmd(path), m.spinner.Tick)
}

func (m Model) handleBranchDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoDetail
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.CopyBranch):
		return m, copyToClipboardCmd(m.branchDetail.Branch.Name)

	case key.Matches(msg, m.keys.OpenURL):
		if m.branchDetail.PRInfo != nil && m.branchDetail.PRInfo.URL != "" {
			return m, openURLCmd(m.branchDetail.PRInfo.URL)
		}

		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m.handleActionKey(msg)
}

func (m Model) detailListLen() int {
	return m.panelRowCount(m.focusedPanel)
}

// panelRowCount is how many rows a panel offers the cursor. Status reports
// state rather than listing anything, so it offers none and the column treats
// it as a single stop.
func (m Model) panelRowCount(id panelID) int {
	switch id {
	case panelBranches:
		return len(m.branches)
	case panelStashes:
		return len(m.stashes)
	case panelPeers:
		return len(m.RepoCheckouts())
	case panelPRs:
		return len(m.prs)
	case panelNotes:
		return len(m.notesFiles)
	case panelStatus:
		return 0
	}

	return 0
}

func (m Model) handleRefresh() (Model, tea.Cmd) {
	var cmds []tea.Cmd

	cmds = append(cmds, func() tea.Msg {
		cache.ClearAll()
		return RefreshCompleteMsg{ViewMode: m.viewMode}
	})

	switch m.viewMode {
	case ViewModeRepoList:
		// Clear all data including downstream views
		m.loading = true
		m.summaries = make(map[string]models.RepoSummary)
		m.prCount = make(map[string]int)
		// Every fetch is reissued from discovery, so the old bookkeeping would
		// otherwise suppress the new CI requests and leave phantom pending cells.
		m.fetching = nil
		m.ciRequested = nil
		m.branches = nil
		m.stashes = nil
		m.worktrees = nil
		m.prs = nil
		m.branchDetail = models.BranchDetail{}
		m.prDetail = models.PRDetail{}
		cmds = append(cmds, discoverReposCmd(m.scanPaths, m.maxDepth))

	case ViewModeRepoDetail:
		// Clear detail views when refreshing repo detail
		m.branches = nil
		m.stashes = nil
		m.worktrees = nil
		m.prs = nil
		m.notesFiles = nil
		m.branchDetail = models.BranchDetail{}
		m.prDetail = models.PRDetail{}
		m.detailLoading = true

		if m.selectedRepo != "" {
			cmds = append(cmds, loadDetailCmd(m.selectedRepo))
			if summary, ok := m.summaries[m.selectedRepo]; ok && summary.Upstream != "" {
				cmds = append(cmds, loadPRCountCmd(m.selectedRepo, summary.RemoteID, summary.Upstream))
			}
		}

	case ViewModeBranchDetail:
		// Clear branch detail when refreshing
		m.branchDetail = models.BranchDetail{}
		m.branchDetailLoading = true

		if m.selectedRepo != "" && m.selectedBranch.Name != "" {
			cmds = append(cmds, loadBranchDetailCmd(m.selectedRepo, m.selectedBranch.Name))
		}

	case ViewModePRDetail:
		// Clear PR detail when refreshing
		m.prDetail = models.PRDetail{}

		if m.selectedRepo != "" && m.selectedPR.Number > 0 {
			cmds = append(cmds,
				loadPRDetailCmd(m.selectedRepo, m.summaries[m.selectedRepo].RemoteID, m.selectedPR.Number))
		}

	default:
		// no per-view refresh behavior for this view
	}

	if m.anyLoading() {
		cmds = append(cmds, m.spinner.Tick)
	}

	return m, tea.Batch(cmds...)
}

// moveToAdjacentPR switches the PR-detail view to the PR delta positions away
// from the currently selected one in m.prs, loading its detail and
// prefetching the next one in the same direction.
func (m Model) moveToAdjacentPR(delta int) (tea.Model, tea.Cmd) {
	currentIdx := slices.IndexFunc(m.prs, func(pr models.PRInfo) bool {
		return pr.Number == m.selectedPR.Number
	})

	newIdx := currentIdx + delta
	if currentIdx == -1 || newIdx < 0 || newIdx >= len(m.prs) {
		return m, nil
	}

	m.selectedPR = m.prs[newIdx]
	m.prDetail = models.PRDetail{PRInfo: m.selectedPR}

	cmds := []tea.Cmd{loadPRDetailCmd(m.selectedRepo, m.summaries[m.selectedRepo].RemoteID, m.selectedPR.Number)}

	if prefetchIdx := newIdx + delta; prefetchIdx >= 0 && prefetchIdx < len(m.prs) {
		cmds = append(cmds,
			prefetchPRDetailCmd(m.selectedRepo, m.summaries[m.selectedRepo].RemoteID, m.prs[prefetchIdx].Number))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handlePRDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoDetail
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Up):
		return m.moveToAdjacentPR(-1)

	case key.Matches(msg, m.keys.Down):
		return m.moveToAdjacentPR(1)

	case key.Matches(msg, m.keys.OpenURL):
		if m.prDetail.URL != "" {
			return m, openURLCmd(m.prDetail.URL)
		}

		return m, nil

	case key.Matches(msg, m.keys.CopyURL):
		if m.prDetail.URL != "" {
			return m, copyToClipboardCmd(m.prDetail.URL)
		}

		return m, nil

	case key.Matches(msg, m.keys.CopyPRNumber):
		prNum := fmt.Sprintf("#%d", m.prDetail.Number)
		return m, copyToClipboardCmd(prNum)

	case key.Matches(msg, m.keys.CopyBranch):
		if m.prDetail.HeadRef != "" {
			return m, copyToClipboardCmd(m.prDetail.HeadRef)
		}

		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m.handleActionKey(msg)
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	modes := models.SelectableFilterModes()

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoList
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.filterCursor > 0 {
			m.filterCursor--
		}

		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.filterCursor < len(modes)-1 {
			m.filterCursor++
		}

		return m, nil

	case key.Matches(msg, m.keys.Enter):
		selectedMode := modes[m.filterCursor]
		m.CycleFilterState(selectedMode)
		m.updateFilteredPaths()
		m.cursor = 0

		return m, nil

	case msg.String() == "*":
		m.ResetFilters()
		m.updateFilteredPaths()
		m.cursor = 0

		return m, nil

	default:
		for _, mode := range modes {
			if msg.String() == mode.ShortKey() {
				m.CycleFilterState(mode)
				m.updateFilteredPaths()
				m.cursor = 0

				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) handleSortKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	modes := models.AllSortModes()

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoList
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.sortCursor > 0 {
			m.sortCursor--
		}

		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.sortCursor < len(modes)-1 {
			m.sortCursor++
		}

		return m, nil

	case key.Matches(msg, m.keys.Enter):
		selectedMode := modes[m.sortCursor]
		m.CycleSortState(selectedMode)
		m.updateFilteredPaths()

		return m, nil

	case msg.String() == "[":
		m.MoveSortUp()
		m.updateFilteredPaths()

		return m, nil

	case msg.String() == "]":
		m.MoveSortDown()
		m.updateFilteredPaths()

		return m, nil

	case msg.String() == "*":
		m.ResetSorts()
		m.updateFilteredPaths()

		return m, nil

	default:
		for _, mode := range modes {
			if msg.String() == mode.ShortKey() {
				m.CycleSortState(mode)
				m.updateFilteredPaths()

				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) handleBatchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		if !m.batchRunning {
			return m, tea.Quit
		}

		return m, nil

	case key.Matches(msg, m.keys.Back):
		if !m.batchRunning {
			m.viewMode = ViewModeRepoList
		}

		return m, nil
	}

	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		m.searching = false
		m.searchInput.Blur()

		return m, nil

	case keyEnter:
		m.searching = false
		m.searchText = m.searchInput.Value()
		m.searchInput.Blur()
		m.updateFilteredPaths()
		m.cursor = 0

		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchText = m.searchInput.Value()
	m.updateFilteredPaths()
	m.cursor = 0

	return m, cmd
}

func (m Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		m.commandMode = false
		m.commandInput.Blur()

		return m, nil

	case keyEnter:
		line := m.commandInput.Value()
		m.commandMode = false
		m.commandInput.Blur()

		return m.ExecuteCommand(line)

	case "ctrl+c":
		return m, tea.Quit

	case keyTab:
		m.completeCommand()
		return m, nil
	}

	m.completionCandidates = nil
	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)

	return m, cmd
}

// commandCompletionCandidates computes the completion candidates for the
// token under the cursor: command names if completing the first word, or
// that command's own Complete func for its arguments. The bool is false if
// the first word doesn't resolve to a completable command.
func (m Model) commandCompletionCandidates() ([]string, bool) {
	line := m.commandInput.Value()
	fields := strings.Fields(line)
	endsWithSpace := strings.HasSuffix(line, " ")

	if len(fields) == 0 || (len(fields) == 1 && !endsWithSpace) {
		prefix := ""
		if len(fields) == 1 {
			prefix = fields[0]
		}

		return m.registry.Candidates(prefix), true
	}

	cmd, found := m.registry.Lookup(fields[0])
	if !found || cmd.Complete == nil {
		return nil, false
	}

	args := fields[1:]
	if endsWithSpace {
		args = append(args, "")
	}

	return cmd.Complete(m, args), true
}

// completeCommand cycles through completion candidates for the token
// under the cursor; the candidate set is pinned on first tab press.
func (m *Model) completeCommand() {
	if m.completionCandidates == nil {
		candidates, ok := m.commandCompletionCandidates()
		if !ok {
			return
		}
		m.completionCandidates = candidates
		m.completionIndex = 0
	} else {
		m.completionIndex = (m.completionIndex + 1) % len(m.completionCandidates)
	}

	if len(m.completionCandidates) == 0 {
		m.completionCandidates = nil
		return
	}

	line := m.commandInput.Value()
	fields := strings.Fields(line)
	candidate := m.completionCandidates[m.completionIndex]

	var newLine string
	switch {
	case len(fields) == 0:
		newLine = candidate
	case strings.HasSuffix(line, " "):
		newLine = strings.Join(fields, " ") + " " + candidate
	default:
		newLine = strings.Join(append(fields[:len(fields)-1], candidate), " ")
	}

	m.commandInput.SetValue(newLine)
	m.commandInput.CursorEnd()
}

func (m *Model) updateFilteredPaths() {
	m.filteredPaths = filters.FilterAndSortMulti(
		m.listableRepos(),
		m.summaries,
		m.activeFilters,
		m.activeSorts,
		m.searchText,
	)

	if m.predicate != nil {
		var matched []string
		for _, path := range m.filteredPaths {
			if summary, ok := m.summaries[path]; ok && m.predicate(summary) {
				matched = append(matched, path)
			}
		}
		m.filteredPaths = matched
	}

	if m.cursor >= len(m.filteredPaths) {
		if len(m.filteredPaths) > 0 {
			m.cursor = len(m.filteredPaths) - 1
		} else {
			m.cursor = 0
		}
	}
}

// confirmBatchTask gates a batch run that deletes things behind the same
// confirmation single-repo writes get, naming how many repos it covers.
// Read-only tasks start immediately.
func (m Model) confirmBatchTask(
	taskName string, destructive bool, paths []string, taskCmd func([]string) tea.Cmd,
) (Model, tea.Cmd) {
	if !destructive || len(paths) == 0 {
		return m.startBatchTaskOn(taskName, paths, taskCmd)
	}

	scope := fmt.Sprintf("across %d repos", len(paths))
	if len(paths) == 1 {
		scope = "in " + filepath.Base(paths[0])
	}

	return m.confirmRun(taskName+"?", repoNameList(paths), scope, func(m Model) (Model, tea.Cmd) {
		return m.startBatchTaskOn(taskName, paths, taskCmd)
	})
}

// repoNameList names the repos an action covers, abbreviating a long set so
// the confirmation stays one line.
func repoNameList(paths []string) string {
	const shown = 4

	names := make([]string, 0, shown)
	for _, path := range paths[:min(shown, len(paths))] {
		names = append(names, filepath.Base(path))
	}

	list := strings.Join(names, ", ")
	if len(paths) > shown {
		list += fmt.Sprintf(" +%d more", len(paths)-shown)
	}

	return list
}

func (m Model) startBatchTaskOn(taskName string, paths []string, taskCmd func([]string) tea.Cmd) (Model, tea.Cmd) {
	if len(paths) == 0 {
		return m, nil
	}

	m.viewMode = ViewModeBatchProgress
	m.batchRunning = true
	m.batchTask = taskName
	m.batchResults = nil
	m.batchProgress = 0
	m.batchTotal = len(paths)

	return m, taskCmd(paths)
}

// deletableBranches marks local, non-current branches whose tip matches a
// merged pull request's head OID as safe to delete. Best-effort: a missing gh
// yields an empty set rather than failing the detail load.
func deletableBranches(ctx context.Context, path, remoteID string, branches []models.BranchInfo) map[string]bool {
	heads, err := github.GetMergedPRHeads(ctx, path, remoteID)
	if err != nil || len(heads) == 0 {
		return nil
	}

	deletable := make(map[string]bool)
	for _, b := range branches {
		if b.IsCurrent || b.IsRemote || b.Head == "" {
			continue
		}
		if oid, ok := heads[b.Name]; ok && oid == b.Head {
			deletable[b.Name] = true
		}
	}

	return deletable
}

func findDefaultBranch(branches []models.BranchInfo) string {
	for _, branch := range branches {
		if vcs.IsDefaultBranchName(branch.Name) {
			return branch.Name
		}
	}

	return ""
}
