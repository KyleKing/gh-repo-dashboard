package app

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.handleSearchKey(msg)
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
		default:
			return m.handleKey(msg)
		}

	case ReposDiscoveredMsg:
		m.repoPaths = msg.Paths
		m.loadingCount = len(msg.Paths)
		m.loadedCount = 0

		if len(msg.Paths) == 0 {
			m.loading = false
		}

		m.updateFilteredPaths()

		var cmds []tea.Cmd
		for _, path := range msg.Paths {
			cmds = append(cmds, loadRepoSummaryCmd(path))
		}
		return m, tea.Batch(cmds...)

	case RepoSummaryLoadedMsg:
		m.loadedCount++

		var cmds []tea.Cmd
		if msg.Error != nil {
			summary := models.RepoSummary{
				Path:    msg.Path,
				VCSType: vcs.DetectVCSType(msg.Path),
				Error:   msg.Error,
			}
			m.summaries[msg.Path] = summary
		} else {
			m.summaries[msg.Path] = msg.Summary
			cmds = append(cmds, loadPRCmd(msg.Path, msg.Summary.Branch, msg.Summary.Upstream))
			cmds = append(cmds, loadPRCountCmd(msg.Path, msg.Summary.Upstream))
		}

		if m.loadedCount >= m.loadingCount {
			m.loading = false
			m.updateFilteredPaths()
		}

		return m, tea.Batch(cmds...)

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

	case DetailLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.branches = msg.Branches
			m.stashes = msg.Stashes
			m.worktrees = msg.Worktrees
			m.prs = msg.PRs

			// Prefetch first few PR details in background
			var cmds []tea.Cmd
			prefetchCount := 3 // Prefetch first 3 PRs
			if len(msg.PRs) < prefetchCount {
				prefetchCount = len(msg.PRs)
			}
			for i := 0; i < prefetchCount; i++ {
				cmds = append(cmds, prefetchPRDetailCmd(msg.Path, msg.PRs[i].Number))
			}
			if len(cmds) > 0 {
				return m, tea.Batch(cmds...)
			}
		}
		return m, nil

	case BranchDetailLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.branchDetail = msg.Detail
		}
		return m, nil

	case PRListLoadedMsg:
		if msg.Path == m.selectedRepo {
			m.prs = msg.PRs
		}
		return m, nil

	case PRDetailLoadedMsg:
		if msg.Path == m.selectedRepo && msg.PRNumber == m.selectedPR.Number {
			if msg.Error != nil {
				// Don't clear basic info on error - preserve what we already have
				// Show error status message
				m.statusMessage = fmt.Sprintf("Failed to load PR details: %v", msg.Error)
				return m, clearStatusAfterDelay()
			}
			m.prDetail = msg.Detail
		}
		return m, nil

	case PRCountLoadedMsg:
		if m.prCount == nil {
			m.prCount = make(map[string]int)
		}
		m.prCount[msg.Path] = msg.Count
		return m, nil

	case PRCreatedMsg:
		if msg.Error != nil {
			return m, nil
		}
		return m, nil

	case CopySuccessMsg:
		m.statusMessage = fmt.Sprintf("Copied to clipboard: %s", msg.Text)
		return m, clearStatusAfterDelay()

	case URLOpenedMsg:
		m.statusMessage = fmt.Sprintf("Opened in browser: %s", msg.URL)
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

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		if m.viewMode == ViewModeHelp {
			m.viewMode = ViewModeRepoList
		} else {
			m.viewMode = ViewModeHelp
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.filteredPaths)-1 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		if len(m.filteredPaths) > 0 {
			m.cursor = len(m.filteredPaths) - 1
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if m.viewMode == ViewModeRepoList && m.cursor < len(m.filteredPaths) {
			m.selectedRepo = m.filteredPaths[m.cursor]
			m.viewMode = ViewModeRepoDetail
			m.detailTab = DetailTabBranches
			m.detailCursor = 0
			return m, loadDetailCmd(m.selectedRepo)
		}
		return m, nil

	case key.Matches(msg, m.keys.Back):
		switch m.viewMode {
		case ViewModeRepoDetail:
			m.viewMode = ViewModeRepoList
		case ViewModeBranchDetail:
			m.viewMode = ViewModeRepoDetail
		case ViewModeHelp:
			m.viewMode = ViewModeRepoList
		case ViewModeFilter:
			m.viewMode = ViewModeRepoList
		case ViewModeSort:
			m.viewMode = ViewModeRepoList
		}
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Filter):
		m.viewMode = ViewModeFilter
		return m, nil

	case key.Matches(msg, m.keys.Sort):
		m.viewMode = ViewModeSort
		m.sortCursor = 0
		return m, nil

	case key.Matches(msg, m.keys.Search):
		m.searching = true
		m.searchInput.Focus()
		return m, nil


	case key.Matches(msg, m.keys.FetchAll):
		return m.startBatchTask("Fetch All", batchFetchAllCmd)

	case key.Matches(msg, m.keys.PruneRemote):
		return m.startBatchTask("Prune Remote", batchPruneRemoteCmd)

	case key.Matches(msg, m.keys.CleanupMerged):
		return m.startBatchTask("Cleanup Merged", batchCleanupMergedCmd)
	}

	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoList
		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Right):
		m.detailTab = DetailTab((int(m.detailTab) + 1) % 4)
		m.detailCursor = 0

		// Prefetch first PR when switching to PR tab
		if m.detailTab == DetailTabPRs && len(m.prs) > 0 {
			return m, prefetchPRDetailCmd(m.selectedRepo, m.prs[0].Number)
		}
		return m, nil

	case key.Matches(msg, m.keys.Left):
		newTab := int(m.detailTab) - 1
		if newTab < 0 {
			newTab = 3
		}
		m.detailTab = DetailTab(newTab)
		m.detailCursor = 0

		// Prefetch first PR when switching to PR tab
		if m.detailTab == DetailTabPRs && len(m.prs) > 0 {
			return m, prefetchPRDetailCmd(m.selectedRepo, m.prs[0].Number)
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.detailCursor > 0 {
			m.detailCursor--
			// Prefetch PR detail for newly selected item
			if m.detailTab == DetailTabPRs && m.detailCursor < len(m.prs) {
				pr := m.prs[m.detailCursor]
				return m, prefetchPRDetailCmd(m.selectedRepo, pr.Number)
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		maxIdx := m.detailListLen() - 1
		if m.detailCursor < maxIdx {
			m.detailCursor++
			// Prefetch PR detail for newly selected item
			if m.detailTab == DetailTabPRs && m.detailCursor < len(m.prs) {
				pr := m.prs[m.detailCursor]
				return m, prefetchPRDetailCmd(m.selectedRepo, pr.Number)
			}
		}
		return m, nil

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
		if m.detailTab == DetailTabBranches && m.detailCursor < len(m.branches) {
			m.selectedBranch = m.branches[m.detailCursor]
			m.branchDetail = models.BranchDetail{} // Clear previous detail
			m.viewMode = ViewModeBranchDetail
			return m, loadBranchDetailCmd(m.selectedRepo, m.selectedBranch.Name)
		} else if m.detailTab == DetailTabPRs && m.detailCursor < len(m.prs) {
			m.selectedPR = m.prs[m.detailCursor]
			// Progressive loading: Show basic info from list immediately
			m.prDetail = models.PRDetail{
				PRInfo: m.selectedPR, // Use data already loaded from list
				// Full details (author, assignees, etc.) will load async
			}
			m.viewMode = ViewModePRDetail
			return m, loadPRDetailCmd(m.selectedRepo, m.selectedPR.Number)
		}
		return m, nil

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m, nil
}

func (m Model) handleBranchDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoDetail
		return m, nil

	case key.Matches(msg, m.keys.OpenPR):
		return m, openOrCreatePRCmd(m.selectedRepo, m.branchDetail.Branch.Name)

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

	return m, nil
}

func (m Model) detailListLen() int {
	switch m.detailTab {
	case DetailTabBranches:
		return len(m.branches)
	case DetailTabStashes:
		return len(m.stashes)
	case DetailTabWorktrees:
		return len(m.worktrees)
	case DetailTabPRs:
		return len(m.prs)
	}
	return 0
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
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
		m.branchDetail = models.BranchDetail{}
		m.prDetail = models.PRDetail{}

		if m.selectedRepo != "" {
			cmds = append(cmds, loadDetailCmd(m.selectedRepo))
			if summary, ok := m.summaries[m.selectedRepo]; ok && summary.Upstream != "" {
				cmds = append(cmds, loadPRCountCmd(m.selectedRepo, summary.Upstream))
			}
		}

	case ViewModeBranchDetail:
		// Clear branch detail when refreshing
		m.branchDetail = models.BranchDetail{}

		if m.selectedRepo != "" && m.selectedBranch.Name != "" {
			cmds = append(cmds, loadBranchDetailCmd(m.selectedRepo, m.selectedBranch.Name))
		}

	case ViewModePRDetail:
		// Clear PR detail when refreshing
		m.prDetail = models.PRDetail{}

		if m.selectedRepo != "" && m.selectedPR.Number > 0 {
			cmds = append(cmds, loadPRDetailCmd(m.selectedRepo, m.selectedPR.Number))
		}
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

	case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down):
		// Navigate to adjacent PR
		currentIdx := -1
		for i, pr := range m.prs {
			if pr.Number == m.selectedPR.Number {
				currentIdx = i
				break
			}
		}

		if currentIdx != -1 {
			var newIdx int
			if key.Matches(msg, m.keys.Up) && currentIdx > 0 {
				newIdx = currentIdx - 1
			} else if key.Matches(msg, m.keys.Down) && currentIdx < len(m.prs)-1 {
				newIdx = currentIdx + 1
			} else {
				return m, nil
			}

			// Switch to adjacent PR
			m.selectedPR = m.prs[newIdx]
			m.prDetail = models.PRDetail{
				PRInfo: m.selectedPR,
			}

			var cmds []tea.Cmd
			cmds = append(cmds, loadPRDetailCmd(m.selectedRepo, m.selectedPR.Number))

			// Prefetch next adjacent PR
			if key.Matches(msg, m.keys.Down) && newIdx+1 < len(m.prs) {
				cmds = append(cmds, prefetchPRDetailCmd(m.selectedRepo, m.prs[newIdx+1].Number))
			} else if key.Matches(msg, m.keys.Up) && newIdx-1 >= 0 {
				cmds = append(cmds, prefetchPRDetailCmd(m.selectedRepo, m.prs[newIdx-1].Number))
			}

			return m, tea.Batch(cmds...)
		}
		return m, nil

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

	return m, nil
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
	switch msg.Type {
	case tea.KeyEsc:
		m.searching = false
		m.searchInput.Blur()
		return m, nil

	case tea.KeyEnter:
		m.searching = false
		m.searchText = m.searchInput.Value()
		m.searchInput.Blur()
		m.updateFilteredPaths()
		m.cursor = 0
		return m, nil

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchText = m.searchInput.Value()
	m.updateFilteredPaths()
	m.cursor = 0
	return m, cmd
}

func (m *Model) updateFilteredPaths() {
	m.filteredPaths = filters.FilterAndSortMulti(
		m.repoPaths,
		m.summaries,
		m.activeFilters,
		m.activeSorts,
		m.searchText,
	)

	if m.cursor >= len(m.filteredPaths) {
		if len(m.filteredPaths) > 0 {
			m.cursor = len(m.filteredPaths) - 1
		} else {
			m.cursor = 0
		}
	}
}

func (m Model) startBatchTask(taskName string, taskCmd func([]string) tea.Cmd) (tea.Model, tea.Cmd) {
	if len(m.filteredPaths) == 0 {
		return m, nil
	}

	m.viewMode = ViewModeBatchProgress
	m.batchRunning = true
	m.batchTask = taskName
	m.batchResults = nil
	m.batchProgress = 0
	m.batchTotal = len(m.filteredPaths)

	return m, taskCmd(m.filteredPaths)
}

func discoverReposCmd(scanPaths []string, maxDepth int) tea.Cmd {
	return func() tea.Msg {
		paths := discovery.DiscoverRepos(scanPaths, maxDepth)
		return ReposDiscoveredMsg{Paths: paths}
	}
}

func loadRepoSummaryCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ops := vcs.GetOperations(path)
		summary, err := ops.GetRepoSummary(context.Background(), path)
		return RepoSummaryLoadedMsg{
			Path:    path,
			Summary: summary,
			Error:   err,
		}
	}
}

