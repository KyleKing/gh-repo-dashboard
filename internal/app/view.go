package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// emDash is the placeholder rendered for empty/unknown values.
const emDash = "—"

// mainBranchName and featureBranchName are the conventional branch names used in test fixtures.
const (
	mainBranchName    = "main"
	featureBranchName = "feature"
)

// Layout constants for the repo list table, detail panes, and batch results view.
const (
	repoNameColWidth       = 20
	branchColWidth         = 15
	statusColWidth         = 12
	notesMarkerWidth       = 2
	prColWidth             = 12
	prsColWidth            = 6
	modifiedColWidth       = 12
	nonListRowHeight       = 6
	visibleWindowCenter    = 2
	branchNameTruncLen     = 20
	upstreamTruncLen       = 20
	messageTruncLen        = 40
	worktreePathTruncLen   = 30
	batchNameTruncLen      = 25
	descriptionTruncLen    = 60
	commitSubjectLen       = 50
	commitAuthorLen        = 15
	detailLabelWidth       = 18
	detailLabelWidthPR     = 16
	prBodyMaxLen           = 400
	statusBarHeight        = 2
	emptyStateVPad         = 2
	emptyStateHPad         = 4
	infoPaddingLeft        = 2
	commitEmptyStateVPad   = 1
	loadingStatePad        = 2
	branchDetailMaxCommits = 10
)

// View renders the TUI for the current model state.
func (m Model) View() tea.View {
	v := tea.NewView(m.renderScreen())
	v.AltScreen = true

	return v
}

func (m Model) renderScreen() string {
	if m.width == 0 {
		return "Loading..."
	}

	content := m.renderView()
	if m.commandMode {
		return overlayBottomLine(content, m.commandInput.View(), m.height)
	}

	return content
}

