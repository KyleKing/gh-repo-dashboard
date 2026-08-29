package app

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/aragonite/tui/table"
)

// Grid geometry. The side column holds the panels; the rest is the detail
// pane, which always renders the selected item rather than waiting for a key.
const (
	panelBorderWidth         = 2
	sideColumnFraction       = 100
	sideColumnPercent        = 25
	sideColumnFocusedPercent = 15
	minSideColumnWidth       = 24
	minDetailPaneWidth       = 30
	gridChromeHeight         = 4
)

// panelSet builds the panels for the selected repo at the given content width.
// A jj repo has no Stashes panel at all, rather than one explaining its own
// absence, and the same reasoning drops any panel that finished loading with
// nothing to list. Everything stays on screen while the load is in flight, so
// the grid settles once instead of reflowing under the cursor as data lands.
func (m Model) panelSet(width int) []panelContent {
	summary := m.summaries[m.selectedRepo]

	built := []panelContent{
		statusPanel(summary, width),
		m.branchesPanel(width),
		m.peersPanel(summary, width),
	}

	if summary.VCSType != vcs.TypeJJ {
		built = append(built, m.stashesPanel(width))
	}

	built = append(built, m.notesPanel(width))

	panels := make([]panelContent, 0, len(built))
	for _, p := range built {
		if len(p.rows) == 0 && !m.detailLoading && !panelAlwaysShown(p.id) {
			continue
		}

		p.key = panelKeys[p.id]
		if len(p.rows) == 0 {
			p.rows = []string{styles.SubtitleStyle.Render(padCell(m.panelEmptyLabel(), width))}
		}

		panels = append(panels, p)
	}

	return panels
}

// panelEmptyLabel is what a panel with no rows says: still-loading while the
// detail load is in flight, and the plain empty marker for the panels that are
// kept on screen regardless.
func (m Model) panelEmptyLabel() string {
	if m.detailLoading {
		return "loading…"
	}

	return overviewEmpty
}

// panelForKey returns the panel a jump key selects.
func panelForKey(panels []panelContent, key string) (panelContent, bool) {
	for _, p := range panels {
		if p.key == key {
			return p, true
		}
	}

	return panelContent{}, false
}

// panelIndex returns the position of id in panels, or zero when the panel is
// absent (a jj repo has no Stashes panel to focus).
func panelIndex(panels []panelContent, id panelID) int {
	for i, p := range panels {
		if p.id == id {
			return i
		}
	}

	return 0
}

// statusPanel is the grid's collapsed view of the repo: sync and working-tree
// state, the two facts worth a glance without opening it. Everything else
// (identity, template, CI, config drift, absent panels) lives in the full
// preview repoDetailLines renders once Status is focused.
func statusPanel(summary models.RepoSummary, width int) panelContent {
	files := overviewFiles(summary)
	if extras := statusExtras(summary); extras != "" {
		files += compactSignalSep + extras
	}

	lines := []string{
		overviewSync(summary),
		files,
	}

	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, styles.TableRowStyle.Render(padCell(line, width)))
	}

	relevance := relevanceIdle
	if summary.UncommittedCount() > 0 || summary.Ahead > 0 {
		relevance = relevanceUrgent
	}

	return panelContent{id: panelStatus, title: "Status", relevance: relevance, rows: rows}
}

// statusExtras summarizes the counts that do not deserve their own line.
func statusExtras(summary models.RepoSummary) string {
	parts := []string{}
	if summary.StashCount > 0 {
		parts = append(parts, strconv.Itoa(summary.StashCount)+" "+plural(summary.StashCount, "stash", "stashes"))
	}
	if notes := len(summary.NotesFiles); notes > 0 {
		parts = append(parts, strconv.Itoa(notes)+" "+plural(notes, "note", "notes"))
	}

	return strings.Join(parts, compactSignalSep)
}

