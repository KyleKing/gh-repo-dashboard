package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.viewMode {
	case ViewModeHelp:
		return m.renderHelp()
	case ViewModeRepoDetail:
		return m.renderRepoDetail()
	case ViewModeBranchDetail:
		return m.renderBranchDetail()
	case ViewModePRDetail:
		return m.renderPRDetail()
	case ViewModeFilter:
		return m.renderFilterModal()
	case ViewModeSort:
		return m.renderSortModal()
	case ViewModeBatchProgress:
		return m.renderBatchProgress()
	default:
		return m.renderRepoList()
	}
}

func (m Model) renderRepoList() string {
	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n\n")

	if m.searching {
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	}

	b.WriteString(m.renderTable())

	footer := m.renderFooter()
	footerHeight := 1
	tableLines := strings.Count(b.String(), "\n")
	paddingNeeded := m.height - tableLines - footerHeight - 1
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(footer)

	return b.String()
}

func (m Model) renderBreadcrumbs() string {
	switch m.viewMode {
	case ViewModeRepoDetail:
		summary, ok := m.summaries[m.selectedRepo]
		if !ok {
			return styles.TitleStyle.Render("repo-dashboard")
		}

		home := styles.SubtitleStyle.Render("Repos")
		sep := styles.SubtitleStyle.Render(" > ")
		repo := styles.TitleStyle.Render(summary.Name())

		var badges []string
		badges = append(badges, styles.Badge(summary.VCSType.String(), styles.CountBadgeStyle))
		if summary.IsDirty() {
			badges = append(badges, styles.Badge("dirty", styles.FilterBadgeStyle))
		}
		if summary.PRInfo != nil {
			badges = append(badges, styles.Badge(fmt.Sprintf("PR #%d", summary.PRInfo.Number), styles.PROpenStyle))
		}

		return home + sep + repo + "  " + strings.Join(badges, " ")

	case ViewModeBranchDetail:
		summary, ok := m.summaries[m.selectedRepo]
		if !ok {
			return styles.TitleStyle.Render("repo-dashboard")
		}

		home := styles.SubtitleStyle.Render("Repos")
		sep := styles.SubtitleStyle.Render(" > ")
		repo := styles.BranchStyle.Render(summary.Name())
		branch := styles.TitleStyle.Render(m.branchDetail.Branch.Name)

		var badges []string
		if m.branchDetail.Branch.IsCurrent {
			badges = append(badges, styles.Badge("current", styles.PROpenStyle))
		}
		if m.branchDetail.Branch.Ahead > 0 {
			badges = append(badges, styles.Badge(fmt.Sprintf("↑%d", m.branchDetail.Branch.Ahead), styles.AheadStyle))
		}
		if m.branchDetail.Branch.Behind > 0 {
			badges = append(badges, styles.Badge(fmt.Sprintf("↓%d", m.branchDetail.Branch.Behind), styles.BehindStyle))
		}

		return home + sep + repo + sep + branch + "  " + strings.Join(badges, " ")

	default:
		title := styles.TitleStyle.Render("repo-dashboard")

		badges := []string{}

		repoCount := fmt.Sprintf("%d repos", len(m.filteredPaths))
		if len(m.filteredPaths) != len(m.repoPaths) {
			repoCount = fmt.Sprintf("%d/%d repos", len(m.filteredPaths), len(m.repoPaths))
		}
		badges = append(badges, styles.Badge(repoCount, styles.CountBadgeStyle))

		if dirtyCount := m.DirtyCount(); dirtyCount > 0 {
			badges = append(badges, styles.Badge(fmt.Sprintf("%d dirty", dirtyCount), styles.FilterBadgeStyle))
		}

		if prCount := m.PRCount(); prCount > 0 {
			badges = append(badges, styles.Badge(fmt.Sprintf("%d PRs", prCount), styles.PROpenStyle))
		}

		if m.loading {
			progress := fmt.Sprintf("Loading %d/%d", m.loadedCount, m.loadingCount)
			badges = append(badges, styles.Badge(progress, styles.CountBadgeStyle))
		}

		return title + "  " + strings.Join(badges, " ")
	}
}

func (m Model) renderStatusBar() string {
	parts := []string{}

	for _, f := range m.activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			label := f.Mode.String()
			if f.Inverted {
				label = "NOT " + label
			}
			parts = append(parts, styles.Badge(label, styles.FilterBadgeStyle))
		}
	}

	enabledSorts := []models.ActiveSort{}
	for _, s := range m.activeSorts {
		if s.IsEnabled() {
			enabledSorts = append(enabledSorts, s)
		}
	}

	if len(enabledSorts) > 0 {
		for i := 0; i < len(enabledSorts); i++ {
			for _, s := range m.activeSorts {
				if s.IsEnabled() && s.Priority == i {
					parts = append(parts, styles.Badge(s.DisplayName(), styles.SortBadgeStyle))
					break
				}
			}
		}
	}

	if m.searchText != "" {
		parts = append(parts, styles.Badge("\""+m.searchText+"\"", styles.SearchBadgeStyle))
	}

	return strings.Join(parts, " ")
}