func loadPRCmd(path string, branch string, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}
	return func() tea.Msg {
		return PRLoadedMsg{Path: path, PRInfo: nil}
	}
}

func loadDetailCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		ops := vcs.GetOperations(path)

		branches, _ := ops.GetBranchList(ctx, path)
		stashes, _ := ops.GetStashList(ctx, path)
		worktrees, _ := ops.GetWorktreeList(ctx, path)

		summary, _ := ops.GetRepoSummary(ctx, path)
		var prs []models.PRInfo
		if summary.Upstream != "" {
			prs, _ = github.GetPRsForRepo(ctx, path, summary.Upstream)
		}

		return DetailLoadedMsg{
			Path:      path,
			Branches:  branches,
			Stashes:   stashes,
			Worktrees: worktrees,
			PRs:       prs,
		}
	}
}

func loadBranchDetailCmd(repoPath string, branchName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		ops := vcs.GetOperations(repoPath)

		branches, _ := ops.GetBranchList(ctx, repoPath)
		var selectedBranch models.BranchInfo
		for _, b := range branches {
			if b.Name == branchName {
				selectedBranch = b
				break
			}
		}

		commits, _ := ops.GetCommitLog(ctx, repoPath, 20)

		summary, _ := ops.GetRepoSummary(ctx, repoPath)

		detail := models.BranchDetail{
			Branch:       selectedBranch,
			Commits:      commits,
			Staged:       summary.Staged,
			Unstaged:     summary.Unstaged,
			Untracked:    summary.Untracked,
			Conflicted:   summary.Conflicted,
			PRInfo:       summary.PRInfo,
			WorkflowInfo: summary.WorkflowInfo,
		}

		if vcsType := vcs.DetectVCSType(repoPath); vcsType == models.VCSTypeJJ {
		}

		return BranchDetailLoadedMsg{
			Path:   repoPath,
			Detail: detail,
		}
	}
}