// statusAbsences names the panels the grid dropped for having nothing to list,
// so an absence is still reported once rather than left to be inferred from a
// box that is not there.
func (m Model) statusAbsences(summary models.RepoSummary) string {
	if m.detailLoading {
		return ""
	}

	absent := make([]string, 0, len(panelKeys))
	if len(m.prs) == 0 {
		absent = append(absent, "PRs")
	}
	if len(m.RepoCheckouts()) == 0 {
		absent = append(absent, "peers")
	}
	if summary.VCSType != vcs.TypeJJ && len(m.stashes) == 0 {
		absent = append(absent, "stashes")
	}
	if len(m.notesFiles) == 0 {
		absent = append(absent, "notes")
	}

	if len(absent) == 0 {
		return ""
	}

	return "no " + joinOr(absent)
}

// joinOr renders a list as prose with an Oxford comma: "a", "a or b", "a, b, or c".
func joinOr(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}

	head, last := parts[:len(parts)-1], parts[len(parts)-1]
	if len(head) == 1 {
		return head[0] + " or " + last
	}

	return strings.Join(head, ", ") + ", or " + last
}

func (m Model) branchesPanel(width int) panelContent {
	layout := fitPanelCols(branchPanelSpecs, width)
	prsByBranch := prsByHeadRef(m.prs)
	checkouts := m.RepoCheckouts()

	rows := make([]string, 0, len(m.branches))
	unpushed := 0
	for i, branch := range m.branches {
		row := branchRow{
			branch:    branch,
			pr:        prsByBranch[branch.Name],
			deletable: m.deletableBranches[branch.Name],
			selected:  m.panelSelected(panelBranches, i),
		}
		if checkout, ok := models.CheckoutForBranch(checkouts, branch.Name); ok {
			row.checkout = checkout.Folder()
		}
		if branch.Ahead > 0 {
			unpushed++
		}

		rows = append(rows, renderBranchRow(row, layout))
	}

	relevance := relevanceIdle
	if len(m.branches) > 0 {
		relevance = relevancePresent
	}
	if unpushed > 0 {
		relevance = relevanceUrgent
	}

	return panelContent{
		id: panelBranches, title: tabNameBranches, count: len(m.branches),
		relevance: relevance, rows: rows, selectable: true,
	}
}

func (m Model) peersPanel(summary models.RepoSummary, width int) panelContent {
	checkouts := m.RepoCheckouts()
	conflicts := models.ConflictingBranches(ownCheckoutOf(&summary), checkouts)
	layout := fitPanelCols(peerPanelSpecs, width)

	rows := make([]string, 0, len(checkouts))
	conflicted := 0
	for i, checkout := range checkouts {
		selected := m.panelSelected(panelPeers, i)
		style := rowStyleFor(selected)

		branch := checkout.Branch
		branchStyle := styles.SubtitleStyle
		if !vcs.IsDefaultBranchName(checkout.Branch) {
			branchStyle = styles.BranchStyle
		}
		if conflicts[branch] {
			branch += " " + conflictMark
			branchStyle = styles.WarningStyle
			conflicted++
		}

		values := map[string]string{
			colCheckoutName:   checkout.Folder(),
			colCheckoutKind:   checkout.Kind(),
			colCheckoutBranch: branch,
		}
		cellStyles := map[string]lipgloss.Style{
			colCheckoutBranch: withSelection(branchStyle, selected),
		}

		rows = append(rows, detailRow(rowCursorFor(selected), layout, renderCells(layout, values, cellStyles, &style)))
	}

	relevance := relevanceIdle
	if len(checkouts) > 0 {
		relevance = relevancePresent
	}
	if conflicted > 0 {
		relevance = relevanceUrgent
	}

	return panelContent{
		id: panelPeers, title: tabNamePeers, count: len(checkouts),
		relevance: relevance, rows: rows, selectable: true,
	}
}