func (m Model) renderTable() string {
	if len(m.filteredPaths) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(2, 4).
			Foreground(styles.Subtext0)

		if m.loading {
			return emptyStyle.Render("Discovering repositories...")
		}
		return emptyStyle.Render("No repositories found")
	}

	colWidths := struct {
		name     int
		branch   int
		status   int
		pr       int
		prs      int
		modified int
	}{
		name:     20,
		branch:   15,
		status:   12,
		pr:       12,
		prs:      6,
		modified: 12,
	}

	header := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s  %s",
		colWidths.name, "NAME",
		colWidths.branch, "BRANCH",
		colWidths.status, "STATUS",
		colWidths.pr, "PR",
		colWidths.prs, "PRs",
		"MODIFIED",
	)
	header = styles.HeaderStyle.Render(header)

	availableHeight := m.height - 6
	if m.searching {
		availableHeight--
	}

	startIdx := m.cursor - availableHeight/2
	if startIdx < 0 {
		startIdx = 0
	}

	endIdx := startIdx + availableHeight
	if endIdx > len(m.filteredPaths) {
		endIdx = len(m.filteredPaths)
		if endIdx-availableHeight >= 0 {
			startIdx = endIdx - availableHeight
		}
	}

	var rows []string
	rows = append(rows, header)

	for i := startIdx; i < endIdx; i++ {
		path := m.filteredPaths[i]
		summary := m.summaries[path]
		row := m.renderTableRow(summary, i == m.cursor, colWidths)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderTableRow(s models.RepoSummary, selected bool, colWidths struct {
	name     int
	branch   int
	status   int
	pr       int
	prs      int
	modified int
}) string {
	cursor := "  "
	if selected {
		cursor = "> "
	}

	name := truncate(s.Name(), colWidths.name)
	branch := truncate(s.Branch, colWidths.branch)
	status := s.StatusSummary()
	pr := "—"
	if s.PRInfo != nil {
		// Show PR number with review and CI indicators
		prNum := fmt.Sprintf("#%d", s.PRInfo.Number)

		// Add review status indicator
		reviewStatus := s.PRInfo.ReviewStatus()
		if reviewStatus == "approved" {
			prNum += " ✓"
		} else if reviewStatus == "changes requested" {
			prNum += " ✗"
		}

		// Add CI status indicator
		if s.PRInfo.Checks.Total > 0 {
			checkStatus := s.PRInfo.Checks.Summary()
			if checkStatus == "failing" {
				prNum += " ⚠"
			}
		} else if s.WorkflowInfo != nil {
			wfStatus := s.WorkflowInfo.StatusDisplay()
			if wfStatus == "failing" {
				prNum += " ⚠"
			}
		}

		pr = prNum
	}

	prCountStr := "—"
	if count, ok := m.prCount[s.Path]; ok && count > 0 {
		prCountStr = fmt.Sprintf("%d", count)
	}

	modified := s.RelativeModified()

	var style lipgloss.Style
	if selected {
		style = styles.SelectedRowStyle
	} else {
		style = styles.TableRowStyle
	}

	nameStyle := style
	branchStyle := styles.BranchStyle
	if selected {
		branchStyle = branchStyle.Background(styles.Surface0)
	}

	var statusStyle lipgloss.Style
	switch {
	case s.IsDirty():
		statusStyle = styles.DirtyStyle
	case s.Status() == models.RepoStatusClean:
		statusStyle = styles.CleanStyle
	default:
		statusStyle = style
	}
	if selected {
		statusStyle = statusStyle.Background(styles.Surface0)
	}

	prStyle := style
	if s.PRInfo != nil {
		prStyle = styles.PROpenStyle
		if selected {
			prStyle = prStyle.Background(styles.Surface0)
		}
	}

	formattedName := fmt.Sprintf("%-*s", colWidths.name, name)
	formattedBranch := fmt.Sprintf("%-*s", colWidths.branch, branch)
	formattedStatus := fmt.Sprintf("%-*s", colWidths.status, status)
	formattedPR := fmt.Sprintf("%-*s", colWidths.pr, pr)
	formattedPRCount := fmt.Sprintf("%-*s", colWidths.prs, prCountStr)

	row := fmt.Sprintf("%s%s  %s  %s  %s  %s  %s",
		cursor,
		nameStyle.Render(formattedName),
		branchStyle.Render(formattedBranch),
		statusStyle.Render(formattedStatus),
		prStyle.Render(formattedPR),
		style.Render(formattedPRCount),
		style.Render(modified),
	)

	return row
}

func (m Model) renderFooter() string {
	bindings := []struct {
		key  string
		desc string
	}{
		{"j/k", "nav"},
		{"enter", "select"},
		{"f", "filter"},
		{"s", "sort"},
		{"/", "search"},
		{"r", "refresh"},
		{"?", "help"},
		{"q", "quit"},
	}

	var parts []string
	for _, b := range bindings {
		parts = append(parts,
			styles.FooterKeyStyle.Render(b.key)+
				styles.FooterDescStyle.Render(" "+b.desc))
	}

	return strings.Join(parts, "  ")
}

func (m Model) renderHelp() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Help"))
	b.WriteString("\n\n")

	sectionStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		Bold(true).
		PaddingLeft(1)

	sections := []struct {
		title string
		keys  []struct{ key, desc string }
	}{
		{
			"Navigation",
			[]struct{ key, desc string }{
				{"j/k, Up/Down", "Move up/down"},
				{"h/l, Left/Right", "Switch tabs (detail view)"},
				{"g/G", "Go to top/bottom"},
				{"enter, space", "Select/enter"},
				{"esc, backspace", "Go back"},
				{"tab", "Next tab (detail view)"},
			},
		},
		{
			"Filtering & Sorting",
			[]struct{ key, desc string }{
				{"f", "Filter menu (enter/key cycles, *=reset)"},
				{"s", "Sort menu (enter/key cycles, [/]=reorder, *=reset)"},
				{"/", "Search repositories"},
			},
		},
		{
			"Batch Actions",
			[]struct{ key, desc string }{
				{"F", "Fetch all (filtered repos)"},
				{"P", "Prune remote (filtered repos)"},
				{"C", "Cleanup merged (filtered repos)"},
			},
		},
		{
			"General",
			[]struct{ key, desc string }{
				{"r/ctrl+r", "Refresh all data (clears cache)"},
				{"?", "Toggle help"},
				{"q, ctrl+c", "Quit"},
			},
		},
	}

	for _, section := range sections {
		b.WriteString(sectionStyle.Render(section.title))
		b.WriteString("\n")
		for _, k := range section.keys {
			b.WriteString(fmt.Sprintf("  %s  %s\n",
				styles.HelpKeyStyle.Render(fmt.Sprintf("%-20s", k.key)),
				styles.HelpDescStyle.Render(k.desc)))
		}
		b.WriteString("\n")
	}

	contentLines := strings.Count(b.String(), "\n")
	footerHeight := 1
	paddingNeeded := m.height - contentLines - footerHeight - 1
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(styles.FooterStyle.Render("Press ? or esc to close"))

	return b.String()
}

