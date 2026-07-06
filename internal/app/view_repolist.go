package app

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

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

	style := withSelection(styles.NotesBadgeStyle, selected)

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
	branchStyle := withSelection(styles.BranchStyle, selected)

	var statusStyle lipgloss.Style
	switch {
	case s.IsDirty():
		statusStyle = styles.DirtyStyle
	case s.Status() == models.RepoStatusClean:
		statusStyle = styles.CleanStyle
	default:
		statusStyle = style
	}
	statusStyle = withSelection(statusStyle, selected)

	prStyle := style
	if s.PRInfo != nil {
		prStyle = withSelection(styles.PROpenStyle, selected)
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