func (m Model) stashesPanel(width int) panelContent {
	layout := fitPanelCols(stashPanelSpecs, width)

	rows := make([]string, 0, len(m.stashes))
	for i, stash := range m.stashes {
		selected := m.panelSelected(panelStashes, i)
		style := rowStyleFor(selected)

		values := map[string]string{
			colStashMessage: stash.Message,
			colStashDate:    ui.StashRelativeDate(stash),
		}

		rows = append(rows, detailRow(rowCursorFor(selected), layout, renderCells(layout, values, nil, &style)))
	}

	relevance := relevanceIdle
	if len(m.stashes) > 0 {
		relevance = relevancePresent
	}

	return panelContent{
		id: panelStashes, title: tabNameStashes, count: len(m.stashes),
		relevance: relevance, rows: rows, selectable: true,
	}
}

func (m Model) notesPanel(width int) panelContent {
	shown, hasBody := m.shownNote()

	rows := make([]string, 0, len(m.notesFiles))
	for i, note := range m.notesFiles {
		selected := m.panelSelected(panelNotes, i)
		style := rowStyleFor(selected)

		// The first line is worth a row only where the body below does not
		// already carry it.
		label := note.Name
		if first := firstContentLine(note.Content); first != "" && note.Name != shown.Name {
			label += "  " + first
		}

		rows = append(rows, rowCursorFor(selected)+style.Render(
			table.TruncateMiddle(padCell(label, width-cursorWidth), width-cursorWidth)))
	}

	relevance := relevanceIdle
	if hasBody {
		// A note is something the last session left for this one, so it
		// outranks a list the repo itself can always answer.
		relevance = relevanceUrgent
		rows = append(rows, noteBodyRows(shown.Content, width)...)
	}

	return panelContent{
		id: panelNotes, title: tabNameNotes, count: len(m.notesFiles),
		relevance: relevance, rows: rows, selectable: true,
	}
}

// shownNote is the note whose text the panel spells out: the one under the
// cursor while Notes holds it, and the first otherwise, so a repo's note is
// readable from whichever panel the view opened on.
func (m Model) shownNote() (models.NoteFileContent, bool) {
	if len(m.notesFiles) == 0 {
		return models.NoteFileContent{}, false
	}

	if note, ok := m.selectedPanelNote(); ok {
		return note, true
	}

	return m.notesFiles[0], true
}

// notesPreviewMaxLines caps how many lines of a note the side panel shows, so
// an especially long note can't crowd out the panels beside it the way an
// unbounded panelWant would.
const notesPreviewMaxLines = 8

// noteBodyRows are a note's text, rendered below the file list so the panel's
// earned height carries the note itself rather than blank rows. They sit past
// the selectable rows, so the cursor never lands on one. Rendering (including
// the bang-line highlight and the elided preview) reuses the Repos list's own
// note-preview logic, so a note reads the same way in both places.
func noteBodyRows(content string, width int) []string {
	lines := elideMiddle(notesBodyLines(content, width), notesPreviewMaxLines)

	rows := make([]string, 0, len(lines)+1)
	rows = append(rows, "")

	return append(rows, lines...)
}

// firstContentLine returns a note's first line that carries content, skipping
// markdown headings and blanks: the peek is worth a line only if it says
// something the filename does not.
func firstContentLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		return line
	}

	return ""
}

// panelSelected reports whether row i of panel id is under the cursor, which
// only the focused panel can be.
func (m Model) panelSelected(id panelID, i int) bool {
	return m.focusedPanel == id && m.detailCursor == i
}

// renderPanelGrid draws the focused repo view: a column of always-visible
// panels beside a detail pane for the selected item. Below the compact
// breakpoint the two stack, with the detail pane under the focused panel.
func (m Model) renderPanelGrid() string {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return "Repository not found"
	}

	width := m.gridWidth()
	stacked := gridStacked(m.isCompact(), width)

	sideWidth := width
	if !stacked {
		sideWidth = panelSideWidth(width, m.detailFocused)
	}

	panels := m.panelSet(sideWidth - panelBorderWidth)
	focused := panelIndex(panels, m.focusedPanel)

	var b strings.Builder
	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render(truncate(summary.Path, width)))
	b.WriteString("\n\n")

	if m.panelActions {
		return m.renderActionModal()
	}

	body := m.height - gridChromeHeight
	if stacked {
		b.WriteString(m.renderStackedGrid(panels, focused, sideWidth, body))
	} else {
		b.WriteString(m.renderSplitGrid(panels, focused, sideWidth, width-sideWidth, body))
	}

	b.WriteString("\n")
	b.WriteString(styles.FooterStyle.Render(m.panelFooter(panels, focused, width)))

	return b.String()
}

