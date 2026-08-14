package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// Vertical budget of the repo list. The body is sized once and rendered to
// exactly that many lines, so nothing the body does (the region opening, a
// shorter repo set) can move the footer.
const (
	listChromeHeight   = 8
	searchChromeHeight = 2
)

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

	if m.searching {
		b.WriteString(m.searchInput.View())
		b.WriteString("\n\n")
	}

	b.WriteString(m.renderListBody())
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// listBodyHeight is how many lines the table and the expanded region share.
func (m Model) listBodyHeight() int {
	height := m.height - listChromeHeight
	if m.searching {
		height -= searchChromeHeight
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

// renderListBody renders the repo table with the expanded region under it. The
// result is always exactly listBodyHeight lines, and the split between the two
// depends only on that height, so neither the selected repo nor what is still
// loading can move the footer.
func (m Model) renderListBody() string {
	region := m.expandHeight(m.listBodyHeight())
	list := fitBlock(m.renderTable(), m.tableHeight())

	if region == 0 {
		return list
	}

	return list + "\n" + fitBlock(strings.Join(m.expandLines(listWidth(m.width), region), "\n"), region)
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
// own fixed head plus a note line and the divider that captions it, since a
// region too short for those says nothing the table did not already.
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

// expandLabelCol is the width the head's labels are padded to, so their values
// line up in a column of their own.
const expandLabelCol = 10

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

	lines := []string{
		notesFileRule(qualifiedRepoName(path)+compactSignalSep+
			section(m.summaryPending(path), overviewIdentity(summary)), width),
		expandRow("Peers", section(peersPending, overviewRelevantPeers(m.relevantPeers(path))), width),
		expandRow(tabNameBranches, section(!loaded, expandBranches(data.Branches)), width),
		expandRow(tabNamePRs, section(!loaded, expandPRs(data.PRs)), width),
		notesFileRule("notes", width),
	}

	notes := m.expandNotes(path, summary, width)

	room := height - len(lines) - 1
	lines = append(lines, padBottom(elideMiddle(notes.lines, room), room)...)

	return append(lines, notesDivider(qualifiedRepoName(path), notes.caption, width))
}

// expandRow renders one label/value line of the region's head.
func expandRow(label, value string, width int) string {
	return styles.SubtitleStyle.Render(table.Pad(label, expandLabelCol, table.AlignLeft)) +
		" " + table.Truncate(value, width-expandLabelCol-1)
}

// expandBranches counts the repo's local branches and names the ones holding
// commits their remote does not have, which are the branches with work on them.
// The default branch is left out: it is where the work lands, not work itself.
func expandBranches(branches []models.BranchInfo) string {
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
func expandPRs(prs []models.PRInfo) string {
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

// notesDivider closes the region, captioning what sits above it on the right
// where the text has ended.
func notesDivider(repo, detail string, width int) string {
	label := repo + compactSignalSep + detail
	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-notesRuleSpaces-notesRuleLead, 0))

	return styles.SubtitleStyle.Render(rule+" ") +
		styles.NotesPreviewNameStyle.Render(label) +
		styles.SubtitleStyle.Render(" "+strings.Repeat("─", notesRuleLead))
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

// prCell renders the PR column: the pull request when the repo has one, and
// the pending-or-absent placeholder when it does not.
func (m Model) prCell(s models.RepoSummary) string {
	if s.PRInfo == nil {
		return absentCell(m.fetchPending(s.Path, fetchPR) || m.summaryPending(s.Path))
	}

	return formatPRCell(s)
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

// ciCell renders the default branch's CI rollup: a check when every run
// passed, the count of failures when not, the pending glyph while the fetch is
// in flight, and emDash once it is known there is nothing to report.
func (m Model) ciCell(s models.RepoSummary, base lipgloss.Style, selected bool) (string, lipgloss.Style) {
	if s.WorkflowInfo == nil {
		return absentCell(m.fetchPending(s.Path, fetchCI) || m.summaryPending(s.Path)), base
	}

	wf := s.WorkflowInfo
	switch {
	case wf.Total == 0:
		return emDash, base
	case wf.Failing > 0:
		text := "✗ " + strconv.Itoa(wf.Failing) + "/" + strconv.Itoa(wf.Total)

		return text, withSelection(styles.ErrorStyle, selected)
	case wf.InProgress > 0:
		return "…" + strconv.Itoa(wf.InProgress), withSelection(styles.WarningStyle, selected)
	default:
		return "✓", withSelection(styles.CleanStyle, selected)
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
func (m Model) BranchConflictCount() int {
	count := 0
	for _, path := range m.filteredPaths {
		if m.hasBranchConflict(path) {
			count++
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
	case colBranches:
		return base.Render(padCell(m.branchCountText(s.Path), width))
	case colStatus:
		return m.statusCell(s, width, base, selected)
	case colPeers:
		text, style := m.peersCell(s.Path, base, selected)

		return style.Render(padCell(text, width))
	case colPR:
		return prCellStyle(s, base, selected).Render(padCell(m.prCell(s), width))
	case colPRs:
		return base.Render(padCell(m.prCountText(s.Path), width))
	case colTemplate:
		return templateCellStyle(s, base, selected).Render(padCell(m.templateCell(s, width), width))
	case colCI:
		text, style := m.ciCell(s, base, selected)

		return style.Render(padCell(text, width))
	case colModified:
		if m.summaryPending(s.Path) {
			return base.Render(padCell(pendingGlyph, width))
		}

		return base.Render(padCell(s.RelativeModified(), width))
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

	summary := s.StatusSummary()
	if m.summaryPending(s.Path) {
		summary = pendingGlyph
		style = base
	}

	return style.Render(padCell(summary, width-notesMarkerWidth)) +
		notesStyle.Render(padCell(notesText, notesMarkerWidth))
}

func (m Model) prCountText(path string) string {
	if count, ok := m.prCount[path]; ok && count > 0 {
		return strconv.Itoa(count)
	}

	return absentCell(m.fetchPending(path, fetchPRCount) || m.summaryPending(path))
}

// branchCountText reports the repo's local branch count, reading the branch
// list the fleet map already loaded rather than fetching one of its own.
func (m Model) branchCountText(path string) string {
	if data, ok := m.prMap[path]; ok {
		return strconv.Itoa(len(data.Branches))
	}

	return absentCell(m.fetchPending(path, fetchExpand) || m.summaryPending(path))
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

// The up/down hint, shared by every footer that has one.
const (
	keyNavPair = "j/k"
	descNav    = "nav"
)

// Hints repeated across more than one footer.
const (
	descBack    = "back"
	descCopyURL = "copy URL"
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
func footerHints(expandOpen bool) []footerHint {
	expand := footerHint{key: "v", desc: "expand", priority: 3}
	if expandOpen {
		expand = footerHint{key: "v", desc: "hide", priority: 11}
	}

	return []footerHint{
		{key: keyNavPair, desc: descNav, priority: 4},
		{key: keyEnter, desc: nameSelect, priority: 8},
		expand,
		{key: "f", desc: nameFilter, priority: 6},
		{key: "s", desc: nameSort, priority: 5},
		{key: "/", desc: "search", priority: 7},
		{key: ":", desc: "command", priority: 2},
		{key: "r", desc: nameRefresh, priority: 1},
		{key: "?", desc: nameHelp, priority: 9},
		{key: "q", desc: nameQuit, priority: 10},
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

	hints := fittingHints(footerHints(m.expandOpen), listWidth(m.width)-lipgloss.Width(prefix))

	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts,
			styles.FooterKeyStyle.Render(h.key)+
				styles.FooterDescStyle.Render(" "+h.desc))
	}

	return prefix + strings.Join(parts, "  ")
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

		total += lipgloss.Width(h.key) + 1 + lipgloss.Width(h.desc)
	}

	return total
}