func (m Model) renderRepoDetail() string {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return "Repository not found"
	}

	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render(summary.Path))
	b.WriteString("\n\n")

	b.WriteString(m.renderDetailTabs())
	b.WriteString("\n\n")

	switch m.detailTab {
	case DetailTabBranches:
		b.WriteString(m.renderBranchList())
	case DetailTabStashes:
		b.WriteString(m.renderStashList())
	case DetailTabWorktrees:
		b.WriteString(m.renderWorktreeList())
	case DetailTabPRs:
		b.WriteString(m.renderPRList())
	}

	footer := "tab: switch tabs  j/k: navigate  esc: back"
	if m.detailTab == DetailTabBranches {
		footer = "tab: switch tabs  j/k: navigate  enter: view branch  esc: back"
	} else if m.detailTab == DetailTabPRs {
		footer = "tab: switch tabs  j/k: navigate  enter: view PR  esc: back"
	}

	contentLines := strings.Count(b.String(), "\n")
	footerHeight := 1
	paddingNeeded := m.height - contentLines - footerHeight - 1
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

func (m Model) renderDetailTabs() string {
	summary, _ := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	worktreeLabel := "Worktrees"
	if isJJ {
		worktreeLabel = "Workspaces"
	}

	tabs := []struct {
		name  string
		tab   DetailTab
		count int
	}{
		{"Branches", DetailTabBranches, len(m.branches)},
		{"Stashes", DetailTabStashes, len(m.stashes)},
		{worktreeLabel, DetailTabWorktrees, len(m.worktrees)},
		{"PRs", DetailTabPRs, len(m.prs)},
	}

	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("%s (%d)", t.name, t.count)
		if t.tab == m.detailTab {
			parts = append(parts, styles.TabActiveStyle.Render(label))
		} else {
			parts = append(parts, styles.TabInactiveStyle.Render(label))
		}
	}

	tabRow := strings.Join(parts, styles.TabSeparatorStyle.Render(" │ "))

	ruleWidth := lipgloss.Width(tabRow)
	rule := styles.SubtitleStyle.Render(strings.Repeat("─", ruleWidth))

	return tabRow + "\n" + rule
}