// gridWidth is the width the focused repo view renders into.
func (m Model) gridWidth() int {
	return wideContentWidth(m.width)
}

// gridStacked reports whether the grid drops its side-by-side split, either
// because the breakpoint says so or because the detail pane would be too
// narrow to read beside the panel column. It measures against the widest the
// column ever gets, so moving focus never flips the grid between layouts.
func gridStacked(compact bool, width int) bool {
	return compact || width-panelSideWidth(width, false) < minDetailPaneWidth
}

// panelSideWidth is the width the panel column takes from a split grid. The
// column gives up more of it while the detail pane holds focus, because
// nothing is being selected in it then.
func panelSideWidth(width int, detailFocused bool) int {
	percent := sideColumnPercent
	if detailFocused {
		percent = sideColumnFocusedPercent
	}

	return max(width*percent/sideColumnFraction, minSideColumnWidth)
}

// detailPaneSize returns the detail pane's interior at the current terminal
// size, which is the geometry a scroll offset has to be clamped against.
func (m Model) detailPaneSize() paneSize {
	grid := m.gridWidth()
	body := m.height - gridChromeHeight

	if !gridStacked(m.isCompact(), grid) {
		return paneSize{
			width:  grid - panelSideWidth(grid, m.detailFocused) - panelBorderWidth,
			height: panelRowsHeight(body),
		}
	}

	panels := m.panelSet(grid - panelBorderWidth)
	compressed := panelMinHeight

	return paneSize{
		width:  grid - panelBorderWidth,
		height: max(panelRowsHeight(body-compressed*len(panels)), minStackedDetailHeight),
	}
}

type paneSize struct {
	width  int
	height int
}

// renderSplitGrid stacks the panels on the left and mounts the detail pane
// beside them at the full body height.
func (m Model) renderSplitGrid(panels []panelContent, focused, sideWidth, detailWidth, height int) string {
	column := m.renderPanelColumn(panels, focused, sideWidth, height)
	detail := m.renderPanelDetail(detailWidth-panelBorderWidth, panelRowsHeight(height))

	return lipgloss.JoinHorizontal(lipgloss.Top,
		column,
		panelBox(m.focusedPanelTitle(panels, focused), detail, detailWidth, height,
			focusBorder(m.detailFocused, true)),
	)
}

// minStackedDetailHeight keeps the stacked detail pane readable even when the
// panels above it have eaten the terminal.
const minStackedDetailHeight = 3

// renderStackedGrid drops the side-by-side split: unfocused panels compress to
// their title plus a content line, and the detail pane follows the focused one.
// The stack scrolls when it outgrows the terminal, so the focused panel and its
// detail stay on screen.
func (m Model) renderStackedGrid(panels []panelContent, focused, width, height int) string {
	compressed := panelMinHeight
	detailHeight := max(panelRowsHeight(height-compressed*len(panels)), minStackedDetailHeight)

	var lines []string
	focusStart, focusEnd := 0, 0

	for i, p := range panels {
		if i == focused {
			focusStart = len(lines)
		}

		rows := panelRows(p, panelRowsHeight(compressed), m.detailCursor, i == focused)
		box := panelBox(panelTitle(&p, i == focused), rows, width, compressed,
			focusBorder(!m.detailFocused, i == focused))
		lines = append(lines, strings.Split(box, "\n")...)

		if i == focused {
			detail := panelBox(m.focusedPanelTitle(panels, focused),
				m.renderPanelDetail(width-panelBorderWidth, detailHeight), width,
				detailHeight+panelChromeHeight+panelTitleHeight,
				focusBorder(m.detailFocused, true))
			lines = append(lines, strings.Split(detail, "\n")...)
			focusEnd = len(lines)
		}
	}

	return strings.Join(windowLines(lines, focusStart, focusEnd, height), "\n")
}