func loadPRCountCmd(path string, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		count, err := github.GetPRCount(ctx, path, upstream)
		if err != nil {
			return PRCountLoadedMsg{Path: path, Count: 0}
		}
		return PRCountLoadedMsg{Path: path, Count: count}
	}
}

func loadPRListCmd(path string, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		prs, err := github.GetPRsForRepo(ctx, path, upstream)
		return PRListLoadedMsg{
			Path:  path,
			PRs:   prs,
			Error: err,
		}
	}
}

func loadPRDetailCmd(repoPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		detail, err := github.GetPRDetail(ctx, repoPath, prNumber)
		if err != nil {
			return PRDetailLoadedMsg{
				Path:     repoPath,
				PRNumber: prNumber,
				Error:    err,
			}
		}
		return PRDetailLoadedMsg{
			Path:     repoPath,
			PRNumber: prNumber,
			Detail:   *detail,
		}
	}
}

func prefetchPRDetailCmd(repoPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// Prefetch runs in background and populates cache
		// No message sent to avoid UI updates during prefetch
		_, _ = github.GetPRDetail(ctx, repoPath, prNumber)
		return nil
	}
}

func openOrCreatePRCmd(repoPath string, branchName string) tea.Cmd {
	return func() tea.Msg {
		return PRCreatedMsg{
			URL:   "",
			Error: nil,
		}
	}
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "linux":
			cmd = exec.Command("sh", "-c", "type xclip >/dev/null 2>&1 && xclip -selection clipboard || type xsel >/dev/null 2>&1 && xsel --clipboard --input || type wl-copy >/dev/null 2>&1 && wl-copy")
		case "windows":
			cmd = exec.Command("clip")
		default:
			return StatusMsg{Message: "Clipboard not supported on this platform"}
		}

		if cmd != nil {
			stdin, err := cmd.StdinPipe()
			if err != nil {
				return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
			}

			if err := cmd.Start(); err != nil {
				return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
			}

			if _, err := stdin.Write([]byte(text)); err != nil {
				return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
			}

			if err := stdin.Close(); err != nil {
				return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
			}

			if err := cmd.Wait(); err != nil {
				return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
			}
		}

		return CopySuccessMsg{Text: text}
	}
}

func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			return StatusMsg{Message: "URL opening not supported on this platform"}
		}

		if err := cmd.Start(); err != nil {
			return StatusMsg{Message: fmt.Sprintf("Failed to open URL: %v", err)}
		}

		return URLOpenedMsg{URL: url}
	}
}

func clearStatusAfterDelay() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}