// overlayBottomLine pins line onto the last row of content, padding or
// truncating content to keep the overall height stable.
func overlayBottomLine(content, line string, height int) string {
	lines := strings.Split(content, "\n")
	if height < 1 {
		return content
	}
	switch {
	case len(lines) >= height:
		lines = lines[:height-1]
	default:
		for len(lines) < height-1 {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n") + "\n" + line
}

func (m Model) renderView() string {
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

// centerModal centers content on screen as a single block. Content is first
// left-padded to a uniform width because lipgloss.Place centers each line of
// a multi-line string independently based on that line's own width, which
// would otherwise stagger rows of differing length (e.g. table rows).
func centerModal(m Model, content string) string {
	width := lipgloss.Width(content)
	content = lipgloss.NewStyle().Width(width).Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
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
		return m.renderRepoDetailBreadcrumbs()
	case ViewModeBranchDetail:
		return m.renderBranchDetailBreadcrumbs()
	default:
		return m.renderRepoListBreadcrumbs()
	}
}

func (m Model) renderRepoDetailBreadcrumbs() string {
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
}

func (m Model) renderBranchDetailBreadcrumbs() string {
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
}

func (m Model) renderRepoListBreadcrumbs() string {
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

func (m Model) renderStatusBar() string {
	parts := []string{}
	parts = appendFilterBadges(parts, m.activeFilters)
	parts = appendSortBadges(parts, m.activeSorts)

	if m.predicateText != "" {
		parts = append(parts, styles.Badge(m.predicateText, styles.FilterBadgeStyle))
	}

	if m.searchText != "" {
		parts = append(parts, styles.Badge("\""+m.searchText+"\"", styles.SearchBadgeStyle))
	}

	if count := m.SelectedCount(); count > 0 {
		parts = append(parts, styles.Badge(fmt.Sprintf("%d selected", count), styles.SortBadgeStyle))
	}

	return strings.Join(parts, " ")
}

// appendFilterBadges appends a badge for each enabled, non-"all" filter.
func appendFilterBadges(parts []string, activeFilters []models.ActiveFilter) []string {
	for _, f := range activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			label := f.Mode.String()
			if f.Inverted {
				label = "NOT " + label
			}
			parts = append(parts, styles.Badge(label, styles.FilterBadgeStyle))
		}
	}

	return parts
}

// appendSortBadges appends a badge for each enabled sort, in priority order.
func appendSortBadges(parts []string, activeSorts []models.ActiveSort) []string {
	enabledCount := 0
	for _, s := range activeSorts {
		if s.IsEnabled() {
			enabledCount++
		}
	}

	for priority := range enabledCount {
		for _, s := range activeSorts {
			if s.IsEnabled() && s.Priority == priority {
				parts = append(parts, styles.Badge(s.DisplayName(), styles.SortBadgeStyle))
				break
			}
		}
	}

	return parts
}

func (m Model) renderTable() string {
	if len(m.filteredPaths) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
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
		name:     repoNameColWidth,
		branch:   branchColWidth,
		status:   statusColWidth,
		pr:       prColWidth,
		prs:      prsColWidth,
		modified: modifiedColWidth,
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

	availableHeight := m.height - nonListRowHeight
	if m.searching {
		availableHeight--
	}

	startIdx := m.cursor - availableHeight/visibleWindowCenter
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

// formatPRCell formats a repo's PR-column text: "#N" with a review-status
// indicator and a CI/workflow failure indicator, or emDash if there's no PR.
func formatPRCell(s models.RepoSummary) string {
	if s.PRInfo == nil {
		return emDash
	}

	prNum := fmt.Sprintf("#%d", s.PRInfo.Number)

	switch s.PRInfo.ReviewStatus() {
	case models.ReviewApproved:
		prNum += " ✓"
	case models.ReviewChangesRequested:
		prNum += " ✗"
	}

	switch {
	case s.PRInfo.Checks.Total > 0:
		if s.PRInfo.Checks.Summary() == models.StatusFailing {
			prNum += " ⚠"
		}
	case s.WorkflowInfo != nil:
		if s.WorkflowInfo.StatusDisplay() == models.StatusFailing {
			prNum += " ⚠"
		}
	}

	return prNum
}

func notesMarker(s models.RepoSummary, base lipgloss.Style, selected bool) (string, lipgloss.Style) {
	if s.NotesFile == "" {
		return " ", base
	}

	style := styles.NotesBadgeStyle
	if selected {
		style = style.Background(styles.Surface0)
	}

	return "N", style
}

func (m Model) renderTableRow(s models.RepoSummary, selected bool, colWidths struct {
	name     int
	branch   int
	status   int
	pr       int
	prs      int
	modified int
},
) string {
	cursorChar := " "
	if selected {
		cursorChar = ">"
	}
	markChar := " "
	if m.selectedPaths[s.Path] {
		markChar = "•"
	}
	cursor := cursorChar + markChar

	name := truncate(s.Name(), colWidths.name)
	branch := truncate(s.Branch, colWidths.branch)
	status := s.StatusSummary()
	pr := formatPRCell(s)

	prCountStr := emDash
	if count, ok := m.prCount[s.Path]; ok && count > 0 {
		prCountStr = strconv.Itoa(count)
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

	notesText, notesStyle := notesMarker(s, style, selected)

	statusTextWidth := colWidths.status - notesMarkerWidth

	formattedName := fmt.Sprintf("%-*s", colWidths.name, name)
	formattedBranch := fmt.Sprintf("%-*s", colWidths.branch, branch)
	formattedStatus := fmt.Sprintf("%-*s", statusTextWidth, status)
	formattedNotes := fmt.Sprintf("%-*s", notesMarkerWidth, notesText)
	formattedPR := fmt.Sprintf("%-*s", colWidths.pr, pr)
	formattedPRCount := fmt.Sprintf("%-*s", colWidths.prs, prCountStr)

	statusCell := statusStyle.Render(formattedStatus) + notesStyle.Render(formattedNotes)

	row := fmt.Sprintf("%s%s  %s  %s  %s  %s  %s",
		cursor,
		nameStyle.Render(formattedName),
		branchStyle.Render(formattedBranch),
		statusCell,
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
		{keyEnter, "select"},
		{"f", "filter"},
		{"s", "sort"},
		{"/", "search"},
		{"r", "refresh"},
		{"?", "help"},
		{"q", "quit"},
	}

	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		parts = append(parts,
			styles.FooterKeyStyle.Render(b.key)+
				styles.FooterDescStyle.Render(" "+b.desc))
	}

	footer := strings.Join(parts, "  ")

	if m.pendingOperator != "" {
		hint := m.pendingOperator + m.pendingObject
		pendingHint := " pending (ar/br/dr/pr/sr, " + m.pendingOperator + m.pendingOperator + "=all, esc cancels)"
		pending := styles.FooterKeyStyle.Render(hint) + styles.FooterDescStyle.Render(pendingHint)
		footer = pending + "  " + footer
	}

	return footer
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
	case DetailTabNotes:
		b.WriteString(m.renderNotesTab())
	}

	footer := "tab: switch tabs  j/k: navigate  esc: back"
	switch m.detailTab {
	case DetailTabBranches:
		footer = "tab: switch tabs  j/k: navigate  enter: view branch  esc: back"
	case DetailTabPRs:
		footer = "tab: switch tabs  j/k: navigate  enter: view PR  esc: back"
	default:
		// stashes/worktrees tabs use the generic footer above
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
	summary := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	worktreeLabel := "Worktrees"
	if isJJ {
		worktreeLabel = "Workspaces"
	}

	notesCount := 0
	if m.notesFile != "" {
		notesCount = 1
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
		{"Notes", DetailTabNotes, notesCount},
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
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		return emptyStyle.Render("No branches found")
	}

	var rows []string
	header := fmt.Sprintf("  %-20s  %-20s  %-10s  %s",
		"BRANCH", "UPSTREAM", "STATUS", "LAST COMMIT")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, branch := range m.branches {
		rows = append(rows, renderBranchRow(branch, i == m.detailCursor))
	}

	return strings.Join(rows, "\n")
}

// branchAheadBehindStatus renders a branch's ahead/behind indicator, or a
// checkmark if it's fully in sync with its upstream.
func branchAheadBehindStatus(branch models.BranchInfo) string {
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

	return status
}

func renderBranchRow(branch models.BranchInfo, isSelected bool) string {
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	name := truncate(branch.Name, branchNameTruncLen)
	if branch.IsCurrent {
		name = "* " + name
	}
	upstream := truncate(branch.Upstream, upstreamTruncLen)
	status := branchAheadBehindStatus(branch)
	lastCommit := branch.RelativeLastCommit()

	style := styles.TableRowStyle
	if isSelected {
		style = styles.SelectedRowStyle
	}

	nameStyle := styles.BranchStyle
	if branch.IsCurrent {
		nameStyle = styles.PROpenStyle
	}
	if isSelected {
		nameStyle = nameStyle.Background(styles.Surface0)
	}

	formattedName := fmt.Sprintf("%-20s", name)
	formattedUpstream := fmt.Sprintf("%-20s", upstream)
	formattedStatus := fmt.Sprintf("%-10s", status)

	return fmt.Sprintf("%s%s  %s  %s  %s",
		cursor,
		nameStyle.Render(formattedName),
		style.Render(formattedUpstream),
		style.Render(formattedStatus),
		style.Render(lastCommit),
	)
}

func (m Model) renderStashList() string {
	if len(m.stashes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		noStashesMsg := "No stashes found\n\nStashes are only available for git repositories.\n" +
			"JJ repositories use the working copy change instead."

		return emptyStyle.Render(noStashesMsg)
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
		message := truncate(stash.Message, messageTruncLen)
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

// renderNotesTab shows the full content of the repo's detected notes file, or
// an empty state naming the filenames that are detected.
func (m Model) renderNotesTab() string {
	if m.notesFile == "" {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		noNotesMsg := "No notes file found\n\n" +
			"Add a .doing, doing.md, doing.txt, or TODO.md file to the repo root."

		return emptyStyle.Render(noNotesMsg)
	}

	var b strings.Builder
	b.WriteString(styles.HeaderStyle.Render(m.notesFile))
	b.WriteString("\n\n")

	content := m.notesContent
	if content == "" {
		content = "(empty file)"
	}
	b.WriteString(styles.TableRowStyle.Render(content))

	return b.String()
}

func (m Model) renderWorktreeList() string {
	summary := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	if len(m.worktrees) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		emptyMsg := "No worktrees found\n\nWorktrees allow working on multiple branches simultaneously."
		if isJJ {
			emptyMsg = "No workspaces found\n\nWorkspaces (jj's version of worktrees) allow working on multiple\n" +
				"changes simultaneously in separate working directories."
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

		path := truncate(filepath.Base(wt.Path), worktreePathTruncLen)
		branch := truncate(wt.Branch, branchNameTruncLen)
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
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		return emptyStyle.Render("No open pull requests")
	}

	var rows []string
	header := fmt.Sprintf("  %-8s  %-40s  %-10s  %-18s  %s",
		"NUMBER", "TITLE", "STATE", "REVIEW", "BRANCH")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i := range m.prs {
		pr := &m.prs[i]
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		number := fmt.Sprintf("#%d", pr.Number)
		title := truncate(pr.Title, messageTruncLen)
		state := pr.StatusDisplay()
		review := pr.ReviewStatus()
		branch := truncate(pr.HeadRef, branchNameTruncLen)

		var rowStyle lipgloss.Style
		if i == m.detailCursor {
			rowStyle = styles.SelectedRowStyle
		} else {
			rowStyle = styles.TableRowStyle
		}

		stateStyle := styles.PROpenStyle
		switch {
		case pr.IsDraft:
			stateStyle = styles.PRDraftStyle
		case state == models.PRStatusMerged:
			stateStyle = styles.PRMergedStyle
		case state == models.PRStatusClosed:
			stateStyle = styles.ErrorStyle
		}
		if i == m.detailCursor {
			stateStyle = stateStyle.Background(styles.Surface0)
		}

		reviewStyle := styles.SubtitleStyle
		switch review {
		case models.ReviewApproved:
			reviewStyle = styles.CleanStyle
		case models.ReviewChangesRequested:
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
		formattedCount := strconv.Itoa(count)

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
		styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" close"),
	}
	b.WriteString(strings.Join(helpLines, "  "))

	content := b.String()

	return centerModal(m, content)
}

func (m Model) countForFilter(mode models.FilterMode) int {
	return len(filters.FilterRepos(m.repoPaths, m.summaries, mode))
}

// buildSortModalRows orders activeSorts for display: enabled sorts first (with
// their priority gaps compacted), then disabled sorts.
// CompactSortPriorities closes any gaps in sortsByPriority's Priority values
// (e.g. after a sort was disabled) so priorities are a contiguous 0..n-1
// sequence, in place.
func compactSortPriorities(sortsByPriority []models.ActiveSort) {
	for i := range sortsByPriority {
		hasPriority := slices.ContainsFunc(sortsByPriority, func(s models.ActiveSort) bool {
			return s.Priority == i
		})
		if hasPriority {
			continue
		}

		for k := range sortsByPriority {
			if sortsByPriority[k].Priority > i {
				sortsByPriority[k].Priority--
			}
		}
	}
}

func buildSortModalRows(activeSorts []models.ActiveSort) []models.ActiveSort {
	sortsByPriority := make([]models.ActiveSort, 0)
	for _, s := range activeSorts {
		if s.IsEnabled() {
			sortsByPriority = append(sortsByPriority, s)
		}
	}

	compactSortPriorities(sortsByPriority)

	inactiveSorts := make([]models.ActiveSort, 0)
	for _, s := range activeSorts {
		if !s.IsEnabled() {
			inactiveSorts = append(inactiveSorts, s)
		}
	}

	displaySorts := make([]models.ActiveSort, 0, len(sortsByPriority)+len(inactiveSorts))
	displaySorts = append(displaySorts, sortsByPriority...)
	displaySorts = append(displaySorts, inactiveSorts...)

	return displaySorts
}

func renderSortModalRow(sortState models.ActiveSort, isSelected bool) string {
	cursor := "  "
	if isSelected {
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

	rowStyle := styles.TableRowStyle
	if isSelected {
		rowStyle = styles.SelectedRowStyle
	}

	checkStyle := lipgloss.NewStyle().Foreground(styles.Green)
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Mauve).
		Bold(true)

	formattedIndicator := fmt.Sprintf("%-4s", indicator)
	formattedKey := fmt.Sprintf("%-3s", shortKey)

	return fmt.Sprintf("%s%s  %s  %s",
		cursor,
		checkStyle.Render(formattedIndicator),
		keyStyle.Render(formattedKey),
		rowStyle.Render(label),
	)
}

func (m Model) renderSortModal() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Sort Repositories"))
	b.WriteString("\n\n")

	displaySorts := buildSortModalRows(m.activeSorts)

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
		b.WriteString(renderSortModalRow(sortState, i == cursorIndex))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpLines := []string{
		styles.FooterKeyStyle.Render("enter/key") + styles.FooterDescStyle.Render(" cycle (off/ASC/DESC)"),
		styles.FooterKeyStyle.Render("[/]") + styles.FooterDescStyle.Render(" reorder"),
		styles.FooterKeyStyle.Render("*") + styles.FooterDescStyle.Render(" reset"),
		styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" close"),
	}
	b.WriteString(strings.Join(helpLines, "  "))

	content := b.String()

	return centerModal(m, content)
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
			name := truncate(filepath.Base(result.Path), batchNameTruncLen)
			msg := truncate(result.Message, messageTruncLen)

			row := fmt.Sprintf("  %s %-*s  %s", icon, batchNameTruncLen, name, styles.SubtitleStyle.Render(msg))
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

// branchDetailStyles are the shared styles used across renderBranchDetail's sections.
type branchDetailStyles struct {
	section lipgloss.Style
	info    lipgloss.Style
	label   lipgloss.Style
}

func newBranchDetailStyles() branchDetailStyles {
	return branchDetailStyles{
		section: lipgloss.NewStyle().Foreground(styles.Blue).Bold(true).PaddingLeft(1),
		info:    lipgloss.NewStyle().Foreground(styles.Text).PaddingLeft(infoPaddingLeft),
		label:   lipgloss.NewStyle().Foreground(styles.Subtext0).Width(detailLabelWidth),
	}
}

// writeInfoLine renders "label: value" through the shared info style and
// appends it (plus a trailing newline) to b.
func (s branchDetailStyles) writeInfoLine(b *strings.Builder, label, value string) {
	b.WriteString(s.info.Render(s.label.Render(label) + " " + value))
	b.WriteString("\n")
}

// aheadBehindStatus renders an ahead/behind pair through AheadStyle/BehindStyle,
// or empty if both are zero.
func aheadBehindStatus(ahead, behind int) string {
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

	return status
}

func (m Model) writeBranchInfoSection(b *strings.Builder, s branchDetailStyles) {
	b.WriteString(s.section.Render("Branch Information"))
	b.WriteString("\n\n")

	branch := m.branchDetail.Branch
	if branch.Upstream != "" {
		s.writeInfoLine(b, "Upstream:", branch.Upstream)
	}

	if branch.Ahead > 0 || branch.Behind > 0 {
		s.writeInfoLine(b, "Tracking:", aheadBehindStatus(branch.Ahead, branch.Behind))
	}

	m.writeDefaultBranchComparison(b, s)

	if len(m.branchDetail.Commits) > 0 {
		lastCommit := m.branchDetail.Commits[0]
		s.writeInfoLine(b, "Last commit:", lastCommit.RelativeDate())
		s.writeInfoLine(b, "Author:", lastCommit.Author)
	}

	fileChanges := m.branchDetail.FileChangesSummary()
	fileStyle := s.info
	if m.branchDetail.UncommittedCount() > 0 {
		fileStyle = lipgloss.NewStyle().Foreground(styles.Peach).PaddingLeft(infoPaddingLeft)
	}
	b.WriteString(fileStyle.Render(s.label.Render("File changes:") + " " + fileChanges))
	b.WriteString("\n")

	if summary := m.summaries[m.selectedRepo]; summary.VCSType == models.VCSTypeJJ {
		if m.branchDetail.ChangeID != "" {
			s.writeInfoLine(b, "Change ID:", styles.SubtitleStyle.Render(m.branchDetail.ChangeID))
		}
		if m.branchDetail.Description != "" {
			s.writeInfoLine(b, "Description:", truncate(m.branchDetail.Description, descriptionTruncLen))
		}
	}
}

func (m Model) writeDefaultBranchComparison(b *strings.Builder, s branchDetailStyles) {
	detail := m.branchDetail
	if detail.DefaultBranch == "" || detail.Branch.Name == detail.DefaultBranch {
		return
	}

	status := aheadBehindStatus(detail.DefaultAhead, detail.DefaultBehind)
	if status == "" {
		status = styles.CleanStyle.Render("up to date")
	}
	s.writeInfoLine(b, "vs "+detail.DefaultBranch+":", status)
}

func (m Model) writeBranchPRSection(b *strings.Builder, s branchDetailStyles) {
	if m.branchDetail.PRInfo == nil && m.branchDetail.WorkflowInfo == nil {
		return
	}

	b.WriteString("\n")
	b.WriteString(s.section.Render("Pull Request & CI/CD"))
	b.WriteString("\n\n")

	if pr := m.branchDetail.PRInfo; pr != nil {
		writeBranchPRInfo(b, s, pr)
	}

	if wf := m.branchDetail.WorkflowInfo; wf != nil {
		wfStatus := wf.StatusDisplay()
		wfStyle := styles.SubtitleStyle
		switch wfStatus {
		case "passing":
			wfStyle = styles.CleanStyle
		case models.StatusFailing:
			wfStyle = styles.ErrorStyle
		}
		wfDetail := fmt.Sprintf("%s (%d/%d passing)", wfStatus, wf.Passing, wf.Total)
		s.writeInfoLine(b, "Workflows:", wfStyle.Render(wfDetail))
	}
}

func writeBranchPRInfo(b *strings.Builder, s branchDetailStyles, pr *models.PRInfo) {
	prStatus := pr.StatusDisplay()
	prStyle := styles.PROpenStyle
	switch prStatus {
	case models.PRStatusMerged:
		prStyle = styles.CleanStyle
	case models.PRStatusClosed:
		prStyle = styles.SubtitleStyle
	}

	s.writeInfoLine(b, "PR:", prStyle.Render(fmt.Sprintf("#%d %s", pr.Number, prStatus)))
	s.writeInfoLine(b, "Title:", truncate(pr.Title, descriptionTruncLen))

	reviewStatus := pr.ReviewStatus()
	reviewStyle := styles.SubtitleStyle
	switch reviewStatus {
	case models.ReviewApproved:
		reviewStyle = styles.CleanStyle
	case models.ReviewChangesRequested:
		reviewStyle = styles.ErrorStyle
	}
	s.writeInfoLine(b, "Review:", reviewStyle.Render(reviewStatus))

	if len(pr.ApprovedBy) > 0 {
		approvers := strings.Join(pr.ApprovedBy, ", ")
		s.writeInfoLine(b, "Approved by:", truncate(approvers, descriptionTruncLen))
	}

	if pr.Checks.Total > 0 {
		checkStatus := pr.Checks.Summary()
		checkStyle := styles.SubtitleStyle
		switch checkStatus {
		case "passing":
			checkStyle = styles.CleanStyle
		case models.StatusFailing:
			checkStyle = styles.ErrorStyle
		}
		checkDetail := fmt.Sprintf("%s (%d/%d passing)", checkStatus, pr.Checks.Passing, pr.Checks.Total)
		s.writeInfoLine(b, "Checks:", checkStyle.Render(checkDetail))
	}
}

func (m Model) writeBranchCommitsSection(b *strings.Builder, s branchDetailStyles) {
	b.WriteString("\n")
	b.WriteString(s.section.Render("Recent Commits"))
	b.WriteString("\n\n")

	if len(m.branchDetail.Commits) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(commitEmptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)
		b.WriteString(emptyStyle.Render("No commits found"))

		return
	}

	maxCommits := min(branchDetailMaxCommits, len(m.branchDetail.Commits))
	for i := range maxCommits {
		commit := m.branchDetail.Commits[i]
		line := fmt.Sprintf("  %s  %-50s  %s  %s\n",
			styles.SubtitleStyle.Render(commit.ShortHash),
			truncate(commit.Subject, commitSubjectLen),
			styles.SubtitleStyle.Render(truncate(commit.Author, commitAuthorLen)),
			styles.SubtitleStyle.Render(commit.RelativeDate()),
		)
		b.WriteString(line)
	}
}

func (m Model) writeBranchActionsSection(b *strings.Builder, s branchDetailStyles) {
	b.WriteString("\n")
	b.WriteString(s.section.Render("Actions"))
	b.WriteString("\n\n")

	actionStyle := lipgloss.NewStyle().Foreground(styles.Blue).PaddingLeft(infoPaddingLeft)

	actions := []string{
		styles.FooterKeyStyle.Render("y") + actionStyle.Render(" copy branch name"),
	}

	if m.branchDetail.PRInfo != nil {
		actions = append(actions,
			styles.FooterKeyStyle.Render("p")+actionStyle.Render(" open PR in browser"),
			styles.FooterKeyStyle.Render("o")+actionStyle.Render(" open PR URL"))
	} else {
		actions = append(actions,
			styles.FooterKeyStyle.Render("p")+actionStyle.Render(" create new PR"))
	}

	b.WriteString(strings.Join(actions, "  "))
	b.WriteString("\n")
}

func (m Model) renderBranchDetail() string {
	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n\n")

	s := newBranchDetailStyles()
	m.writeBranchInfoSection(&b, s)
	m.writeBranchPRSection(&b, s)
	m.writeBranchCommitsSection(&b, s)
	m.writeBranchActionsSection(&b, s)

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

// writePRDetailInfo writes the "Pull Request" section's info lines (title,
// author, assignees, reviewers, branch, state, review, and - once fully
// loaded - change stats) via the given writeLine(label, value) callback.
func (m Model) writePRDetailInfo(writeLine func(label, value string)) {
	writeLine("Title:", m.prDetail.Title)

	// Author might not be loaded yet (progressive loading)
	if m.prDetail.Author != "" {
		writeLine("Author:", m.prDetail.Author)
	}

	if len(m.prDetail.Assignees) > 0 {
		writeLine("Assignees:", strings.Join(m.prDetail.Assignees, ", "))
	}

	if len(m.prDetail.Reviewers) > 0 {
		writeLine("Reviewers:", strings.Join(m.prDetail.Reviewers, ", "))
	}

	writeLine("Branch:",
		styles.BranchStyle.Render(m.prDetail.HeadRef)+" → "+styles.BranchStyle.Render(m.prDetail.BaseRef))

	stateStyle := styles.PROpenStyle
	switch {
	case m.prDetail.IsDraft:
		stateStyle = styles.PRDraftStyle
	case m.prDetail.State == models.PRStatusMerged:
		stateStyle = styles.PRMergedStyle
	case m.prDetail.State == models.PRStatusClosed:
		stateStyle = styles.ErrorStyle
	}
	writeLine("State:", stateStyle.Render(m.prDetail.StatusDisplay()))

	reviewStyle := styles.SubtitleStyle
	reviewStatus := m.prDetail.ReviewStatus()
	switch reviewStatus {
	case models.ReviewApproved:
		reviewStyle = styles.CleanStyle
	case models.ReviewChangesRequested:
		reviewStyle = styles.ErrorStyle
	}
	writeLine("Review:", reviewStyle.Render(reviewStatus))

	// Only show detailed stats if fully loaded
	if m.prDetail.Author == "" {
		return
	}

	writeLine("Changes:",
		styles.CleanStyle.Render(fmt.Sprintf("+%d", m.prDetail.Additions))+" "+
			styles.ErrorStyle.Render(fmt.Sprintf("-%d", m.prDetail.Deletions)))

	if m.prDetail.Comments > 0 {
		writeLine("Comments:", strconv.Itoa(m.prDetail.Comments))
	}

	if !m.prDetail.CreatedAt.IsZero() {
		writeLine("Created:", m.prDetail.RelativeCreated())
	}

	if !m.prDetail.UpdatedAt.IsZero() {
		writeLine("Updated:", m.prDetail.RelativeUpdated())
	}
}

// writePRDetailDescription writes the "Description" section (truncated to
// prBodyMaxLen), or nothing if the PR has no body.
func writePRDetailDescription(b *strings.Builder, sectionStyle, valueStyle lipgloss.Style, body string) {
	if body == "" {
		return
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Description"))
	b.WriteString("\n")

	desc := body
	if len(desc) > prBodyMaxLen {
		desc = desc[:prBodyMaxLen] + "..."
	}
	b.WriteString(valueStyle.Render(desc))
	b.WriteString("\n")
}

// writePRDetailActions writes the "Actions" section's footer key hints.
func writePRDetailActions(b *strings.Builder, sectionStyle lipgloss.Style) {
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Actions"))
	b.WriteString("\n")

	actionPadding := lipgloss.NewStyle().PaddingLeft(infoPaddingLeft)
	actions := []string{
		styles.FooterKeyStyle.Render("o") + styles.FooterDescStyle.Render(" open in browser"),
		styles.FooterKeyStyle.Render("u") + styles.FooterDescStyle.Render(" copy URL"),
		styles.FooterKeyStyle.Render("n") + styles.FooterDescStyle.Render(" copy PR number"),
		styles.FooterKeyStyle.Render("b") + styles.FooterDescStyle.Render(" copy branch name"),
	}
	b.WriteString(actionPadding.Render(strings.Join(actions, "    ")))
	b.WriteString("\n")
}

// renderPRDetailLoading renders the placeholder shown before any PR detail
// has arrived (shouldn't normally be seen, since PR info loads progressively).
func renderPRDetailLoading(home, sep, repo string) string {
	var b strings.Builder

	b.WriteString(home + sep + repo + sep + styles.SubtitleStyle.Render("PR Detail"))
	b.WriteString("\n\n")
	loadingStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		Padding(loadingStatePad)
	b.WriteString(loadingStyle.Render("Loading PR details..."))
	b.WriteString("\n\n")

	footer := styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" back  ") +
		styles.FooterKeyStyle.Render("?") + styles.FooterDescStyle.Render(" help")
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

func (m Model) renderPRDetail() string {
	var b strings.Builder

	summary := m.summaries[m.selectedRepo]
	home := styles.SubtitleStyle.Render("Repos")
	sep := styles.SubtitleStyle.Render(" > ")
	repo := styles.BranchStyle.Render(summary.Name())

	// Check if PR detail has been loaded
	if m.prDetail.Number == 0 {
		return renderPRDetailLoading(home, sep, repo)
	}

	prTitle := styles.TitleStyle.Render(fmt.Sprintf("PR #%d", m.prDetail.Number))
	b.WriteString(home + sep + repo + sep + prTitle)
	b.WriteString("\n\n")

	// Show loading indicator for additional details if not yet loaded
	if m.prDetail.Author == "" {
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
		Width(detailLabelWidthPR)

	valueStyle := lipgloss.NewStyle().
		Foreground(styles.Text).
		PaddingLeft(infoPaddingLeft)

	writeLine := func(label, value string) {
		b.WriteString(valueStyle.Render(labelStyle.Render(label) + " " + value))
		b.WriteString("\n")
	}

	b.WriteString(sectionStyle.Render("Pull Request"))
	b.WriteString("\n")

	m.writePRDetailInfo(writeLine)

	writePRDetailDescription(&b, sectionStyle, valueStyle, m.prDetail.Body)
	writePRDetailActions(&b, sectionStyle)

	contentLines := strings.Count(b.String(), "\n")
	statusLines := 0
	if m.statusMessage != "" {
		statusLines = 1
	}
	paddingNeeded := m.height - contentLines - statusLines - statusBarHeight
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

	footer := styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" back  ") +
		styles.FooterKeyStyle.Render("?") + styles.FooterDescStyle.Render(" help")
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

func truncate(s string, maxLen int) string {
	const ellipsis = "..."

	if len(s) <= maxLen {
		return s
	}
	if maxLen <= len(ellipsis) {
		return s[:maxLen]
	}

	return s[:maxLen-len(ellipsis)] + ellipsis
}