// windowLines trims lines to height, scrolling so the [start, end) block stays
// visible rather than letting the stack run off the bottom.
func windowLines(lines []string, start, end, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}

	top := min(start, len(lines)-height)
	if end > top+height {
		top = max(end-height, 0)
	}

	return lines[top : top+height]
}

// renderPanelColumn draws every panel at the height its relevance earned.
func (m Model) renderPanelColumn(panels []panelContent, focused, width, height int) string {
	heights := distributePanelHeights(panels, focused, height)

	boxes := make([]string, 0, len(panels))
	for i, p := range panels {
		boxes = append(boxes, panelBox(panelTitle(&p, i == focused),
			panelRows(p, panelRowsHeight(heights[i]), m.detailCursor, i == focused),
			width, heights[i], focusBorder(!m.detailFocused, i == focused)))
	}

	return strings.Join(boxes, "\n")
}

// panelRows windows a panel's rows to the lines it was given, keeping the
// cursor in view.
func panelRows(p panelContent, lines, cursor int, focused bool) string {
	if lines <= 0 {
		return ""
	}

	window := visibleRange(cursor, len(p.rows), lines)
	if !focused {
		window = repoWindow{start: 0, end: min(lines, len(p.rows))}
	}

	shown := p.rows[window.start:window.end]
	if len(shown) < lines {
		shown = append(append([]string{}, shown...), make([]string, lines-len(shown))...)
	}

	return strings.Join(shown, "\n")
}

// focusBorder colors a box's border. Blue marks where the keyboard is; the
// muted accent marks the box that still holds the selection while focus sits
// in the other region, so the two are never confused for one another.
func focusBorder(active, current bool) color.Color {
	switch {
	case active && current:
		return styles.Blue
	case current:
		return styles.Overlay1
	default:
		return styles.Surface2
	}
}

// panelBox wraps content in a titled border.
func panelBox(title, content string, width, height int, border color.Color) string {
	// lipgloss sizes the bordered block as a whole, so width and height here
	// are the box's outer dimensions, not its interior.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(width).
		Height(max(height, panelMinHeight)).
		Render(title + "\n" + content)
}

// focusedPanelTitle labels the detail pane with the item it is describing.
func (m Model) focusedPanelTitle(panels []panelContent, focused int) string {
	title := "Detail"
	if focused >= 0 && focused < len(panels) {
		title = m.panelDetailTitle(panels[focused])
	}

	marker := m.detailScrollMarker()
	title = truncate(title, max(m.detailPaneSize().width-lipgloss.Width(marker), 1))

	return styles.SubtitleStyle.Render(title) + marker
}

// detailScrollMarker reports how much of the pane's text is on screen, so text
// running past the bottom is never mistaken for the whole of it.
func (m Model) detailScrollMarker() string {
	pane := m.detailPaneSize()

	total := len(m.panelDetailLines(pane.width))
	if total <= pane.height {
		return ""
	}

	shown := min(m.detailScroll+pane.height, total)

	return styles.SubtitleStyle.Render("  " + strconv.Itoa(shown) + "/" + strconv.Itoa(total))
}

func (m Model) panelDetailTitle(p panelContent) string {
	switch p.id {
	case panelBranches:
		if branch, ok := m.selectedPanelBranch(); ok {
			return branch.Name
		}
	case panelPeers:
		if checkout, ok := m.selectedPanelCheckout(); ok {
			return checkout.Folder()
		}
	case panelStashes:
		if stash, ok := m.selectedPanelStash(); ok {
			return "stash@{" + strconv.Itoa(stash.Index) + "}"
		}
	case panelNotes:
		if note, ok := m.selectedPanelNote(); ok {
			return note.Name
		}
	case panelStatus:
		return m.summaries[m.selectedRepo].Name()
	}

	return p.title
}

