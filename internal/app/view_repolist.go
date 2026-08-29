package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui"
	"github.com/kyleking/aragonite/tui/region"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/aragonite/tui/table"
)

// Vertical budget of the repo list. The body is sized once and rendered to
// exactly that many lines, so nothing the body does (the region opening, a
// shorter repo set) can move the footer.
const (
	listChromeHeight   = 8
	searchChromeHeight = 3
)

// searchScopeLegend spells out every scope prefix "/" search accepts, short
// key and long name together, so the grammar is visible the moment the
// search box opens rather than living only behind "?" help.
func searchScopeLegend() string {
	return styles.SubtitleStyle.Render(
		"[r]epo:  [b]ranch:  [p]r:  [t]emplate:  [n]otes:  [c]ommit:")
}

func (m Model) renderRepoList() string {
	if m.panelActions {
		return m.renderActionModal()
	}

	var b strings.Builder

	b.WriteString(m.renderTabBar(listWidth(m.width)))
	b.WriteString("\n")
	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n\n")
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n\n")

	switch {
	case m.searching:
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
		b.WriteString(searchScopeLegend())
		b.WriteString("\n\n")
	case m.viewMode == ViewModeFilter:
		b.WriteString(m.renderFilterDock())
		b.WriteString("\n\n")
	case m.viewMode == ViewModeSort:
		b.WriteString(m.renderSortDock())
		b.WriteString("\n\n")
	}

	b.WriteString(m.renderListBody())
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// listBodyHeight is how many lines the table and the expanded region share.
// Search, Filter, and Sort each dock an editor above the table instead of
// covering it, so the table shrinks by whichever one is open rather than
// disappearing behind a full-screen takeover.
func (m Model) listBodyHeight() int {
	height := m.height - listChromeHeight

	switch {
	case m.searching:
		height -= searchChromeHeight
	case m.viewMode == ViewModeFilter:
		height -= m.filterDockHeight()
	case m.viewMode == ViewModeSort:
		height -= sortDockHeight()
	}

	return max(height, 1)
}

func (m Model) renderRepoListBreadcrumbs() string {
	title := styles.TitleStyle.Render("repo-dashboard")

	badges := []string{}

	listable := len(m.listableRepos())

	repoCount := fmt.Sprintf("%d repos", len(m.filteredPaths))
	if len(m.filteredPaths) != listable {
		repoCount = fmt.Sprintf("%d/%d repos", len(m.filteredPaths), listable)
	}
	badges = append(badges, styles.Badge(repoCount, styles.CountBadgeStyle))

	if dirtyCount := m.DirtyCount(); dirtyCount > 0 {
		badges = append(badges, styles.Badge(fmt.Sprintf("%d dirty", dirtyCount), styles.FilterBadgeStyle))
	}

	if prCount := m.PRCount(); prCount > 0 {
		badges = append(badges, styles.Badge(fmt.Sprintf("%d PRs", prCount), styles.PROpenStyle))
	}

	if notesCount := m.NotesCount(); notesCount > 0 {
		badges = append(badges, styles.Badge(fmt.Sprintf("%d notes", notesCount), styles.NotesBadgeStyle))
	}

	if conflicts := m.BranchConflictCount(); conflicts > 0 {
		label := fmt.Sprintf("%s %d branch conflicts", conflictMark, conflicts)
		if conflicts == 1 {
			label = conflictMark + " 1 branch conflict"
		}
		badges = append(badges, styles.Badge(label, styles.WarningStyle))
	}

	if progress := m.loadingBadge(); progress != "" {
		badges = append(badges, styles.Badge(progress, styles.CountBadgeStyle))
	}

	return joinWithinWidth(title, badges, listWidth(m.width))
}

// loadingBadge reports what is still arriving, or is empty once the fleet has
// settled. Discovery counts against a denominator it knows, and the per-repo
// fetches it starts count down instead: CI is only asked for the rows on
// screen, so their total is not known in advance and a combined fraction would
// have to move its own denominator as the user scrolls.
func (m Model) loadingBadge() string {
	if m.loading {
		return fmt.Sprintf("Loading %d/%d", m.loadedCount, m.loadingCount)
	}

	if inFlight := m.fetchesInFlight(); inFlight > 0 {
		return fmt.Sprintf("Fetching %d", inFlight)
	}

	return ""
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

	if summary := m.markSummary(); summary != "" {
		parts = append(parts, styles.Badge(summary, styles.SortBadgeStyle))
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

// renderModalRow renders one cursor/mark/key/label row, shared by the docked
// filter and sort editors. Trailing is an extra styled column (a per-filter
// match count), left empty where there is none.
func renderModalRow(selected bool, mark, shortKey, label string, markStyle lipgloss.Style, trailing string) string {
	cursor := "  "
	rowStyle := styles.TableRowStyle
	if selected {
		cursor = "> "
		rowStyle = styles.SelectedRowStyle
	}

	keyStyle := lipgloss.NewStyle().Foreground(styles.Mauve).Bold(true)

	row := fmt.Sprintf("%s%s  %s  %s",
		cursor,
		markStyle.Render(padCell(mark, modalMarkColWidth)),
		keyStyle.Render(padCell(shortKey, modalKeyColWidth)),
		rowStyle.Render(padCell(label, filterLabelColWidth)))

	if trailing != "" {
		row += "  " + styles.SubtitleStyle.Render(trailing)
	}

	return row
}

// dockHeaderAndSpacerLines is the header row plus the blank separator line
// every docked editor (search, filter, sort) spends besides its own rows.
const dockHeaderAndSpacerLines = 2

// filterDockHeight is the line count renderFilterDock spends, so
// listBodyHeight can shrink the table by exactly that much. An active
// predicate adds one more line for its own row.
func (m Model) filterDockHeight() int {
	height := len(models.SelectableFilterModes()) + dockHeaderAndSpacerLines
	if m.predicateText != "" {
		height++
	}

	return height
}

// renderFilterDock renders the filter editor docked above the table: a
// header row, one row per filter mode showing its cycle state and the
// fleet-wide count if that mode were turned on, combined with everything
// else already active. The combined result of every active filter is
// already visible in the breadcrumb's "N/M repos" badge above, since docking
// (instead of a full-screen takeover) keeps that badge on screen while
// editing. An active :filter predicate gets its own row below the modes,
// since it is an extra AND term the mode rows can't represent or clear.
func (m Model) renderFilterDock() string {
	modes := models.SelectableFilterModes()

	headerStyle := lipgloss.NewStyle().Foreground(styles.Subtext0).Bold(true)
	header := fmt.Sprintf("  %s  %s  %s  %s",
		padCell("", modalMarkColWidth), padCell("Key", modalKeyColWidth),
		padCell("Filter", filterLabelColWidth), "If on")

	lines := make([]string, 1, 1+len(modes)+1)
	lines[0] = headerStyle.Render(header)

	for i, mode := range modes {
		var filterState models.ActiveFilter
		for _, f := range m.activeFilters {
			if f.Mode == mode {
				filterState = f
				break
			}
		}

		mark := "   "
		markStyle := lipgloss.NewStyle().Foreground(styles.Green)

		switch {
		case filterState.Enabled && filterState.Inverted:
			mark = "NOT"
			markStyle = lipgloss.NewStyle().Foreground(styles.Peach)
		case filterState.Enabled:
			mark = " ✓ "
		}

		lines = append(lines, renderModalRow(i == m.filterCursor, mark, mode.ShortKey(), mode.String(),
			markStyle, strconv.Itoa(m.countForFilter(mode))))
	}

	if m.predicateText != "" {
		lines = append(lines, renderModalRow(false, "AND", "x", m.predicateText,
			lipgloss.NewStyle().Foreground(styles.Peach), "clear"))
	}

	return strings.Join(lines, "\n")
}

// sortDockHeight is the line count renderSortDock spends, so listBodyHeight
// can shrink the table by exactly that much.
func sortDockHeight() int {
	return len(models.AllSortModes()) + dockHeaderAndSpacerLines
}

// renderSortDock renders the sort editor docked above the table: a header
// row, then one row per sort mode, active ones first in priority order.
func (m Model) renderSortDock() string {
	displaySorts := buildSortModalRows(m.activeSorts)

	headerStyle := lipgloss.NewStyle().Foreground(styles.Subtext0).Bold(true)
	header := fmt.Sprintf("  %s  %s  %s",
		padCell("", modalMarkColWidth), padCell("Key", modalKeyColWidth), "Sort By")

	lines := make([]string, 1, 1+len(displaySorts))
	lines[0] = headerStyle.Render(header)

	cursorIndex := -1

	for i, s := range displaySorts {
		if s.Mode == m.activeSorts[m.sortCursor].Mode {
			cursorIndex = i
			break
		}
	}

	for i, sortState := range displaySorts {
		mark := "   "
		if sortState.IsEnabled() {
			mark = fmt.Sprintf(" %d ", sortState.Priority+1)
		}

		label := sortState.DisplayName()
		if !sortState.IsEnabled() {
			label = sortState.Mode.String()
		}

		lines = append(lines, renderModalRow(i == cursorIndex, mark, sortState.ShortKey(), label,
			lipgloss.NewStyle().Foreground(styles.Green), ""))
	}

	return strings.Join(lines, "\n")
}

// renderListBody renders the repo table with the expanded region under it. The
// result is always exactly listBodyHeight lines, and the split between the two
// depends only on that height, so neither the selected repo nor what is still
// loading can move the footer.
func (m Model) renderListBody() string {
	expanded := m.expandHeight(m.listBodyHeight())
	list := fitBlock(m.renderTable(), m.tableHeight())

	if expanded == 0 {
		return list
	}

	return list + "\n" + fitBlock(strings.Join(m.expandLines(listWidth(m.width), expanded), "\n"), expanded)
}

// padBottom grows lines to exactly height by adding blanks below them, so a
// note shorter than its region starts under the row above it and the slack
// collects against the divider that closes the region.
func padBottom(lines []string, height int) []string {
	if len(lines) >= height {
		return lines[:height]
	}

	return append(lines, make([]string, height-len(lines))...)
}

// fitBlock pads or truncates a block of lines to exactly height lines.
func fitBlock(block string, height int) string {
	lines := strings.Split(block, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// rowSummary is the summary a list row renders from. A row exists from
// discovery, before its own summary has been read, and the zero summary carries
// no path: standing the path back in keeps the row named and lets every cell
// tell a value still being fetched from one that is genuinely absent.
func (m Model) rowSummary(path string) models.RepoSummary {
	summary, loaded := m.summaries[path]
	if !loaded {
		summary.Path = path
	}

	return summary
}

func (m Model) renderTable() string {
	if len(m.filteredPaths) == 0 {
		if m.loading {
			return m.loadingPlaceholder("Discovering repositories")
		}
		if len(m.repoPaths) > 0 {
			return m.emptyPlaceholder("No repositories match the active filters",
				"Press f to change filters, / to clear the search.")
		}

		return m.emptyPlaceholder("No repositories found",
			"Nothing was discovered under the configured scan paths.")
	}

	compact := m.isCompact()
	width := listWidth(m.width)

	var (
		rows      []string
		rowHeight = 1
	)

	layout := layoutRepoCols(width)
	if compact {
		layout = table.Fit(compactColSpecs, width-cursorWidth)
		rowHeight = compactRowHeight
	} else {
		rows = append(rows, styles.HeaderStyle.Render(renderRepoHeader(layout)))
	}

	window := m.visibleRepoRange(rowHeight)
	for i := window.start; i < window.end; i++ {
		path := m.filteredPaths[i]
		summary := m.rowSummary(path)

		if compact {
			rows = append(rows, m.renderCompactRow(summary, i == m.cursor, layout))
		} else {
			rows = append(rows, m.renderTableRow(summary, i == m.cursor, layout))
		}
	}

	return strings.Join(rows, "\n")
}

// Expanded region geometry. Opening the region is a request to read one repo in
// full, so it takes the larger share of the body; the table keeps enough rows
// above it that scrolling still moves through the fleet rather than
// re-centering a two-row window on every keypress. The minimum is the region's
// own fixed head plus a note line and the divider that captions it.
const (
	expandPercent = 60
	expandMinRows = expandHeadRows + 2
	listMinRows   = 6
)

// expandHeight is how many of the body's lines the expanded region holds, or
// zero when it is closed. It depends only on the body height, never on the
// selected repo, so scrolling between a repo with notes and one without cannot
// resize the table under the cursor.
func (m Model) expandHeight(body int) int {
	if !m.expandOpen {
		return 0
	}

	headerRows := 1
	if m.isCompact() {
		headerRows = 0
	}

	room := body - headerRows - listMinRows
	if room < expandMinRows {
		return 0
	}

	return min(body*expandPercent/percentDenominator, room)
}

// tableHeight is how many of the body's lines the table keeps once the expanded
// region has taken its share.
func (m Model) tableHeight() int {
	body := m.listBodyHeight()

	return body - m.expandHeight(body)
}

// percentDenominator scales the percentage budgets above; elisionEnds is how
// many ends of a list survive a middle elision.
const (
	percentDenominator = 100
	elisionEnds        = 2
)

// visibleRepoRange returns the half-open slice of filteredPaths that fits the
// window, keeping the cursor near the middle. The rowHeight argument is how
// many terminal lines one record covers, so the compact layout scrolls by
// records rather than by lines.
func (m Model) visibleRepoRange(rowHeight int) repoWindow {
	availableLines := m.tableHeight()
	if !m.isCompact() {
		availableLines--
	}

	return visibleRange(m.cursor, len(m.filteredPaths), availableLines/rowHeight)
}

// visibleRange returns the half-open slice of a list of the given length that
// fits visible rows, keeping the cursor near the middle.
func visibleRange(cursor, length, visible int) repoWindow {
	visible = max(visible, 1)

	start := max(cursor-visible/visibleWindowCenter, 0)

	end := start + visible
	if end > length {
		end = length
		start = max(end-visible, 0)
	}

	return repoWindow{start: start, end: end}
}

type repoWindow struct {
	start int
	end   int
}

// expandHeadRows is the region's fixed head: the rule that names the repo, one
// row each for peers, branches, and pull requests, and the divider that
// separates that metadata from the notes below it. Everything below it
// belongs to the notes, which is the only section whose length the repo decides.
const expandHeadRows = 5

// expandLines renders the region below the table for the repo under the cursor:
// a rule naming the repo, a row each for peers, branches, and pull requests,
// then the repo's notes over whatever height is left, closed by the divider
// that captions them. Sections whose data has not arrived say so rather than
// disappearing, so the region's shape never moves as data lands.
func (m Model) expandLines(width, height int) []string {
	if m.cursor >= len(m.filteredPaths) {
		return nil
	}

	path := m.filteredPaths[m.cursor]
	summary := m.rowSummary(path)
	data, loaded := m.prMap[path]
	peersPending := m.loading || !loaded || m.fetchPending(path, fetchPeerBranches)
	notes := m.expandNotes(path, summary, width)

	block := region.Region{
		Title: qualifiedRepoName(path) + compactSignalSep +
			section(m.summaryPending(path), overviewIdentity(summary)),
		Head: []region.Fact{
			{Label: tabNamePeers, Value: section(peersPending, overviewRelevantPeers(m.relevantPeers(path)))},
			{Label: tabNameBranches, Value: section(!loaded, expandBranches(data.Branches))},
			{Label: tabNamePRs, Value: section(!loaded, expandPRs(data.PRs))},
		},
		Section: "notes",
		Body:    notes.lines,
		Caption: qualifiedRepoName(path) + compactSignalSep + notes.caption,
	}

	return block.Render(regionStyles(), width, height)
}

// expandBranches counts the repo's local branches and names the ones holding
// commits their remote does not have, which are the branches with work on them.
// The default branch is left out: it is where the work lands, not work itself.
func expandBranches(branches []vcs.BranchInfo) string {
	if len(branches) == 0 {
		return emDash
	}

	defaultBranch := findDefaultBranch(branches)

	names := make([]string, 0, len(branches))
	for _, branch := range branches {
		if branch.Ahead == 0 || branch.Name == defaultBranch {
			continue
		}

		names = append(names, branch.Name+" ↑"+strconv.Itoa(branch.Ahead))
	}

	count := strconv.Itoa(len(branches)) + " local"
	if len(names) == 0 {
		return count
	}

	return count + compactSignalSep + strings.Join(names, ", ")
}

// expandPRs counts the repo's open pull requests and names them newest first.
func expandPRs(prs []forge.PullRequest) string {
	if len(prs) == 0 {
		return emDash
	}

	titles := make([]string, 0, len(prs))
	for i := range prs {
		titles = append(titles, "#"+strconv.Itoa(prs[i].Number)+" "+prs[i].Title)
	}

	return strconv.Itoa(len(prs)) + " open" + compactSignalSep + strings.Join(titles, ", ")
}

// notesBlock is the region's notes section: the note text itself, and the
// caption the closing divider carries, which is what names a note when nothing
// else on screen does.
type notesBlock struct {
	lines   []string
	caption string
}

// expandNotes renders the selected repo's notes at the foot of the region.
func (m Model) expandNotes(path string, summary models.RepoSummary, width int) notesBlock {
	if len(summary.NotesFiles) == 0 {
		return notesBlock{caption: "no notes"}
	}

	contents, loaded := m.notesPreview[path]
	if !loaded {
		return notesBlock{caption: readingLabel}
	}

	// One note needs no heading of its own: the divider already names the
	// file. Several do, or their text runs together.
	if len(contents) == 1 {
		return notesBlock{lines: notesBodyLines(contents[0].Content, width), caption: contents[0].Name}
	}

	lines := make([]string, 0, len(contents))
	for i := range contents {
		lines = append(lines, notesFileRule(contents[i].Name, width))
		lines = append(lines, notesBodyLines(contents[i].Content, width)...)
	}

	return notesBlock{
		lines:   lines,
		caption: strconv.Itoa(len(contents)) + " " + plural(len(contents), "note", "notes"),
	}
}

// qualifiedRepoName names a repo by its parent directory as well as its own,
// which is what tells two checkouts of the same project apart.
func qualifiedRepoName(path string) string {
	return filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path))
}

// notesFileRule heads one note's text when the repo has more than one.
func notesFileRule(label string, width int) string {
	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-notesRuleLead-notesRuleSpaces, 0))

	return styles.SubtitleStyle.Render(strings.Repeat("─", notesRuleLead)+" ") +
		styles.NotesPreviewNameStyle.Render(label) +
		styles.SubtitleStyle.Render(" "+rule)
}

// notesRuleSpaces is the blank on each side of a rule's label.
const (
	notesRuleLead   = 2
	notesRuleSpaces = 2
)

// notesBodyLines styles a note's text one line per source line, cutting the
// middle of any line too wide for the region.
func notesBodyLines(content string, width int) []string {
	body := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(body) == 1 && strings.TrimSpace(body[0]) == "" {
		return []string{styles.SubtitleStyle.Render("  (empty)")}
	}

	lines := make([]string, len(body))
	for i, line := range body {
		style := styles.NotesPreviewLineStyle
		if strings.HasPrefix(strings.TrimSpace(line), "!") {
			style = styles.NotesPreviewBangStyle
		}

		lines[i] = style.Render(table.TruncateMiddle(strings.TrimRight(line, " \t"), width))
	}

	return lines
}

// elideMiddle drops the middle of lines until it fits height, marking the cut,
// so the newest entries at the end of a note survive alongside the oldest.
func elideMiddle(lines []string, height int) []string {
	if len(lines) <= height || height < 1 {
		return lines
	}

	kept := height - 1
	head := kept / elisionEnds
	tail := kept - head

	out := make([]string, 0, height)
	out = append(out, lines[:head]...)
	out = append(out, styles.SubtitleStyle.Render(
		"  ⋯ "+strconv.Itoa(len(lines)-head-tail)+" more lines ⋯"))

	return append(out, lines[len(lines)-tail:]...)
}

// prCell renders the pull request open on the checked-out branch, if any,
// plus a "+N" for the repo's other open pull requests once that count has
// loaded, so the list's PR column carries the count a separate PRs column
// used to. The own-PR text truncates first, reserving room for the suffix,
// the same way branchCell does.
func (m Model) prCell(s models.RepoSummary, width int) string {
	text := absentCell(m.fetchPending(s.Path, fetchPR) || m.summaryPending(s.Path))
	if s.PRInfo != nil {
		text = formatPRCell(s)
	}

	others := m.prCount[s.Path]
	if s.PRInfo != nil {
		others--
	}

	suffix := ""
	if others > 0 {
		suffix = " +" + strconv.Itoa(others)
	}

	text = table.Truncate(text, max(width-lipgloss.Width(suffix), 0))

	return text + suffix
}

// templateCell renders the copier-template column, standing in the
// pending-or-absent placeholder for a repo whose template info has not arrived.
func (m Model) templateCell(s models.RepoSummary, width int) string {
	if s.TemplateInfo == nil {
		return absentCell(m.fetchPending(s.Path, fetchTemplate) || m.summaryPending(s.Path))
	}

	return formatCopierCell(s, width)
}

// formatPRCell formats a repo's PR-column text: "#N" with a review-status
// indicator and a CI/workflow failure indicator, or emDash if there's no PR.
func formatPRCell(s models.RepoSummary) string {
	if s.PRInfo == nil {
		return emDash
	}

	prNum := fmt.Sprintf("#%d", s.PRInfo.Number)

	switch ui.PRReviewStatus(*s.PRInfo) {
	case forge.ReviewApproved:
		prNum += " ✓"
	case forge.ReviewChangesRequested:
		prNum += " ✗"
	}

	switch {
	case s.PRInfo.Checks.Total > 0:
		if ui.ChecksSummary(s.PRInfo.Checks) == forge.StatusFailing {
			prNum += " ⚠"
		}
	case s.WorkflowInfo != nil:
		if ui.WorkflowSummaryStatusDisplay(*s.WorkflowInfo) == forge.StatusFailing {
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

func renderRepoHeader(layout table.Layout) string {
	return strings.Repeat(" ", cursorWidth) + table.Header(layout)
}

// peersCell renders the count of peer checkouts holding a branch that tracks
// one of this repo's open pull requests, or the pending-or-absent placeholder
// while that is still being worked out. A repo with other checkouts but none
// on a shared pull request reads as absent, not as zero peers: the noise the
// filter exists to remove.
func (m Model) peersCell(path string, base lipgloss.Style, selected bool) (string, lipgloss.Style) {
	pending := m.loading || m.fetchPending(path, fetchExpand) || m.fetchPending(path, fetchPeerBranches)

	peers := m.relevantPeers(path)
	if len(peers) == 0 {
		return absentCell(pending), base
	}

	cell := "⧉ " + strconv.Itoa(len(peers))
	if m.hasBranchConflict(path) {
		return cell + conflictMark, withSelection(styles.WarningStyle, selected)
	}

	return cell, withSelection(styles.CountStyle, selected)
}

// ciBucketGlyphs names the CI column's four tracked run outcomes, in the
// fixed order both the header legend and every row's numbers use, alongside
// the color a nonzero count takes. A count of zero renders dim instead, so
// the buckets that actually happened are what draws the eye.
var ciBucketGlyphs = []struct {
	icon  string
	style lipgloss.Style
}{
	{"✓", styles.CleanStyle},
	{"⊘", styles.CountStyle},
	{"⊗", styles.WarningStyle},
	{"✗", styles.ErrorStyle},
}

// ciColumnTitle builds the CI column's header: the column name followed by
// the four outcome icons in their semantic colors, so the row below reads as
// numbers under a legend rather than requiring the glossary to decode.
func ciColumnTitle() string {
	icons := make([]string, len(ciBucketGlyphs))
	for i := range ciBucketGlyphs {
		icons[i] = ciBucketGlyphs[i].style.Render(ciBucketGlyphs[i].icon)
	}

	return colCI + " " + strings.Join(icons, "/")
}

// ciListCell renders the CI column's per-outcome breakdown for one row: how
// many of the default branch's most recent workflow runs passed, were
// skipped, were canceled, and failed, in ciBucketGlyphs' order, padded to
// width itself so the row's selection background can be painted behind every
// separator and pad space rather than just the colored digits. A run still
// in progress prefixes the count rather than taking one of the four slots,
// since it has not resolved into any of those outcomes yet.
func (m Model) ciListCell(s models.RepoSummary, width int, selected bool) string {
	rowStyle := withSelection(styles.TableRowStyle, selected)

	if s.WorkflowInfo == nil {
		text := m.pendingCell(m.fetchPending(s.Path, fetchCI) || m.summaryPending(s.Path))

		return rowStyle.Render(padCell(text, width))
	}

	wf := s.WorkflowInfo
	if wf.Total == 0 {
		return rowStyle.Render(padCell(emDash, width))
	}

	counts := []int{wf.Passing, wf.Skipped, wf.Canceled, wf.Failing}

	parts := make([]string, len(counts))
	for i, count := range counts {
		style := withSelection(styles.SubtitleStyle, selected)
		if count > 0 {
			style = withSelection(ciBucketGlyphs[i].style, selected)
		}

		parts[i] = style.Render(strconv.Itoa(count))
	}

	sep := rowStyle.Render("/")
	cell := strings.Join(parts, sep)

	if wf.InProgress > 0 {
		running := withSelection(styles.WarningStyle, selected).Render("…" + strconv.Itoa(wf.InProgress))
		cell = running + rowStyle.Render(" ") + cell
	}

	if gap := width - lipgloss.Width(cell); gap > 0 {
		cell += rowStyle.Render(strings.Repeat(" ", gap))
	}

	return cell
}

// ciCell renders the default branch's CI rollup as a single word: a check
// when every run passed, the count of failures when not, the pending glyph
// while the fetch is in flight, and emDash once it is known there is nothing
// to report. Used where a line of text has room for one signal, not a table
// column; see ciListCell for the Repos list's fuller breakdown.
func (m Model) ciCell(s models.RepoSummary) string {
	if s.WorkflowInfo == nil {
		return absentCell(m.fetchPending(s.Path, fetchCI) || m.summaryPending(s.Path))
	}

	wf := s.WorkflowInfo
	switch {
	case wf.Total == 0:
		return emDash
	case wf.Failing > 0:
		return "✗ " + strconv.Itoa(wf.Failing) + "/" + strconv.Itoa(wf.Total)
	case wf.InProgress > 0:
		return "…" + strconv.Itoa(wf.InProgress)
	default:
		return "✓"
	}
}

// conflictMark flags a repo whose branch is checked out somewhere else too.
const conflictMark = "⚠"

// hasBranchConflict reports whether the repo shares its branch with one of its
// peer checkouts, the state where a commit made in one is invisible to the
// other.
func (m Model) hasBranchConflict(path string) bool {
	summary, ok := m.summaries[path]
	if !ok {
		return false
	}

	return models.ConflictingBranches(ownCheckoutOf(&summary), m.PeerCheckouts(path))[summary.Branch]
}

// ownCheckoutOf adapts a summary into the checkout form the conflict check
// compares its peers against.
func ownCheckoutOf(summary *models.RepoSummary) *models.PeerCheckout {
	own := models.OwnCheckout(summary)

	return &own
}

// BranchConflictCount counts the repos sharing a branch with a peer checkout.
// It groups every summary by remote once rather than checking each repo
// against a freshly rebuilt full scan of every other repo, which made this
// call quadratic in fleet size despite running on every render.
func (m Model) BranchConflictCount() int {
	filtered := make(map[string]bool, len(m.filteredPaths))
	for _, path := range m.filteredPaths {
		filtered[path] = true
	}

	const minSharedRemote = 2 // a conflict needs at least one peer sharing the remote

	count := 0
	for _, group := range m.summariesByRemote() {
		if len(group) < minSharedRemote {
			continue
		}

		for i := range group {
			if !filtered[group[i].Path] {
				continue
			}

			own := ownCheckoutOf(&group[i])

			peers := make([]models.PeerCheckout, 0, len(group)-1)
			for j := range group {
				if j != i {
					peers = append(peers, models.OwnCheckout(&group[j]))
				}
			}

			if models.ConflictingBranches(own, peers)[group[i].Branch] {
				count++
			}
		}
	}

	return count
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

	return base.Render(rowCursor(m.isMarked(s.Path), selected)) +
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
		return withSelection(styles.BranchStyle, selected).Render(padCell(m.branchCell(s, width), width))
	case colStatus:
		return m.statusCell(s, width, base, selected)
	case colPeers:
		text, style := m.peersCell(s.Path, base, selected)

		return style.Render(padCell(text, width))
	case colPR:
		return prCellStyle(s, base, selected).Render(padCell(m.prCell(s, width), width))
	case colTemplate:
		return templateCellStyle(s, base, selected).Render(padCell(m.templateCell(s, width), width))
	case colCI:
		return m.ciListCell(s, width, selected)
	case colModified:
		if m.summaryPending(s.Path) {
			return base.Render(padCell(pendingGlyph, width))
		}

		return base.Render(padCell(ui.RepoRelativeModified(s.RepoSummary), width))
	default:
		return base.Render(padCell("", width))
	}
}

// statusCell renders the status summary and the notes marker that shares its
// column, keeping the marker right-aligned within the column. A summary still
// being read has no state to report, and reporting a clean tree before the read
// lands would be a claim about the repo rather than a placeholder.
func (m Model) statusCell(s models.RepoSummary, width int, base lipgloss.Style, selected bool) string {
	notesText, notesStyle := notesMarker(s, base, selected)
	style := withSelection(statusCellStyle(s, base), selected)

	summary := ui.RepoStatusSummary(s.RepoSummary)
	if m.summaryPending(s.Path) {
		summary = pendingGlyph
		style = base
	}

	return style.Render(padCell(summary, width-notesMarkerWidth)) +
		notesStyle.Render(padCell(notesText, notesMarkerWidth))
}

// branchCell renders the checked-out branch name plus, once the repo's local
// branch count has loaded, a "+N" for the other branches it holds, so the
// list's BRANCH column carries the same count a separate BRs column used to.
// The name truncates first, reserving room for the suffix, so a long branch
// name never crowds out the count the way trimming the combined string from
// the tail would.
func (m Model) branchCell(s models.RepoSummary, width int) string {
	suffix := ""
	if count, ok := m.branchCount[s.Path]; ok && count > 1 {
		suffix = " +" + strconv.Itoa(count-1)
	}

	name := table.Truncate(s.Branch, max(width-lipgloss.Width(suffix), 0))

	return name + suffix
}

// pendingCell marks a cell whose fetch is still in flight with the spinner
// rather than emDash, so a count still loading cannot be mistaken for a count
// of zero.
func (m Model) pendingCell(pending bool) string {
	if pending {
		return m.spinner.View()
	}

	return emDash
}

func statusCellStyle(s models.RepoSummary, base lipgloss.Style) lipgloss.Style {
	switch {
	case s.IsDirty():
		return styles.DirtyStyle
	case s.Status() == vcs.RepoStatusClean:
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

// The up/down hint, shared by every footer that has one.
const (
	keyNavPair = "j/k"
	descNav    = "nav"
)

// Hints repeated across more than one footer.
const (
	descBack    = "back"
	descCopyURL = "copy URL"
	descOpen    = "open"
	descOpenPR  = "open in browser"
)

// footerHint is one key hint and how readily it is dropped when the footer no
// longer fits. Priority is the hint's value at a glance, so the lowest goes
// first and the survivors are the keys a new user needs: enter, /, f, s, ?, q.
type footerHint struct {
	key      string
	desc     string
	priority int
}

// footerHints lists every repo-list hint in render order. The expand hint
// outranks the rest of the second tier while the region is open, because the
// key that closes a region taking most of the screen must stay on offer.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func footerHints(expandOpen bool, marked string) []footerHint {
	expand := footerHint{key: "v", desc: "expand", priority: 3}
	if expandOpen {
		expand = footerHint{key: "v", desc: "hide", priority: 11}
	}

	// The mark hint states the count once there is one, since that count is
	// what every operator is about to act on and nothing else on screen says
	// it. With nothing marked it names the key's job instead.
	mark := footerHint{key: markToggleKey, desc: "mark", priority: 2}
	if marked != "" {
		mark = footerHint{key: markToggleKey, desc: marked, priority: 12}
	}

	return []footerHint{
		{key: keyNavPair, desc: descNav, priority: 4},
		{key: keyEnter, desc: nameSelect, priority: 8},
		expand,
		mark,
		{key: actionLeader, desc: nameActions, priority: 7},
		{key: "f", desc: nameFilter, priority: 6},
		{key: "s", desc: nameSort, priority: 5},
		{key: "/", desc: "search", priority: 5},
		{key: ":", desc: "command", priority: 1},
		{key: "r", desc: nameRefresh, priority: 1},
		{key: "?", desc: nameHelp, priority: 9},
		{key: "q", desc: nameQuit, priority: 10},
	}
}

// filterDockFooterHints lists the filter dock's own keys, so the grammar
// (cycle, invert, reset, clear the predicate) is visible while the dock is
// actually open rather than only behind the help overlay.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func filterDockFooterHints(hasPredicate bool) []footerHint {
	hints := []footerHint{
		{key: keyNavPair, desc: descNav, priority: 5},
		{key: "enter/key", desc: "cycle", priority: 6},
		{key: "*", desc: "reset", priority: 4},
		{key: keyEsc, desc: descBack, priority: 7},
		{key: "q", desc: nameQuit, priority: 8},
	}
	if hasPredicate {
		hints = append(hints, footerHint{key: "x", desc: "clear predicate", priority: 3})
	}

	return hints
}

// sortDockFooterHints lists the sort dock's own keys.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func sortDockFooterHints() []footerHint {
	return []footerHint{
		{key: keyNavPair, desc: descNav, priority: 5},
		{key: "enter/key", desc: "cycle", priority: 6},
		{key: keyBracketPair, desc: "reorder", priority: 4},
		{key: "*", desc: "reset", priority: 3},
		{key: keyEsc, desc: descBack, priority: 7},
		{key: "q", desc: nameQuit, priority: 8},
	}
}

func (m Model) renderFooter() string {
	prefix := ""
	if m.pendingOperator != "" {
		hint := m.pendingOperator + m.pendingObject
		pendingHint := " pending (ar/br/dr/pr/sr, " +
			strings.ToLower(m.pendingOperator) + "=all, esc cancels)"
		prefix = styles.FooterKeyStyle.Render(hint) + styles.FooterDescStyle.Render(pendingHint) + "  "
	}

	var dockHints []footerHint

	switch m.viewMode {
	case ViewModeFilter:
		dockHints = filterDockFooterHints(m.predicateText != "")
	case ViewModeSort:
		dockHints = sortDockFooterHints()
	default:
		dockHints = footerHints(m.expandOpen, m.markSummary())
	}

	return prefix + renderHints(fittingHints(dockHints, listWidth(m.width)-lipgloss.Width(prefix)))
}

// hintLabel writes one hint and says which bytes of it are the key, so the two
// can be colored apart. A description containing its own key gets the key
// bracketed inside the word it triggers ("pre[v]iew"), the way the tab bar and
// the panel borders write theirs. A description that does not contain it takes
// the key in front of it bare, since brackets around a letter that is not in
// the word say nothing the color has not already said.
func hintLabel(h footerHint) hintPart {
	marked := markHotkey(h.desc, h.key)
	if strings.HasPrefix(h.key, "[") || marked == "["+h.key+"] "+h.desc {
		return hintPart{key: h.key, after: " " + h.desc}
	}

	bracketed := "[" + h.key + "]"
	before, after, _ := strings.Cut(marked, bracketed)

	return hintPart{before: before, key: bracketed, after: after}
}

// hintPart is one rendered hint split at its key.
type hintPart struct {
	before string
	key    string
	after  string
}

func (p hintPart) width() int {
	return lipgloss.Width(p.before) + lipgloss.Width(p.key) + lipgloss.Width(p.after)
}

// renderHints draws a fitted hint row, styling each hint's key apart from the
// words around it.
func renderHints(hints []footerHint) string {
	parts := make([]string, 0, len(hints))

	for _, h := range hints {
		label := hintLabel(h)
		parts = append(parts, styles.FooterDescStyle.Render(label.before)+
			styles.FooterKeyStyle.Render(label.key)+
			styles.FooterDescStyle.Render(label.after))
	}

	return strings.Join(parts, "  ")
}

// fittingHints drops the lowest-priority hints until the rendered footer fits
// in width, so the line ends on a whole hint instead of a clipped one.
func fittingHints(hints []footerHint, width int) []footerHint {
	kept := make([]footerHint, len(hints))
	copy(kept, hints)

	for len(kept) > 1 && hintsWidth(kept) > width {
		victim := 0
		for i, h := range kept {
			if h.priority < kept[victim].priority {
				victim = i
			}
		}

		kept = append(kept[:victim], kept[victim+1:]...)
	}

	return kept
}

func hintsWidth(hints []footerHint) int {
	total := 0
	for i, h := range hints {
		if i > 0 {
			total += table.Gutter
		}

		total += hintLabel(h).width()
	}

	return total
}

// regionStyles are the faces the expandable regions draw with, kept in one
// place so the Repos and PRs tabs cannot drift apart.
func regionStyles() region.Styles {
	return region.Styles{Rule: styles.SubtitleStyle, Label: styles.NotesPreviewNameStyle}
}
