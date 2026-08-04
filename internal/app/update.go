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

		return m, nil

	case spinner.TickMsg:
		return m.handleSpinnerTick(msg)

	case tea.KeyMsg:
		return m.routeKeyMsg(msg)

	case ReposDiscoveredMsg:
		return m.handleReposDiscovered(msg)

	case RepoSummaryLoadedMsg:
		return m.handleRepoSummaryLoaded(msg)

	case PRLoadedMsg:
		if summary, ok := m.summaries[msg.Path]; ok {
			summary.PRInfo = msg.PRInfo
			m.summaries[msg.Path] = summary
		}

		return m, nil

	case WorkflowLoadedMsg:
		if summary, ok := m.summaries[msg.Path]; ok {
			summary.WorkflowInfo = msg.Workflow
			m.summaries[msg.Path] = summary
		}

		return m, nil

	case CopierInfoLoadedMsg:
		if summary, ok := m.summaries[msg.Path]; ok {
			summary.TemplateInfo = msg.Info
			m.summaries[msg.Path] = summary
		}

		return m, nil

	case DetailLoadedMsg:
		return m.handleDetailLoaded(msg)

	case BranchDetailLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.branchDetailLoading = false
			m.branchDetail = msg.Detail
		}

		return m, nil

	case PRListLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.prs = msg.PRs
		}

		return m, nil

	case PRDetailLoadedMsg:
		return m.handlePRDetailLoaded(msg)

	case ActionResultMsg:
		return m.handleActionResult(msg)

	case PRMapLoadedMsg:
		if m.prMap == nil {
			m.prMap = make(map[string]PRMapLoadedMsg)
		}
		m.prMap[msg.Path] = msg

		return m, nil

	case PRCountLoadedMsg:
		if m.prCount == nil {
			m.prCount = make(map[string]int)
		}
		m.prCount[msg.Path] = msg.Count

		return m, nil

	case CopySuccessMsg:
		m.statusMessage = "Copied to clipboard: " + msg.Text
		return m, clearStatusAfterDelay()

	case URLOpenedMsg:
		m.statusMessage = "Opened in browser: " + msg.URL
		return m, clearStatusAfterDelay()

	case StatusMsg:
		m.statusMessage = msg.Message
		return m, nil

	case ClearStatusMsg:
		m.statusMessage = ""
		return m, nil

	case RefreshCompleteMsg:
		m.statusMessage = "Data refreshed"
		return m, clearStatusAfterDelay()

	case batch.TaskProgressMsg:
		m.batchResults = append(m.batchResults, BatchResult{
			Path:    msg.Result.Path,
			Success: msg.Result.Success,
			Message: msg.Result.Message,
		})
		m.batchProgress = len(m.batchResults)

		return m, nil

	case batch.TaskCompleteMsg:
		m.batchRunning = false
		for _, r := range msg.Results {
			m.batchResults = append(m.batchResults, BatchResult{
				Path:    r.Path,
				Success: r.Success,
				Message: r.Message,
			})
		}
		m.batchProgress = len(m.batchResults)

		return m, nil

	case ErrorMsg:
		return m, nil
	}

	return m, nil
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
	if m.pendingRepeat {
		m.pendingRepeat = false
		if key.Matches(msg, m.keys.Command) {
			return m.repeatLastCommand()
		}

		return m, nil
	}
	if key.Matches(msg, m.keys.Command) {
		m.commandMode = true
		m.commandInput.Reset()
		m.completionCandidates = nil
		m.commandInput.Focus()

		return m, nil
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
		m.detailTab = DetailTabBranches
		m.detailCursor = 0
		cmds = append(cmds, loadDetailCmd(m.selectedRepo))
	}

	return m, tea.Batch(cmds...)
}

// handleRepoSummaryLoaded merges a loaded repo summary and, once every repo
// has reported in, ends the initial loading state.
func (m Model) handleRepoSummaryLoaded(msg RepoSummaryLoadedMsg) (tea.Model, tea.Cmd) {
	m.loadedCount++

	var cmds []tea.Cmd
	if msg.Error != nil {
		m.summaries[msg.Path] = models.RepoSummary{
			Path:    msg.Path,
			VCSType: vcs.DetectVCSType(msg.Path),
			Error:   msg.Error,
		}
	} else {
		m.summaries[msg.Path] = msg.Summary
		cmds = append(cmds,
			loadPRCmd(msg.Path, msg.Summary.Branch, msg.Summary.Upstream),
			loadPRCountCmd(msg.Path, msg.Summary.Upstream),
			loadCopierInfoCmd(msg.Path),
		)
	}

	if m.loadedCount >= m.loadingCount {
		m.loading = false
		m.updateFilteredPaths()
	}

	return m, tea.Batch(cmds...)
}