// panelDetailLines renders the selected item of the focused panel in full. It
// is never empty: with nothing selected it falls back to the repo's own
// detail, so the pane always earns its width.
func (m Model) panelDetailLines(width int) []string {
	var lines []string

	switch m.focusedPanel {
	case panelBranches:
		lines = m.branchDetailLines(width)
	case panelPeers:
		lines = m.peerDetailLines(width)
	case panelStashes:
		lines = m.stashDetailLines(width)
	case panelNotes:
		lines = m.noteDetailLines(width)
	case panelStatus:
		lines = m.repoDetailLines(width)
	}

	if len(lines) == 0 {
		lines = m.repoDetailLines(width)
	}

	return lines
}

// renderPanelDetail windows the selected item's detail to the pane, honoring
// the scroll offset that focusing the pane makes reachable.
func (m Model) renderPanelDetail(width, height int) string {
	lines := m.panelDetailLines(width)

	start := min(m.detailScroll, max(len(lines)-height, 0))
	lines = lines[start:min(start+height, len(lines))]

	return strings.Join(lines, "\n")
}

// repoDetailFieldCount is how many fixed fields repoDetailLines writes before
// any config overrides.
const repoDetailFieldCount = 12

// repoDetailLines is Status's full preview, opened by focusing the panel: the
// repo's identity facts that once lived as breadcrumb badges (vcs, protocol,
// detached/dirty state, the branch's pull request, parallel checkouts),
// alongside the fields the collapsed panel already carried and the config
// drift and absent-panel notes that have nowhere else to show.
func (m Model) repoDetailLines(width int) []string {
	summary := m.summaries[m.selectedRepo]

	branch := summary.Branch
	if summary.IsDetached() {
		branch = "detached at " + summary.Branch
	}

	remote := orDash(summary.RemoteRepo)
	if summary.RemoteProtocol != "" {
		remote += " (" + summary.RemoteProtocol + ")"
	}

	lines := make([]string, 0, len(summary.ConfigOverrides)+repoDetailFieldCount)
	lines = append(lines,
		detailField("path", truncate(summary.Path, width)),
		detailField("vcs", summary.VCSType.String()),
		detailField("branch", branch),
		detailField("upstream", orDash(summary.Upstream)),
		detailField("remote", remote),
		detailField("files", overviewFiles(summary)),
		detailField("template", formatCopierCell(summary, width)),
		detailField("ci", m.overviewCI(summary)),
	)

	if label := summary.DirtyLabel(); label != "" {
		lines = append(lines, detailField("dirty", label))
	}

	if m.newestFile != "" {
		lines = append(lines, detailField("newest edit", m.newestFile+" "+ui.RelativeTime(m.newestFileTime)))
	}

	if defaultBranch := findDefaultBranch(m.branches); defaultBranch != "" && defaultBranch != summary.Branch {
		lines = append(lines, detailField("default branch", defaultBranch))
	}

	if summary.PRInfo != nil {
		lines = append(lines, detailField("pull request", formatPRCell(summary)+" "+summary.PRInfo.Title))
	}

	if checkouts := m.RepoCheckouts(); len(checkouts) > 0 {
		label := "⧉ " + strconv.Itoa(len(checkouts)) + " parallel checkouts"
		if len(checkouts) == 1 {
			label = "⧉ " + checkouts[0].Folder()
		}

		lines = append(lines, detailField("checkouts", label))
	}

	if absent := m.statusAbsences(summary); absent != "" {
		lines = append(lines, detailField("absent", absent))
	}

	for _, override := range summary.ConfigOverrides {
		lines = append(lines, detailField(override.Key, override.LocalValue+" ≠ "+override.GlobalValue))
	}

	if summary.UncommittedCount() > 0 {
		lines = append(lines, m.workingDiffLines(summary.Path, width)...)
	}

	return lines
}

