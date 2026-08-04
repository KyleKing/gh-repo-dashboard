package app

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
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
		if m.loading {
			return m.loadingPlaceholder("Discovering repositories")
		}
		if len(m.repoPaths) > 0 {
			return emptyPlaceholder("No repositories match the active filters",
				"Press f to change filters, / to clear the search.")
		}

		return emptyPlaceholder("No repositories found",
			"Nothing was discovered under the configured scan paths.")
	}

	layout := layoutRepoCols(contentWidth(m.width))
	header := styles.HeaderStyle.Render(renderRepoHeader(layout))

	previewLineCount := 0
	if m.notesPreviewOpen && m.cursor < len(m.filteredPaths) {
		previewLineCount = len(m.summaries[m.filteredPaths[m.cursor]].NotesFiles)
	}

	availableHeight := m.height - nonListRowHeight - previewLineCount
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
		row := m.renderTableRow(summary, i == m.cursor, layout)
		rows = append(rows, row)

		if i == m.cursor && m.notesPreviewOpen {
			if preview := renderNotesPreview(summary); preview != "" {
				rows = append(rows, preview)
			}
		}
	}

	return strings.Join(rows, "\n")
}

// renderNotesPreview renders one line per detected notes file, showing its
// name and first line. It reuses the first line already captured during
// detection, so opening the preview requires no extra file reads.
func renderNotesPreview(s models.RepoSummary) string {
	if len(s.NotesFiles) == 0 {
		return ""
	}

	lines := make([]string, len(s.NotesFiles))
	for i, nf := range s.NotesFiles {
		firstLine := nf.FirstLine
		if firstLine == "" {
			firstLine = "(empty)"
		}

		lines[i] = "     " + styles.NotesPreviewNameStyle.Render(nf.Name+":") + " " +
			styles.NotesPreviewLineStyle.Render(firstLine)
	}

	return strings.Join(lines, "\n")
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

// warnSuffix marks a template ref whose currency cannot be judged.
const warnSuffix = " ⚠"

// formatCopierCell formats a repo's copier-template column: emDash if the
// repo isn't copier-generated, the installed tag (with "→ latest" appended
// when behind), or the installed ref plus a warning icon when it isn't a
// semver tag at all (commit- or branch-pinned, so currency can't be judged).
func formatCopierCell(s models.RepoSummary, width int) string {
	info := s.TemplateInfo
	if info == nil {
		return emDash
	}

	if !info.IsTag {
		return truncate(info.Commit, width-lipgloss.Width(warnSuffix)) + warnSuffix
	}

	if info.Behind && info.LatestTag != "" {
		return info.Commit + " → " + info.LatestTag
	}

	return info.Commit
}

func notesMarker(s models.RepoSummary, base lipgloss.Style, selected bool) (string, lipgloss.Style) {
	count := len(s.NotesFiles)
	if count == 0 {
		return " ", base
	}

	style := withSelection(styles.NotesBadgeStyle, selected)
	if count == 1 {
		return "N", style
	}

	return "N" + strconv.Itoa(count), style
}

// renderRepoHeader renders the table header for a resolved column layout,
// including the marker that announces any columns collapse hid.
func renderRepoHeader(layout table.Layout) string {
	return strings.Repeat(" ", cursorWidth) + table.Header(layout)
}

// peersCell renders a repo's parallel-checkout count, or emDash when the repo
// is the only checkout of its remote.
func (m Model) peersCell(path string, base lipgloss.Style, selected bool) (string, lipgloss.Style) {
	peers := m.PeerCheckouts(path)
	if len(peers) == 0 {
		return emDash, base
	}

	return "⧉" + strconv.Itoa(len(peers)), withSelection(styles.CountStyle, selected)
}

func (m Model) renderTableRow(s models.RepoSummary, selected bool, layout table.Layout) string {
	base := styles.TableRowStyle
	if selected {
		base = styles.SelectedRowStyle
	}

	cells := make([]string, 0, len(layout.Columns)+1)
	for _, c := range layout.Columns {
		cells = append(cells, m.repoCell(c.Key, s, layout.Width(c.Key), base, selected))
	}

	if marker := layout.Marker(); marker != "" {
		cells = append(cells, base.Render(strings.Repeat(" ", lipgloss.Width(marker))))
	}

	gutter := base.Render(strings.Repeat(" ", table.Gutter))

	return base.Render(rowCursor(m.selectedPaths[s.Path], selected)) +
		strings.Join(cells, gutter)
}

// rowCursor renders the two-cell leader: the cursor arrow and the batch
// selection mark.
func rowCursor(marked, selected bool) string {
	cursorChar := " "
	if selected {
		cursorChar = ">"
	}
	markChar := " "
	if marked {
		markChar = "•"
	}

	return cursorChar + markChar
}

// repoCell renders one column of a repo row, padded to exactly width cells so
// the columns after it stay aligned.
func (m Model) repoCell(col string, s models.RepoSummary, width int, base lipgloss.Style, selected bool) string {
	switch col {
	case colName:
		return base.Render(padCell(s.Name(), width))
	case colBranch:
		return withSelection(styles.BranchStyle, selected).Render(padCell(s.Branch, width))
	case colStatus:
		return statusCell(s, width, base, selected)
	case colPeers:
		text, style := m.peersCell(s.Path, base, selected)

		return style.Render(padCell(text, width))
	case colPR:
		return prCellStyle(s, base, selected).Render(padCell(formatPRCell(s), width))
	case colPRs:
		return base.Render(padCell(m.prCountText(s.Path), width))
	case colTemplate:
		return templateCellStyle(s, base, selected).Render(padCell(formatCopierCell(s, width), width))
	case colModified:
		return base.Render(padCell(s.RelativeModified(), width))
	default:
		return base.Render(padCell("", width))
	}
}

// statusCell renders the status summary and the notes marker that shares its
// column, keeping the marker right-aligned within the column.
func statusCell(s models.RepoSummary, width int, base lipgloss.Style, selected bool) string {
	notesText, notesStyle := notesMarker(s, base, selected)
	style := withSelection(statusCellStyle(s, base), selected)

	return style.Render(padCell(s.StatusSummary(), width-notesMarkerWidth)) +
		notesStyle.Render(padCell(notesText, notesMarkerWidth))
}

func (m Model) prCountText(path string) string {
	if count, ok := m.prCount[path]; ok && count > 0 {
		return strconv.Itoa(count)
	}

	return emDash
}

func statusCellStyle(s models.RepoSummary, base lipgloss.Style) lipgloss.Style {
	switch {
	case s.IsDirty():
		return styles.DirtyStyle
	case s.Status() == models.RepoStatusClean:
		return styles.CleanStyle
	default:
		return base
	}
}

func prCellStyle(s models.RepoSummary, base lipgloss.Style, selected bool) lipgloss.Style {
	if s.PRInfo != nil {
		return withSelection(styles.PROpenStyle, selected)
	}

	return base
}

func templateCellStyle(s models.RepoSummary, base lipgloss.Style, selected bool) lipgloss.Style {
	if info := s.TemplateInfo; info != nil && (!info.IsTag || info.Behind) {
		return withSelection(styles.WarningStyle, selected)
	}

	return base
}

func (m Model) renderFooter() string {
	bindings := []struct {
		key  string
		desc string
	}{
		{"j/k", "nav"},
		{keyEnter, "select"},
		{"v", "notes"},
		{"f", nameFilter},
		{"s", nameSort},
		{"/", "search"},
		{":", "command"},
		{"r", nameRefresh},
		{"?", nameHelp},
		{"q", nameQuit},
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