func (m Model) renderBranchList() string {
	if len(m.branches) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(2, 4).
			Foreground(styles.Subtext0)
		return emptyStyle.Render("No branches found")
	}

	var rows []string
	header := fmt.Sprintf("  %-20s  %-20s  %-10s  %s",
		"BRANCH", "UPSTREAM", "STATUS", "LAST COMMIT")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, branch := range m.branches {
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		name := truncate(branch.Name, 20)
		if branch.IsCurrent {
			name = "* " + name
		}
		upstream := truncate(branch.Upstream, 20)
		status := ""
		if branch.Ahead > 0 {
			status += fmt.Sprintf("↑%d", branch.Ahead)
		}
		if branch.Behind > 0 {
			if status != "" {
				status += " "
			}
			status += fmt.Sprintf("↓%d", branch.Behind)
		}
		if status == "" {
			status = "✓"
		}
		lastCommit := branch.RelativeLastCommit()

		var style lipgloss.Style
		if i == m.detailCursor {
			style = styles.SelectedRowStyle
		} else {
			style = styles.TableRowStyle
		}

		nameStyle := styles.BranchStyle
		if branch.IsCurrent {
			nameStyle = styles.PROpenStyle
		}
		if i == m.detailCursor {
			nameStyle = nameStyle.Background(styles.Surface0)
		}

		formattedName := fmt.Sprintf("%-20s", name)
		formattedUpstream := fmt.Sprintf("%-20s", upstream)
		formattedStatus := fmt.Sprintf("%-10s", status)

		row := fmt.Sprintf("%s%s  %s  %s  %s",
			cursor,
			nameStyle.Render(formattedName),
			style.Render(formattedUpstream),
			style.Render(formattedStatus),
			style.Render(lastCommit),
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderStashList() string {
	if len(m.stashes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(2, 4).
			Foreground(styles.Subtext0)
		return emptyStyle.Render("No stashes found\n\nStashes are only available for git repositories.\nJJ repositories use the working copy change instead.")
	}

	var rows []string
	header := fmt.Sprintf("  %-8s  %-40s  %s",
		"INDEX", "MESSAGE", "DATE")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, stash := range m.stashes {
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		index := fmt.Sprintf("stash@{%d}", stash.Index)
		message := truncate(stash.Message, 40)
		date := stash.RelativeDate()

		var style lipgloss.Style
		if i == m.detailCursor {
			style = styles.SelectedRowStyle
		} else {
			style = styles.TableRowStyle
		}

		formattedIndex := fmt.Sprintf("%-8s", index)
		formattedMessage := fmt.Sprintf("%-40s", message)

		row := fmt.Sprintf("%s%s  %s  %s",
			cursor,
			style.Render(formattedIndex),
			style.Render(formattedMessage),
			style.Render(date),
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderWorktreeList() string {
	summary, _ := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	if len(m.worktrees) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(2, 4).
			Foreground(styles.Subtext0)

		emptyMsg := "No worktrees found\n\nWorktrees allow working on multiple branches simultaneously."
		if isJJ {
			emptyMsg = "No workspaces found\n\nWorkspaces (jj's version of worktrees) allow working on multiple\nchanges simultaneously in separate working directories."
		}
		return emptyStyle.Render(emptyMsg)
	}

	var rows []string
	header := fmt.Sprintf("  %-30s  %-20s  %s",
		"PATH", "BRANCH", "STATUS")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, wt := range m.worktrees {
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		path := truncate(filepath.Base(wt.Path), 30)
		branch := truncate(wt.Branch, 20)
		status := ""
		if wt.IsBare {
			status = "bare"
		}
		if wt.IsLocked {
			if status != "" {
				status += ", "
			}
			status += "locked"
		}
		if status == "" {
			status = "active"
		}

		var style lipgloss.Style
		if i == m.detailCursor {
			style = styles.SelectedRowStyle
		} else {
			style = styles.TableRowStyle
		}

		formattedPath := fmt.Sprintf("%-30s", path)
		formattedBranch := fmt.Sprintf("%-20s", branch)

		branchStyleLocal := styles.BranchStyle
		if i == m.detailCursor {
			branchStyleLocal = branchStyleLocal.Background(styles.Surface0)
		}

		row := fmt.Sprintf("%s%s  %s  %s",
			cursor,
			style.Render(formattedPath),
			branchStyleLocal.Render(formattedBranch),
			style.Render(status),
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderPRList() string {
	if len(m.prs) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(2, 4).
			Foreground(styles.Subtext0)
		return emptyStyle.Render("No open pull requests")
	}

	var rows []string
	header := fmt.Sprintf("  %-8s  %-40s  %-10s  %-18s  %s",
		"NUMBER", "TITLE", "STATE", "REVIEW", "BRANCH")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, pr := range m.prs {
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		number := fmt.Sprintf("#%d", pr.Number)
		title := truncate(pr.Title, 40)
		state := pr.StatusDisplay()
		review := pr.ReviewStatus()
		branch := truncate(pr.HeadRef, 20)

		var rowStyle lipgloss.Style
		if i == m.detailCursor {
			rowStyle = styles.SelectedRowStyle
		} else {
			rowStyle = styles.TableRowStyle
		}

		stateStyle := styles.PROpenStyle
		if pr.IsDraft {
			stateStyle = styles.PRDraftStyle
		} else if state == "MERGED" {
			stateStyle = styles.PRMergedStyle
		} else if state == "CLOSED" {
			stateStyle = styles.ErrorStyle
		}
		if i == m.detailCursor {
			stateStyle = stateStyle.Background(styles.Surface0)
		}

		reviewStyle := styles.SubtitleStyle
		if review == "approved" {
			reviewStyle = styles.CleanStyle
		} else if review == "changes requested" {
			reviewStyle = styles.ErrorStyle
		}
		if i == m.detailCursor {
			reviewStyle = reviewStyle.Background(styles.Surface0)
		}

		branchStyleLocal := styles.BranchStyle
		if i == m.detailCursor {
			branchStyleLocal = branchStyleLocal.Background(styles.Surface0)
		}

		row := fmt.Sprintf("%s%-8s  %-40s  %s  %-18s  %s",
			cursor,
			rowStyle.Render(number),
			rowStyle.Render(title),
			stateStyle.Render(fmt.Sprintf("%-10s", state)),
			reviewStyle.Render(review),
			branchStyleLocal.Render(branch),
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderFilterModal() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Filter Repositories"))
	b.WriteString("\n\n")

	modes := models.SelectableFilterModes()

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Subtext0).
		Bold(true)

	header := fmt.Sprintf("  %-4s  %-3s  %-15s  %s",
		"", "Key", "Filter", "Count")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, mode := range modes {
		cursor := "  "
		if i == m.filterCursor {
			cursor = "> "
		}

		var filterState models.ActiveFilter
		for _, f := range m.activeFilters {
			if f.Mode == mode {
				filterState = f
				break
			}
		}

		checkbox := "   "
		if filterState.Enabled && filterState.Inverted {
			checkbox = "NOT"
		} else if filterState.Enabled {
			checkbox = " ✓ "
		}

		shortKey := mode.ShortKey()
		label := mode.String()
		count := m.countForFilter(mode)

		var rowStyle lipgloss.Style
		if i == m.filterCursor {
			rowStyle = styles.SelectedRowStyle
		} else {
			rowStyle = styles.TableRowStyle
		}

		checkStyle := lipgloss.NewStyle().Foreground(styles.Green)
		if filterState.Inverted {
			checkStyle = lipgloss.NewStyle().Foreground(styles.Peach)
		}

		keyStyle := lipgloss.NewStyle().
			Foreground(styles.Mauve).
			Bold(true)

		formattedCheck := fmt.Sprintf("%-4s", checkbox)
		formattedKey := fmt.Sprintf("%-3s", shortKey)
		formattedLabel := fmt.Sprintf("%-15s", label)
		formattedCount := fmt.Sprintf("%d", count)

		row := fmt.Sprintf("%s%s  %s  %s  %s",
			cursor,
			checkStyle.Render(formattedCheck),
			keyStyle.Render(formattedKey),
			rowStyle.Render(formattedLabel),
			styles.SubtitleStyle.Render(formattedCount),
		)
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpLines := []string{
		styles.FooterKeyStyle.Render("enter/key") + styles.FooterDescStyle.Render(" cycle (off/on/NOT)"),
		styles.FooterKeyStyle.Render("*") + styles.FooterDescStyle.Render(" reset"),
		styles.FooterKeyStyle.Render("esc") + styles.FooterDescStyle.Render(" close"),
	}
	b.WriteString(strings.Join(helpLines, "  "))

	content := b.String()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) countForFilter(mode models.FilterMode) int {
	count := 0
	for _, s := range m.summaries {
		switch mode {
		case models.FilterModeAll:
			count++
		case models.FilterModeAhead:
			if s.Ahead > 0 {
				count++
			}
		case models.FilterModeBehind:
			if s.Behind > 0 {
				count++
			}
		case models.FilterModeDirty:
			if s.IsDirty() {
				count++
			}
		case models.FilterModeHasPR:
			if s.PRInfo != nil {
				count++
			}
		case models.FilterModeHasStash:
			if s.StashCount > 0 {
				count++
			}
		}
	}
	return count
}

func (m Model) renderSortModal() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Sort Repositories"))
	b.WriteString("\n\n")

	sortsByPriority := make([]models.ActiveSort, 0)
	for _, s := range m.activeSorts {
		if s.IsEnabled() {
			sortsByPriority = append(sortsByPriority, s)
		}
	}

	for i := 0; i < len(sortsByPriority); i++ {
		for j := range sortsByPriority {
			if sortsByPriority[j].Priority == i {
				break
			}
			if j == len(sortsByPriority)-1 {
				for k := range sortsByPriority {
					if sortsByPriority[k].Priority > i {
						sortsByPriority[k].Priority--
					}
				}
			}
		}
	}

	inactiveSorts := make([]models.ActiveSort, 0)
	for _, s := range m.activeSorts {
		if !s.IsEnabled() {
			inactiveSorts = append(inactiveSorts, s)
		}
	}

	displaySorts := append(sortsByPriority, inactiveSorts...)

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Subtext0).
		Bold(true)

	header := fmt.Sprintf("  %-4s  %-3s  %s",
		"", "Key", "Sort By")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	cursorIndex := -1
	for i, s := range displaySorts {
		if s.Mode == m.activeSorts[m.sortCursor].Mode {
			cursorIndex = i
			break
		}
	}

	for i, sortState := range displaySorts {
		cursor := "  "
		if i == cursorIndex {
			cursor = "> "
		}

		indicator := "   "
		if sortState.IsEnabled() {
			indicator = fmt.Sprintf(" %d ", sortState.Priority+1)
		}

		shortKey := sortState.ShortKey()
		label := sortState.DisplayName()
		if !sortState.IsEnabled() {
			label = sortState.Mode.String()
		}

		var rowStyle lipgloss.Style
		if i == cursorIndex {
			rowStyle = styles.SelectedRowStyle
		} else {
			rowStyle = styles.TableRowStyle
		}

		checkStyle := lipgloss.NewStyle().Foreground(styles.Green)
		keyStyle := lipgloss.NewStyle().
			Foreground(styles.Mauve).
			Bold(true)

		formattedIndicator := fmt.Sprintf("%-4s", indicator)
		formattedKey := fmt.Sprintf("%-3s", shortKey)

		row := fmt.Sprintf("%s%s  %s  %s",
			cursor,
			checkStyle.Render(formattedIndicator),
			keyStyle.Render(formattedKey),
			rowStyle.Render(label),
		)
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpLines := []string{
		styles.FooterKeyStyle.Render("enter/key") + styles.FooterDescStyle.Render(" cycle (off/ASC/DESC)"),
		styles.FooterKeyStyle.Render("[/]") + styles.FooterDescStyle.Render(" reorder"),
		styles.FooterKeyStyle.Render("*") + styles.FooterDescStyle.Render(" reset"),
		styles.FooterKeyStyle.Render("esc") + styles.FooterDescStyle.Render(" close"),
	}
	b.WriteString(strings.Join(helpLines, "  "))

	content := b.String()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderBatchProgress() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render(m.batchTask))
	b.WriteString("\n\n")

	progressWidth := 40
	filled := 0
	if m.batchTotal > 0 {
		filled = (m.batchProgress * progressWidth) / m.batchTotal
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", progressWidth-filled)
	progressStr := fmt.Sprintf("[%s] %d/%d", bar, m.batchProgress, m.batchTotal)
	b.WriteString(progressStr)
	b.WriteString("\n\n")

	if len(m.batchResults) > 0 {
		b.WriteString(styles.HeaderStyle.Render("Results"))
		b.WriteString("\n")

		maxShow := 15
		startIdx := 0
		if len(m.batchResults) > maxShow {
			startIdx = len(m.batchResults) - maxShow
		}

		for i := startIdx; i < len(m.batchResults); i++ {
			result := m.batchResults[i]
			icon := styles.SuccessStyle.Render("✓")
			if !result.Success {
				icon = styles.ErrorStyle.Render("✗")
			}
			name := truncate(filepath.Base(result.Path), 25)
			msg := truncate(result.Message, 40)

			row := fmt.Sprintf("  %s %-25s  %s", icon, name, styles.SubtitleStyle.Render(msg))
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.batchRunning {
		b.WriteString(styles.SubtitleStyle.Render("Running... please wait"))
	} else {
		b.WriteString(styles.FooterStyle.Render("Press esc to close"))
	}

	return b.String()
}

func (m Model) renderBranchDetail() string {
	summary, _ := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n\n")

	sectionStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		Bold(true).
		PaddingLeft(1)

	infoStyle := lipgloss.NewStyle().
		Foreground(styles.Text).
		PaddingLeft(2)

	labelStyle := lipgloss.NewStyle().
		Foreground(styles.Subtext0).
		Width(18)

	// Branch Information Section
	b.WriteString(sectionStyle.Render("Branch Information"))
	b.WriteString("\n\n")

	if m.branchDetail.Branch.Upstream != "" {
		b.WriteString(infoStyle.Render(
			labelStyle.Render("Upstream:") + " " + m.branchDetail.Branch.Upstream,
		))
		b.WriteString("\n")
	}

	if m.branchDetail.Branch.Ahead > 0 || m.branchDetail.Branch.Behind > 0 {
		status := ""
		if m.branchDetail.Branch.Ahead > 0 {
			status += styles.AheadStyle.Render(fmt.Sprintf("↑%d ahead", m.branchDetail.Branch.Ahead))
		}
		if m.branchDetail.Branch.Behind > 0 {
			if status != "" {
				status += " "
			}
			status += styles.BehindStyle.Render(fmt.Sprintf("↓%d behind", m.branchDetail.Branch.Behind))
		}
		b.WriteString(infoStyle.Render(
			labelStyle.Render("Tracking:") + " " + status,
		))
		b.WriteString("\n")
	}

	defaultBranch := m.findDefaultBranch()
	if defaultBranch != "" && m.branchDetail.Branch.Name != defaultBranch {
		ahead, behind := m.compareToDefaultBranch(defaultBranch)
		if ahead >= 0 && behind >= 0 {
			status := ""
			if ahead > 0 {
				status += styles.AheadStyle.Render(fmt.Sprintf("↑%d ahead", ahead))
			}
			if behind > 0 {
				if status != "" {
					status += " "
				}
				status += styles.BehindStyle.Render(fmt.Sprintf("↓%d behind", behind))
			}
			if status == "" {
				status = styles.CleanStyle.Render("up to date")
			}
			b.WriteString(infoStyle.Render(
				labelStyle.Render("vs "+defaultBranch+":") + " " + status,
			))
			b.WriteString("\n")
		}
	}

	if len(m.branchDetail.Commits) > 0 {
		lastCommit := m.branchDetail.Commits[0]
		b.WriteString(infoStyle.Render(
			labelStyle.Render("Last commit:") + " " + lastCommit.RelativeDate(),
		))
		b.WriteString("\n")
		b.WriteString(infoStyle.Render(
			labelStyle.Render("Author:") + " " + lastCommit.Author,
		))
		b.WriteString("\n")
	}

	// File Changes
	fileChanges := m.branchDetail.FileChangesSummary()
	fileStyle := infoStyle
	if m.branchDetail.UncommittedCount() > 0 {
		fileStyle = lipgloss.NewStyle().
			Foreground(styles.Peach).
			PaddingLeft(2)
	}
	b.WriteString(fileStyle.Render(
		labelStyle.Render("File changes:") + " " + fileChanges,
	))
	b.WriteString("\n")

	// JJ-specific information
	if isJJ {
		if m.branchDetail.ChangeID != "" {
			b.WriteString(infoStyle.Render(
				labelStyle.Render("Change ID:") + " " + styles.SubtitleStyle.Render(m.branchDetail.ChangeID),
			))
			b.WriteString("\n")
		}
		if m.branchDetail.Description != "" {
			b.WriteString(infoStyle.Render(
				labelStyle.Render("Description:") + " " + truncate(m.branchDetail.Description, 60),
			))
			b.WriteString("\n")
		}
	}

	// PR & CI Section
	if m.branchDetail.PRInfo != nil || m.branchDetail.WorkflowInfo != nil {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("Pull Request & CI/CD"))
		b.WriteString("\n\n")

		if m.branchDetail.PRInfo != nil {
			pr := m.branchDetail.PRInfo
			prStatus := pr.StatusDisplay()
			prStyle := styles.PROpenStyle
			if prStatus == "MERGED" {
				prStyle = styles.CleanStyle
			} else if prStatus == "CLOSED" {
				prStyle = styles.SubtitleStyle
			}

			b.WriteString(infoStyle.Render(
				labelStyle.Render("PR:") + " " + prStyle.Render(fmt.Sprintf("#%d %s", pr.Number, prStatus)),
			))
			b.WriteString("\n")
			b.WriteString(infoStyle.Render(
				labelStyle.Render("Title:") + " " + truncate(pr.Title, 60),
			))
			b.WriteString("\n")

			// Review Status
			reviewStatus := pr.ReviewStatus()
			reviewStyle := styles.SubtitleStyle
			if reviewStatus == "approved" {
				reviewStyle = styles.CleanStyle
			} else if reviewStatus == "changes requested" {
				reviewStyle = styles.ErrorStyle
			}
			b.WriteString(infoStyle.Render(
				labelStyle.Render("Review:") + " " + reviewStyle.Render(reviewStatus),
			))
			b.WriteString("\n")

			if len(pr.ApprovedBy) > 0 {
				approvers := strings.Join(pr.ApprovedBy, ", ")
				b.WriteString(infoStyle.Render(
					labelStyle.Render("Approved by:") + " " + truncate(approvers, 60),
				))
				b.WriteString("\n")
			}

			// CI Checks
			if pr.Checks.Total > 0 {
				checkStatus := pr.Checks.Summary()
				checkStyle := styles.SubtitleStyle
				if checkStatus == "passing" {
					checkStyle = styles.CleanStyle
				} else if checkStatus == "failing" {
					checkStyle = styles.ErrorStyle
				}
				checkDetail := fmt.Sprintf("%s (%d/%d passing)", checkStatus, pr.Checks.Passing, pr.Checks.Total)
				b.WriteString(infoStyle.Render(
					labelStyle.Render("Checks:") + " " + checkStyle.Render(checkDetail),
				))
				b.WriteString("\n")
			}
		}

		// Workflow Status
		if m.branchDetail.WorkflowInfo != nil {
			wf := m.branchDetail.WorkflowInfo
			wfStatus := wf.StatusDisplay()
			wfStyle := styles.SubtitleStyle
			if wfStatus == "passing" {
				wfStyle = styles.CleanStyle
			} else if wfStatus == "failing" {
				wfStyle = styles.ErrorStyle
			}
			wfDetail := fmt.Sprintf("%s (%d/%d passing)", wfStatus, wf.Passing, wf.Total)
			b.WriteString(infoStyle.Render(
				labelStyle.Render("Workflows:") + " " + wfStyle.Render(wfDetail),
			))
			b.WriteString("\n")
		}
	}

	// Recent Commits Section
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Recent Commits"))
	b.WriteString("\n\n")

	if len(m.branchDetail.Commits) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(1, 2).
			Foreground(styles.Subtext0)
		b.WriteString(emptyStyle.Render("No commits found"))
	} else {
		maxCommits := 10
		if len(m.branchDetail.Commits) < maxCommits {
			maxCommits = len(m.branchDetail.Commits)
		}
		for i := 0; i < maxCommits; i++ {
			commit := m.branchDetail.Commits[i]
			hash := styles.SubtitleStyle.Render(commit.ShortHash)
			subject := truncate(commit.Subject, 50)
			author := truncate(commit.Author, 15)
			date := commit.RelativeDate()

			line := fmt.Sprintf("  %s  %-50s  %s  %s\n",
				hash,
				subject,
				styles.SubtitleStyle.Render(author),
				styles.SubtitleStyle.Render(date),
			)
			b.WriteString(line)
		}
	}

	// Actions Section
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Actions"))
	b.WriteString("\n\n")

	actionStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		PaddingLeft(2)

	actions := []string{
		styles.FooterKeyStyle.Render("y") + actionStyle.Render(" copy branch name"),
	}

	if m.branchDetail.PRInfo != nil {
		actions = append(actions,
			styles.FooterKeyStyle.Render("p") + actionStyle.Render(" open PR in browser"),
			styles.FooterKeyStyle.Render("o") + actionStyle.Render(" open PR URL"))
	} else {
		actions = append(actions,
			styles.FooterKeyStyle.Render("p") + actionStyle.Render(" create new PR"))
	}

	b.WriteString(strings.Join(actions, "  "))
	b.WriteString("\n")

	contentLines := strings.Count(b.String(), "\n")
	footerHeight := 1
	paddingNeeded := m.height - contentLines - footerHeight - 1
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(styles.FooterStyle.Render("esc: back  ?: help"))

	return b.String()
}

func (m Model) renderPRDetail() string {
	var b strings.Builder

	summary, _ := m.summaries[m.selectedRepo]
	home := styles.SubtitleStyle.Render("Repos")
	sep := styles.SubtitleStyle.Render(" > ")
	repo := styles.BranchStyle.Render(summary.Name())

	// Check if PR detail has been loaded
	if m.prDetail.Number == 0 {
		// Show loading state (shouldn't happen with progressive loading)
		b.WriteString(home + sep + repo + sep + styles.SubtitleStyle.Render("PR Detail"))
		b.WriteString("\n\n")
		loadingStyle := lipgloss.NewStyle().
			Foreground(styles.Blue).
			Padding(2)
		b.WriteString(loadingStyle.Render("Loading PR details..."))
		b.WriteString("\n\n")

		footer := styles.FooterKeyStyle.Render("esc") + styles.FooterDescStyle.Render(" back  ") +
			styles.FooterKeyStyle.Render("?") + styles.FooterDescStyle.Render(" help")
		b.WriteString(styles.FooterStyle.Render(footer))
		return b.String()
	}

	prTitle := styles.TitleStyle.Render(fmt.Sprintf("PR #%d", m.prDetail.Number))
	b.WriteString(home + sep + repo + sep + prTitle)
	b.WriteString("\n\n")

	// Show loading indicator for additional details if not yet loaded
	isFullyLoaded := m.prDetail.Author != ""
	if !isFullyLoaded {
		loadingIndicator := lipgloss.NewStyle().
			Foreground(styles.Subtext0).
			Italic(true).
			Render(" (loading details...)")
		b.WriteString(loadingIndicator)
		b.WriteString("\n")
	}

	sectionStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		Bold(true).
		PaddingLeft(1).
		PaddingTop(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(styles.Subtext0).
		Width(16)

	valueStyle := lipgloss.NewStyle().
		Foreground(styles.Text).
		PaddingLeft(2)

	b.WriteString(sectionStyle.Render("Pull Request"))
	b.WriteString("\n")

	b.WriteString(valueStyle.Render(labelStyle.Render("Title:") + " " + m.prDetail.Title))
	b.WriteString("\n")

	// Author might not be loaded yet (progressive loading)
	if m.prDetail.Author != "" {
		b.WriteString(valueStyle.Render(labelStyle.Render("Author:") + " " + m.prDetail.Author))
		b.WriteString("\n")
	}

	if len(m.prDetail.Assignees) > 0 {
		b.WriteString(valueStyle.Render(
			labelStyle.Render("Assignees:") + " " + strings.Join(m.prDetail.Assignees, ", "),
		))
		b.WriteString("\n")
	}

	if len(m.prDetail.Reviewers) > 0 {
		b.WriteString(valueStyle.Render(
			labelStyle.Render("Reviewers:") + " " + strings.Join(m.prDetail.Reviewers, ", "),
		))
		b.WriteString("\n")
	}

	b.WriteString(valueStyle.Render(
		labelStyle.Render("Branch:") + " " +
			styles.BranchStyle.Render(m.prDetail.HeadRef) + " → " +
			styles.BranchStyle.Render(m.prDetail.BaseRef),
	))
	b.WriteString("\n")

	stateStyle := styles.PROpenStyle
	if m.prDetail.IsDraft {
		stateStyle = styles.PRDraftStyle
	} else if m.prDetail.State == "MERGED" {
		stateStyle = styles.PRMergedStyle
	} else if m.prDetail.State == "CLOSED" {
		stateStyle = styles.ErrorStyle
	}

	b.WriteString(valueStyle.Render(
		labelStyle.Render("State:") + " " + stateStyle.Render(m.prDetail.StatusDisplay()),
	))
	b.WriteString("\n")

	reviewStyle := styles.SubtitleStyle
	reviewStatus := m.prDetail.ReviewStatus()
	if reviewStatus == "approved" {
		reviewStyle = styles.CleanStyle
	} else if reviewStatus == "changes requested" {
		reviewStyle = styles.ErrorStyle
	}

	b.WriteString(valueStyle.Render(
		labelStyle.Render("Review:") + " " + reviewStyle.Render(reviewStatus),
	))
	b.WriteString("\n")

	// Only show detailed stats if fully loaded
	if m.prDetail.Author != "" {
		b.WriteString(valueStyle.Render(
			labelStyle.Render("Changes:") + " " +
				styles.CleanStyle.Render(fmt.Sprintf("+%d", m.prDetail.Additions)) + " " +
				styles.ErrorStyle.Render(fmt.Sprintf("-%d", m.prDetail.Deletions)),
		))
		b.WriteString("\n")

		if m.prDetail.Comments > 0 {
			b.WriteString(valueStyle.Render(
				labelStyle.Render("Comments:") + " " + fmt.Sprintf("%d", m.prDetail.Comments),
			))
			b.WriteString("\n")
		}

		if !m.prDetail.CreatedAt.IsZero() {
			b.WriteString(valueStyle.Render(
				labelStyle.Render("Created:") + " " + m.prDetail.RelativeCreated(),
			))
			b.WriteString("\n")
		}

		if !m.prDetail.UpdatedAt.IsZero() {
			b.WriteString(valueStyle.Render(
				labelStyle.Render("Updated:") + " " + m.prDetail.RelativeUpdated(),
			))
			b.WriteString("\n")
		}
	}

	if m.prDetail.Body != "" {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("Description"))
		b.WriteString("\n")

		desc := m.prDetail.Body
		if len(desc) > 400 {
			desc = desc[:400] + "..."
		}
		b.WriteString(valueStyle.Render(desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Actions"))
	b.WriteString("\n")

	actionPadding := lipgloss.NewStyle().PaddingLeft(2)
	actions := []string{
		styles.FooterKeyStyle.Render("o") + styles.FooterDescStyle.Render(" open in browser"),
		styles.FooterKeyStyle.Render("u") + styles.FooterDescStyle.Render(" copy URL"),
		styles.FooterKeyStyle.Render("n") + styles.FooterDescStyle.Render(" copy PR number"),
		styles.FooterKeyStyle.Render("b") + styles.FooterDescStyle.Render(" copy branch name"),
	}
	b.WriteString(actionPadding.Render(strings.Join(actions, "    ")))
	b.WriteString("\n")

	contentLines := strings.Count(b.String(), "\n")
	statusLines := 0
	if m.statusMessage != "" {
		statusLines = 1
	}
	paddingNeeded := m.height - contentLines - statusLines - 2
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	}

	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(styles.Green).
			Background(styles.Surface0).
			Padding(0, 1)
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	footer := styles.FooterKeyStyle.Render("esc") + styles.FooterDescStyle.Render(" back  ") +
		styles.FooterKeyStyle.Render("?") + styles.FooterDescStyle.Render(" help")
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

func (m Model) findDefaultBranch() string {
	for _, branch := range m.branches {
		if branch.Name == "main" || branch.Name == "master" {
			return branch.Name
		}
	}
	return ""
}

func (m Model) compareToDefaultBranch(defaultBranch string) (int, int) {
	if defaultBranch == "" || m.branchDetail.Branch.Name == defaultBranch {
		return -1, -1
	}

	for _, branch := range m.branches {
		if branch.Name == defaultBranch {
			ahead := 0
			behind := 0

			for _, commit := range m.branchDetail.Commits {
				found := false
				for _, defCommit := range m.branchDetail.Commits {
					if commit.Hash == defCommit.Hash {
						found = true
						break
					}
				}
				if !found {
					ahead++
				}
			}

			return ahead, behind
		}
	}

	return -1, -1
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