func (m Model) branchDetailLines(width int) []string {
	branch, ok := m.selectedPanelBranch()
	if !ok {
		return nil
	}

	lines := []string{
		detailField("upstream", orDash(branch.Upstream)),
		detailField("tracking", branchAheadBehindStatus(branch)),
		detailField("last commit", ui.BranchRelativeLastCommit(branch)),
	}

	if pr := prsByHeadRef(m.prs)[branch.Name]; pr != nil {
		lines = append(lines, detailField("pull request", formatBranchPRCell(pr)+" "+pr.Title))
	}

	if m.branchDetail.Branch.Name != branch.Name {
		return append(lines, "", styles.SubtitleStyle.Render("loading commits…"))
	}

	lines = append(lines, "", styles.HeaderStyle.Render("recent commits"))
	for _, commit := range m.branchDetail.Commits {
		lines = append(lines, styles.TableRowStyle.Render(
			truncate("  "+commit.ShortHash+" "+commit.Subject, width)))
	}

	return lines
}

func (m Model) peerDetailLines(width int) []string {
	checkout, ok := m.selectedPanelCheckout()
	if !ok {
		return nil
	}

	lines := []string{
		detailField("path", checkout.Path),
		detailField("kind", checkout.Kind()),
		detailField("branch", orDash(checkout.Branch)),
		detailField("state", checkoutState(checkout)),
		detailField("last commit", relativeOrDash(checkout.LastCommit)),
	}

	if checkout.Dirty {
		lines = append(lines, m.workingDiffLines(checkout.Path, width)...)
	}

	return lines
}

// workingDiffLines renders the cached diff or diffstat of repoPath's working
// tree, or a loading placeholder if the fetch hasn't landed yet. Shared by the
// Status and Peers detail panes, which each describe a different repo path
// but need the same diff/diffstat toggle and truncation.
func (m Model) workingDiffLines(repoPath string, width int) []string {
	label, texts := "diffstat", m.uncommittedDiffstat
	if m.uncommittedFullDiff {
		label, texts = "diff", m.uncommittedDiff
	}

	body, loaded := texts[repoPath]
	if !loaded {
		return []string{"", styles.SubtitleStyle.Render("loading " + label + "…")}
	}

	lines := []string{"", styles.HeaderStyle.Render(label)}
	for _, line := range diffBodyLines(body) {
		lines = append(lines, diffBodyLine(line, width))
	}

	return lines
}

func (m Model) stashDetailLines(width int) []string {
	stash, ok := m.selectedPanelStash()
	if !ok {
		return nil
	}

	lines := []string{
		detailField("message", stash.Message),
		detailField("branch", orDash(stash.Branch)),
		detailField("created", ui.StashRelativeDate(stash)),
	}

	label, texts := "diffstat", m.stashDiffstat
	if m.stashFullDiff {
		label, texts = "diff", m.stashDiff
	}

	body, loaded := texts[stash.Index]
	if !loaded {
		return append(lines, "", styles.SubtitleStyle.Render("loading "+label+"…"))
	}

	lines = append(lines, "", styles.HeaderStyle.Render(label))
	for _, line := range diffBodyLines(body) {
		lines = append(lines, diffBodyLine(line, width))
	}

	return lines
}

// diffBodyIndent is what a patch line is inset by inside the detail pane.
const diffBodyIndent = 2

// diffBodyLine renders one line of a patch. A line an external viewer colored
// is only clipped, never restyled: the viewer's first reset would end an outer
// style anyway, leaving the rest of the pane's line unstyled.
func diffBodyLine(line string, width int) string {
	indented := strings.Repeat(" ", diffBodyIndent) + line
	if strings.ContainsRune(line, '\x1b') {
		return lipgloss.NewStyle().MaxWidth(max(width, 1)).Render(indented)
	}

	return styles.TableRowStyle.Render(truncate(indented, width))
}