// handlePRDetailLoaded stores a loaded PR detail if it's still for the
// currently selected repo/PR, preserving prior info on error.
func (m Model) handlePRDetailLoaded(msg PRDetailLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Path != m.selectedRepo || msg.PRNumber != m.selectedPR.Number {
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

// handleDetailLoaded stores the loaded repo detail (branches/stashes/
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

// worktrees/PRs) and kicks off background prefetch of the first few PR
// details.
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

	prefetchCount := min(prDetailPrefetchCount, len(msg.PRs))

	var cmds []tea.Cmd
	for i := range prefetchCount {
		cmds = append(cmds, prefetchPRDetailCmd(msg.Path, msg.PRs[i].Number))
	}
	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}

	return m, nil
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
		return newM, nil
	}

	if newM, handled := m.handleModeKey(msg); handled {
		return newM, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Enter):
		return m.handleEnterKey()

	case key.Matches(msg, m.keys.Back):
		return m.handleBackKey()

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Search):
		return m.openSearch()

	case key.Matches(msg, m.keys.NotesPreview):
		m.notesPreviewOpen = !m.notesPreviewOpen
		return m, nil

	case key.Matches(msg, m.keys.Peers):
		return m.openCheckouts()

	case key.Matches(msg, m.keys.FetchAll),
		key.Matches(msg, m.keys.PruneRemote),
		key.Matches(msg, m.keys.CleanupMerged):
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
		m.prMap = nil
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

