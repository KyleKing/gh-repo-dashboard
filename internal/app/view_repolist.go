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

// Vertical budget of the repo list. The body is sized once and rendered to
// exactly that many lines, so nothing the body does (a notes preview opening,
// a shorter repo set) can move the footer.
const (
	listChromeHeight   = 6
	searchChromeHeight = 2
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

	b.WriteString(m.renderListWithPreview(m.listBodyHeight()))
	b.WriteString("\n\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

// listBodyHeight is how many lines the list and its preview panel share.
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

	if conflicts := m.BranchConflictCount(); conflicts > 0 {
		label := fmt.Sprintf("%s %d branch conflicts", conflictMark, conflicts)
		if conflicts == 1 {
			label = conflictMark + " 1 branch conflict"
		}
		badges = append(badges, styles.Badge(label, styles.WarningStyle))
	}

	if m.loading {
		progress := fmt.Sprintf("Loading %d/%d", m.loadedCount, m.loadingCount)
		badges = append(badges, styles.Badge(progress, styles.CountBadgeStyle))
	}

	return joinWithinWidth(title, badges, contentWidth(m.width))
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

// renderListWithPreview renders the repo table, mounting the overview panel
// beside it when the terminal is wide enough to carry one. The result is
// always exactly height lines.
func (m Model) renderListWithPreview(height int) string {
	list := m.renderTable(height)

	panel := panelWidth(m.width, m.height)
	if panel == 0 || len(m.filteredPaths) == 0 || m.cursor >= len(m.filteredPaths) {
		return fitBlock(list, height)
	}

	summary := m.summaries[m.filteredPaths[m.cursor]]

	overview := m.renderOverview(summary, overviewOpts{width: panel, standalone: true})

	return joinListAndPanel(list, overview, listWidth(m.width, m.height), panel, height)
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

func (m Model) renderTable(height int) string {
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
	width := listWidth(m.width, m.height)

	var (
		rows      []string
		rowHeight = 1
	)

	// The preview leads. Its height is fixed, so the table below it starts on
	// the same line whatever the note says, and moving the cursor swaps the
	// text in place instead of sliding the rows around it.
	if preview := m.notesPreviewHeight(height); preview > 0 {
		rows = append(rows, fitBlock(
			strings.Join(elideMiddle(m.notesPreviewLines(width), preview), "\n"), preview))
	}

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
		summary := m.summaries[path]

		if compact {
			rows = append(rows, m.renderCompactRow(summary, i == m.cursor, layout))
		} else {
			rows = append(rows, m.renderTableRow(summary, i == m.cursor, layout))
		}
	}

	return strings.Join(rows, "\n")
}

// Notes preview geometry. Opening the preview is a request to read the notes,
// so it takes the larger share of the body; the list keeps enough rows below
// it that scrolling still moves through the fleet rather than re-centering a
// two-row window on every keypress.
const (
	notesPreviewPercent = 60
	notesPreviewMinRows = 4
	listMinRows         = 6
)

// notesPreviewHeight is how many of the body's lines the notes preview holds,
// or zero when it is closed. It depends only on the body height, never on the
// selected repo, so scrolling between a repo with notes and one without cannot
// resize the list under the cursor.
func (m Model) notesPreviewHeight(body int) int {
	if !m.notesPreviewOpen {
		return 0
	}

	headerRows := 1
	if m.isCompact() {
		headerRows = 0
	}

	room := body - headerRows - listMinRows
	if room < notesPreviewMinRows {
		return 0
	}

	return min(body*notesPreviewPercent/percentDenominator, room)
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
	body := m.listBodyHeight()

	availableLines := body - m.notesPreviewHeight(body)
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

// notesPreviewLines renders the selected repo's notes below the list: a rule
// naming each file, then its text.
func (m Model) notesPreviewLines(width int) []string {
	if m.cursor >= len(m.filteredPaths) {
		return nil
	}

	path := m.filteredPaths[m.cursor]
	summary := m.summaries[path]

	if len(summary.NotesFiles) == 0 {
		return []string{notesPreviewRule("no notes", width)}
	}

	contents, loaded := m.notesPreview[path]
	if !loaded {
		return []string{notesPreviewRule("reading notes…", width)}
	}

	var lines []string
	for i := range contents {
		lines = append(lines, notesPreviewRule(contents[i].Name, width))
		lines = append(lines, notesBodyLines(contents[i].Content, width)...)
	}

	return lines
}

// notesPreviewRule draws the labeled rule that opens the preview region.
func notesPreviewRule(label string, width int) string {
	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-notesRuleLead-notesRuleSpaces, 0))

	return styles.SubtitleStyle.Render(strings.Repeat("─", notesRuleLead)+" ") +
		styles.NotesPreviewNameStyle.Render(label) +
		styles.SubtitleStyle.Render(" "+rule)
}

// notesRuleSpaces is the blank on each side of the rule's label.
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
		lines[i] = styles.NotesPreviewLineStyle.Render(
			table.TruncateMiddle("  "+strings.TrimRight(line, " \t"), width))
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

// peersCell renders a repo's parallel-checkout count, or emDash when the repo
// is the only checkout of its remote.
func (m Model) peersCell(path string, base lipgloss.Style, selected bool) (string, lipgloss.Style) {
	peers := m.PeerCheckouts(path)
	if len(peers) == 0 {
		return emDash, base
	}

	cell := "⧉ " + strconv.Itoa(len(peers))
	if m.hasBranchConflict(path) {
		return cell + conflictMark, withSelection(styles.WarningStyle, selected)
	}

	return cell, withSelection(styles.CountStyle, selected)
}

// ciCell renders the default branch's CI rollup: a check when every run
// passed, the count of failures when not, an ellipsis while the fetch is in
// flight, and emDash once it is known there is nothing to report.
func (m Model) ciCell(s models.RepoSummary, base lipgloss.Style, selected bool) (string, lipgloss.Style) {
	if s.WorkflowInfo == nil {
		if m.ciRequested[s.Path] && !m.ciSettled[s.Path] {
			return "…", base
		}

		return emDash, base
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
	case colCI:
		text, style := m.ciCell(s, base, selected)

		return style.Render(padCell(text, width))
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

// The up/down hint, shared by every footer that has one.
const (
	keyNavPair = "j/k"
	descNav    = "nav"
)

// footerHint is one key hint and how readily it is dropped when the footer no
// longer fits. Priority is the hint's value at a glance, so the lowest goes
// first and the survivors are the keys a new user needs: enter, /, f, s, ?, q.
type footerHint struct {
	key      string
	desc     string
	priority int
}

// footerHints lists every repo-list hint in render order.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func footerHints() []footerHint {
	return []footerHint{
		{key: keyNavPair, desc: descNav, priority: 4},
		{key: keyEnter, desc: "select", priority: 8},
		{key: "v", desc: "notes", priority: 1},
		{key: "f", desc: nameFilter, priority: 6},
		{key: "s", desc: nameSort, priority: 5},
		{key: "/", desc: "search", priority: 7},
		{key: ":", desc: "command", priority: 3},
		{key: "r", desc: nameRefresh, priority: 2},
		{key: "?", desc: nameHelp, priority: 9},
		{key: "q", desc: nameQuit, priority: 10},
	}
}

func (m Model) renderFooter() string {
	prefix := ""
	if m.pendingOperator != "" {
		hint := m.pendingOperator + m.pendingObject
		pendingHint := " pending (ar/br/dr/pr/sr, " + m.pendingOperator + m.pendingOperator + "=all, esc cancels)"
		prefix = styles.FooterKeyStyle.Render(hint) + styles.FooterDescStyle.Render(pendingHint) + "  "
	}

	hints := fittingHints(footerHints(), contentWidth(m.width)-lipgloss.Width(prefix))

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