// diffBodyLines caps the pane's line count, since the whole detail is
// re-rendered on every frame and a patch has no upper bound.
func diffBodyLines(body string) []string {
	lines := strings.Split(body, "\n")
	if len(lines) <= diffMaxLines {
		return lines
	}

	return append(lines[:diffMaxLines:diffMaxLines],
		"… "+strconv.Itoa(len(lines)-diffMaxLines)+" more lines")
}

func (m Model) noteDetailLines(width int) []string {
	note, ok := m.selectedPanelNote()
	if !ok {
		return nil
	}

	content := note.Content
	if strings.TrimSpace(content) == "" {
		return []string{styles.SubtitleStyle.Render("(empty file)")}
	}

	return wrapLines(content, width)
}

// Selected-item accessors. Each answers only for its own panel, so a cursor
// left over from another panel can never index the wrong list.
func (m Model) selectedPanelBranch() (vcs.BranchInfo, bool) {
	if m.focusedPanel != panelBranches || m.detailCursor >= len(m.branches) {
		return vcs.BranchInfo{}, false
	}

	return m.branches[m.detailCursor], true
}

func (m Model) selectedPanelCheckout() (models.PeerCheckout, bool) {
	checkouts := m.RepoCheckouts()
	if m.focusedPanel != panelPeers || m.detailCursor >= len(checkouts) {
		return models.PeerCheckout{}, false
	}

	return checkouts[m.detailCursor], true
}

func (m Model) selectedPanelStash() (vcs.StashDetail, bool) {
	if m.focusedPanel != panelStashes || m.detailCursor >= len(m.stashes) {
		return vcs.StashDetail{}, false
	}

	return m.stashes[m.detailCursor], true
}

func (m Model) selectedPanelNote() (models.NoteFileContent, bool) {
	if m.focusedPanel != panelNotes || m.detailCursor >= len(m.notesFiles) {
		return models.NoteFileContent{}, false
	}

	return m.notesFiles[m.detailCursor], true
}

// panelHintPriority is the top of the focused view's footer collapse order:
// navigation survives longest, actions drop first.
const (
	panelHintPriority = 9
	navHintStep       = 1
)

// panelFooter names the keys the focused panel supports, so what is on offer
// always matches what is selected and where the keyboard currently is. The
// panel jump keys are absent because each panel's border already carries its
// own.
func (m Model) panelFooter(panels []panelContent, focused, width int) string {
	navDesc, backDesc := descNav, descBack
	if m.detailFocused {
		navDesc, backDesc = "scroll", "panels"
	}

	hints := []footerHint{
		{key: keyNavPair, desc: navDesc, priority: panelHintPriority - navHintStep},
		{key: keyEsc, desc: backDesc, priority: panelHintPriority - navHintStep*3},
		{key: "space", desc: nameFind, priority: panelHintPriority - navHintStep*2},
	}

	if !m.detailFocused {
		hints = append(hints, footerHint{
			key: keyEnter, desc: descDetail, priority: panelHintPriority - navHintStep*2,
		})
	}

	if focused >= 0 && focused < len(panels) && len(panelActionsFor(panels[focused].id)) > 0 {
		hints = append(hints, footerHint{
			key: actionLeader, desc: nameActions, priority: panelHintPriority - navHintStep,
		})
	}

	return renderHints(fittingHints(hints, width))
}

func detailField(label, value string) string {
	return styles.SubtitleStyle.Render(label+" ") + styles.TableRowStyle.Render(value)
}

func orDash(value string) string {
	if value == "" {
		return emDash
	}

	return value
}

// wrapLines hard-wraps text to width and returns it as styled lines.
func wrapLines(text string, width int) []string {
	wrapped := lipgloss.Wrap(strings.TrimSpace(text), max(width, 1), "")

	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		lines[i] = styles.TableRowStyle.Render(line)
	}

	return lines
}