// openPRMapRepo drills into the repo behind the row under the cursor.
func (m Model) openPRMapRepo(entries []prMapEntry) (tea.Model, tea.Cmd) {
	if m.prMapCursor >= len(entries) {
		return m, nil
	}

	repo := entries[m.prMapCursor].Repo
	for _, path := range m.filteredPaths {
		if filepath.Base(path) == repo {
			m.prMap = nil
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
	m.prMap = make(map[string]PRMapLoadedMsg, len(m.filteredPaths))
	m.prMapCursor = 0

	cmds := make([]tea.Cmd, 0, len(m.filteredPaths)+1)
	for _, path := range m.filteredPaths {
		cmds = append(cmds, loadPRMapCmd(path, m.summaries[path].Upstream))
	}
	cmds = append(cmds, m.spinner.Tick)

	return m, tea.Batch(cmds...)
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
	m.detailTab = DetailTabBranches
	m.detailCursor = 0
	m.branches = nil
	m.stashes = nil
	m.worktrees = nil
	m.prs = nil
	m.notesFiles = nil
	m.detailLoading = true

	return m, tea.Batch(loadDetailCmd(m.selectedRepo), m.spinner.Tick)
}

// handleBackKey pops the current view back to its parent, if it has one.
func (m Model) handleBackKey() (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeRepoDetail:
		// A jump to a peer checkout pushed the repo it came from; walk back to
		// that repo before leaving the focused view entirely.
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
	default:
		// no back transition from this view
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
		return m.startBatchTaskOn(op.TaskName, m.filteredPaths, op.Cmd)
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

	return m.startBatchTaskOn(fmt.Sprintf("%s (%s)", op.TaskName, obj.Name), paths, op.Cmd)
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
	if s, ok := sceneForKey(msg.String()); ok {
		return m.setDetailTab(int(s.tab))
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		return m.handleBackKey()

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Right):
		return m.setDetailTab(int(m.detailTab) + 1)

	case key.Matches(msg, m.keys.Left):
		return m.setDetailTab(int(m.detailTab) - 1)

	case key.Matches(msg, m.keys.Up):
		return m.moveDetailCursor(-1)

	case key.Matches(msg, m.keys.Down):
		return m.moveDetailCursor(1)

	case key.Matches(msg, m.keys.Top):
		m.detailCursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		maxIdx := m.detailListLen() - 1
		if maxIdx >= 0 {
			m.detailCursor = maxIdx
		}

		return m, nil

	case key.Matches(msg, m.keys.Enter):
		return m.handleDetailEnterKey()

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m.handleActionKey(msg)
}

// handleActionKey routes the write-action keys shared by the repo-detail,
// branch-detail, and PR-detail views.
func (m Model) handleActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
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

// setDetailTab switches the repo-detail tab to tabIndex (wrapping around),
// resets the row cursor, and prefetches the first PR if landing on the PR tab.
func (m Model) setDetailTab(tabIndex int) (tea.Model, tea.Cmd) {
	tabIndex = ((tabIndex % detailTabCount) + detailTabCount) % detailTabCount
	m.detailTab = DetailTab(tabIndex)
	m.detailCursor = 0

	if m.detailTab == DetailTabPRs && len(m.prs) > 0 {
		return m, prefetchPRDetailCmd(m.selectedRepo, m.prs[0].Number)
	}

	return m, nil
}

// moveDetailCursor moves the repo-detail row cursor by delta (clamped to the
// current tab's list bounds) and prefetches the newly selected PR's detail.
func (m Model) moveDetailCursor(delta int) (tea.Model, tea.Cmd) {
	newIdx := m.detailCursor + delta
	if newIdx < 0 || newIdx > m.detailListLen()-1 {
		return m, nil
	}

	m.detailCursor = newIdx
	if m.detailTab == DetailTabPRs && m.detailCursor < len(m.prs) {
		return m, prefetchPRDetailCmd(m.selectedRepo, m.prs[m.detailCursor].Number)
	}

	return m, nil
}

// handleDetailEnterKey opens the branch-detail or PR-detail view for the
// currently selected row.
func (m Model) handleDetailEnterKey() (tea.Model, tea.Cmd) {
	switch {
	case m.detailTab == DetailTabBranches && m.detailCursor < len(m.branches):
		m.selectedBranch = m.branches[m.detailCursor]
		m.branchDetail = models.BranchDetail{} // Clear previous detail
		m.branchDetailLoading = true
		m.viewMode = ViewModeBranchDetail

		return m, tea.Batch(loadBranchDetailCmd(m.selectedRepo, m.selectedBranch.Name), m.spinner.Tick)

	case m.detailTab == DetailTabWorktrees:
		return m.jumpToCheckout()

	case m.detailTab == DetailTabPRs && m.detailCursor < len(m.prs):
		m.selectedPR = m.prs[m.detailCursor]
		// Progressive loading: show basic info from the list immediately,
		// full details (author, assignees, etc.) load async.
		m.prDetail = models.PRDetail{PRInfo: m.selectedPR}
		m.viewMode = ViewModePRDetail

		return m, loadPRDetailCmd(m.selectedRepo, m.selectedPR.Number)

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
	opened.detailTab = DetailTabWorktrees

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
	m.detailTab = DetailTabBranches
	m.detailCursor = 0
	m.branches = nil
	m.stashes = nil
	m.worktrees = nil
	m.prs = nil
	m.notesFiles = nil
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
	switch m.detailTab {
	case DetailTabBranches:
		return len(m.branches)
	case DetailTabStashes:
		return len(m.stashes)
	case DetailTabWorktrees:
		return len(m.RepoCheckouts())
	case DetailTabPRs:
		return len(m.prs)
	case DetailTabNotes:
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
				cmds = append(cmds, loadPRCountCmd(m.selectedRepo, summary.Upstream))
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
			cmds = append(cmds, loadPRDetailCmd(m.selectedRepo, m.selectedPR.Number))
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

	cmds := []tea.Cmd{loadPRDetailCmd(m.selectedRepo, m.selectedPR.Number)}

	if prefetchIdx := newIdx + delta; prefetchIdx >= 0 && prefetchIdx < len(m.prs) {
		cmds = append(cmds, prefetchPRDetailCmd(m.selectedRepo, m.prs[prefetchIdx].Number))
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
		m.repoPaths,
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
func deletableBranches(ctx context.Context, path string, branches []models.BranchInfo) map[string]bool {
	heads, err := github.GetMergedPRHeads(ctx, path)
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
