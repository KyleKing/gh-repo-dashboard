package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// Grid geometry. The side column holds the panels; the rest is the detail
// pane, which always renders the selected item rather than waiting for a key.
const (
	panelBorderWidth   = 2
	sideColumnFraction = 100
	sideColumnPercent  = 42
	minSideColumnWidth = 34
	minDetailPaneWidth = 30
	gridChromeHeight   = 4
)

// panelSet builds every panel for the selected repo at the given content
// width. A jj repo has no Stashes panel at all, rather than one explaining its
// own absence.
func (m Model) panelSet(width int) []panelContent {
	summary := m.summaries[m.selectedRepo]

	panels := []panelContent{
		m.statusPanel(summary, width),
		m.branchesPanel(width),
		m.prsPanel(width),
		m.peersPanel(summary, width),
	}

	if summary.VCSType != models.VCSTypeJJ {
		panels = append(panels, m.stashesPanel(width))
	}

	panels = append(panels, m.notesPanel(width))

	empty := overviewEmpty
	if m.detailLoading {
		empty = "loading…"
	}

	for i := range panels {
		panels[i].key = strconv.Itoa(i + 1)
		if len(panels[i].rows) == 0 {
			panels[i].rows = []string{styles.SubtitleStyle.Render(padCell(empty, width))}
		}
	}

	return panels
}

// panelForKey returns the panel a number key selects.
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

func (m Model) statusPanel(summary models.RepoSummary, width int) panelContent {
	lines := []string{
		overviewSync(summary),
		overviewFiles(summary) + compactSignalSep + statusExtras(summary),
		"template " + formatCopierCell(summary, width) + compactSignalSep + "CI " + m.overviewCI(summary),
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
	if len(parts) == 0 {
		return "no stashes or notes"
	}

	return strings.Join(parts, compactSignalSep)
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

func (m Model) prsPanel(width int) panelContent {
	layout := fitPanelCols(prPanelSpecs, width)

	rows := make([]string, 0, len(m.prs))
	for i := range m.prs {
		pr := &m.prs[i]
		selected := m.panelSelected(panelPRs, i)
		style := rowStyleFor(selected)

		values := map[string]string{
			colPRNumber:   "#" + strconv.Itoa(pr.Number),
			colPRTitle:    pr.Title,
			colPRState:    prStateCell(pr),
			colChecks:     formatChecksCell(pr),
			colPRActivity: pr.ActivitySummary(),
		}
		cellStyles := map[string]lipgloss.Style{
			colPRState:    withSelection(prStateStyle(pr), selected),
			colChecks:     withSelection(checksCellStyle(pr, style), selected),
			colPRActivity: withSelection(styles.SubtitleStyle, selected),
		}

		rows = append(rows, detailRow(rowCursorFor(selected), layout, renderCells(layout, values, cellStyles, &style)))
	}

	relevance := relevanceIdle
	if len(m.prs) > 0 {
		relevance = relevanceUrgent
	}

	return panelContent{
		id: panelPRs, title: tabNamePRs, count: len(m.prs),
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
		branchStyle := styles.BranchStyle
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
		id: panelPeers, title: "Peers", count: len(checkouts),
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
			colStashDate:    stash.RelativeDate(),
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
	rows := make([]string, 0, len(m.notesFiles))
	for i, note := range m.notesFiles {
		selected := m.panelSelected(panelNotes, i)
		style := rowStyleFor(selected)

		label := note.Name
		if first := firstContentLine(note.Content); first != "" {
			label += "  " + first
		}

		rows = append(rows, rowCursorFor(selected)+style.Render(padCell(label, width-cursorWidth)))
	}

	relevance := relevanceIdle
	if len(m.notesFiles) > 0 {
		relevance = relevancePresent
	}

	return panelContent{
		id: panelNotes, title: tabNameNotes, count: len(m.notesFiles),
		relevance: relevance, rows: rows, selectable: true,
	}
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

	width := contentWidth(m.width)
	stacked := m.isCompact() || width-panelSideWidth(width) < minDetailPaneWidth

	sideWidth := width
	if !stacked {
		sideWidth = panelSideWidth(width)
	}

	panels := m.panelSet(sideWidth - panelBorderWidth)
	focused := panelIndex(panels, m.focusedPanel)

	var b strings.Builder
	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render(truncate(summary.Path, width)))
	b.WriteString("\n\n")

	body := m.height - gridChromeHeight
	if stacked {
		b.WriteString(m.renderStackedGrid(panels, focused, sideWidth, body))
	} else {
		b.WriteString(m.renderSplitGrid(panels, focused, sideWidth, width-sideWidth, body))
	}

	b.WriteString("\n")
	b.WriteString(styles.FooterStyle.Render(panelFooter(panels, focused, width)))

	return b.String()
}

// panelSideWidth is the width the panel column takes from a split grid.
func panelSideWidth(width int) int {
	return max(width*sideColumnPercent/sideColumnFraction, minSideColumnWidth)
}

// renderSplitGrid stacks the panels on the left and mounts the detail pane
// beside them at the full body height.
func (m Model) renderSplitGrid(panels []panelContent, focused, sideWidth, detailWidth, height int) string {
	column := m.renderPanelColumn(panels, focused, sideWidth, height)
	detail := m.renderPanelDetail(detailWidth-panelBorderWidth, height-panelChromeHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		column,
		panelBox(m.focusedPanelTitle(panels, focused), detail, detailWidth, height, true),
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
	compressed := panelChromeHeight + 1
	detailHeight := max(height-compressed*len(panels)-panelChromeHeight, minStackedDetailHeight)

	var lines []string
	focusStart, focusEnd := 0, 0

	for i, p := range panels {
		if i == focused {
			focusStart = len(lines)
		}

		rows := panelRows(p, compressed-panelChromeHeight, m.detailCursor, i == focused)
		box := panelBox(panelTitle(&p, i == focused), rows, width, compressed, i == focused)
		lines = append(lines, strings.Split(box, "\n")...)

		if i == focused {
			detail := panelBox(m.focusedPanelTitle(panels, focused),
				m.renderPanelDetail(width-panelBorderWidth, detailHeight), width, detailHeight+panelChromeHeight, true)
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
			panelRows(p, heights[i]-panelChromeHeight, m.detailCursor, i == focused),
			width, heights[i], i == focused))
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

// panelBox wraps content in a titled border, blue when focused so the active
// panel is unmistakable without a mode indicator.
func panelBox(title, content string, width, height int, focused bool) string {
	border := styles.Surface2
	if focused {
		border = styles.Blue
	}

	// lipgloss sizes the bordered block as a whole, so width and height here
	// are the box's outer dimensions, not its interior.
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(width).
		Height(max(height, panelChromeHeight+1)).
		Render(title + "\n" + content)
}

// focusedPanelTitle labels the detail pane with the item it is describing.
func (m Model) focusedPanelTitle(panels []panelContent, focused int) string {
	if focused < 0 || focused >= len(panels) {
		return styles.SubtitleStyle.Render("Detail")
	}

	return styles.SubtitleStyle.Render(m.panelDetailTitle(panels[focused]))
}

func (m Model) panelDetailTitle(p panelContent) string {
	switch p.id {
	case panelBranches:
		if branch, ok := m.selectedPanelBranch(); ok {
			return branch.Name
		}
	case panelPRs:
		if pr, ok := m.selectedPanelPR(); ok {
			return "#" + strconv.Itoa(pr.Number) + " " + pr.Title
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

// renderPanelDetail renders the selected item of the focused panel. It is
// never empty: with nothing selected it falls back to the repo's own detail,
// so the pane always earns its width.
func (m Model) renderPanelDetail(width, height int) string {
	var lines []string

	switch m.focusedPanel {
	case panelBranches:
		lines = m.branchDetailLines(width)
	case panelPRs:
		lines = m.prDetailLines(width)
	case panelPeers:
		lines = m.peerDetailLines()
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

	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// repoDetailFieldCount is how many fixed fields repoDetailLines writes before
// any config overrides.
const repoDetailFieldCount = 7

func (m Model) repoDetailLines(width int) []string {
	summary := m.summaries[m.selectedRepo]

	lines := make([]string, 0, len(summary.ConfigOverrides)+repoDetailFieldCount)
	lines = append(lines,
		detailField("path", truncate(summary.Path, width)),
		detailField("vcs", summary.VCSType.String()),
		detailField("branch", summary.Branch),
		detailField("upstream", orDash(summary.Upstream)),
		detailField("remote", orDash(summary.RemoteRepo)),
		detailField("files", overviewFiles(summary)),
		detailField("ci", m.overviewCI(summary)),
	)

	for _, override := range summary.ConfigOverrides {
		lines = append(lines, detailField(override.Key, override.LocalValue+" ≠ "+override.GlobalValue))
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
		detailField("last commit", branch.RelativeLastCommit()),
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

func (m Model) prDetailLines(width int) []string {
	pr, ok := m.selectedPanelPR()
	if !ok {
		return nil
	}

	lines := []string{
		detailField("state", pr.StatusDisplay()),
		detailField("head", pr.HeadRef+" → "+pr.BaseRef),
		detailField("checks", formatChecksCell(&pr)),
		detailField("review", pr.ReviewStatus()),
		detailField("activity", pr.ActivitySummary()),
	}

	if m.prDetail.Number != pr.Number {
		return append(lines, "", styles.SubtitleStyle.Render("loading description…"))
	}

	if body := strings.TrimSpace(m.prDetail.Body); body != "" {
		lines = append(lines, "", styles.HeaderStyle.Render("description"))
		lines = append(lines, wrapLines(truncateWords(body, prBodyMaxLen), width)...)
	}

	if comment := m.prDetail.LatestComment; comment != nil {
		lines = append(lines, "", styles.HeaderStyle.Render("latest comment · "+comment.Author))
		lines = append(lines, wrapLines(truncateWords(strings.TrimSpace(comment.Body), prCommentMaxLen), width)...)
	}

	return lines
}

func (m Model) peerDetailLines() []string {
	checkout, ok := m.selectedPanelCheckout()
	if !ok {
		return nil
	}

	return []string{
		detailField("path", checkout.Path),
		detailField("kind", checkout.Kind()),
		detailField("branch", orDash(checkout.Branch)),
		detailField("state", checkoutState(checkout)),
		detailField("last commit", relativeOrDash(checkout.LastCommit)),
	}
}

func (m Model) stashDetailLines(width int) []string {
	stash, ok := m.selectedPanelStash()
	if !ok {
		return nil
	}

	lines := []string{
		detailField("message", stash.Message),
		detailField("branch", orDash(stash.Branch)),
		detailField("created", stash.RelativeDate()),
	}

	diffstat, loaded := m.stashDiffstat[stash.Index]
	if !loaded {
		return append(lines, "", styles.SubtitleStyle.Render("loading diffstat…"))
	}

	lines = append(lines, "", styles.HeaderStyle.Render("diffstat"))
	for _, line := range strings.Split(diffstat, "\n") {
		lines = append(lines, styles.TableRowStyle.Render(truncate("  "+line, width)))
	}

	return lines
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
func (m Model) selectedPanelBranch() (models.BranchInfo, bool) {
	if m.focusedPanel != panelBranches || m.detailCursor >= len(m.branches) {
		return models.BranchInfo{}, false
	}

	return m.branches[m.detailCursor], true
}

func (m Model) selectedPanelPR() (models.PRInfo, bool) {
	if m.focusedPanel != panelPRs || m.detailCursor >= len(m.prs) {
		return models.PRInfo{}, false
	}

	return m.prs[m.detailCursor], true
}

func (m Model) selectedPanelCheckout() (models.PeerCheckout, bool) {
	checkouts := m.RepoCheckouts()
	if m.focusedPanel != panelPeers || m.detailCursor >= len(checkouts) {
		return models.PeerCheckout{}, false
	}

	return checkouts[m.detailCursor], true
}

func (m Model) selectedPanelStash() (models.StashDetail, bool) {
	if m.focusedPanel != panelStashes || m.detailCursor >= len(m.stashes) {
		return models.StashDetail{}, false
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

// panelFooter names the panel jump keys and the actions the focused panel
// supports, so the keys on offer always match what is selected.
func panelFooter(panels []panelContent, focused, width int) string {
	keys := panels[0].key + "-" + panels[len(panels)-1].key

	hints := []footerHint{
		{key: keys, desc: "panels", priority: panelHintPriority},
		{key: keyNavPair, desc: descNav, priority: panelHintPriority - navHintStep},
		{key: "space", desc: nameFind, priority: panelHintPriority - navHintStep*2},
		{key: keyEsc, desc: "back", priority: panelHintPriority - navHintStep*3},
	}

	if focused >= 0 && focused < len(panels) {
		hints = append(hints, panelActionHints(panels[focused].id)...)
	}

	parts := make([]string, 0, len(hints))
	for _, h := range fittingHints(hints, width) {
		parts = append(parts, styles.FooterKeyStyle.Render(h.key)+styles.FooterDescStyle.Render(" "+h.desc))
	}

	return strings.Join(parts, "  ")
}

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func panelActionHints(id panelID) []footerHint {
	switch id {
	case panelBranches:
		return []footerHint{
			{key: keyEnter, desc: nameBranch, priority: 5},
			{key: "c", desc: "switch", priority: 4},
			{key: "p", desc: "push", priority: 2},
			{key: "N", desc: "new PR", priority: 3},
		}
	case panelPRs:
		return []footerHint{
			{key: keyEnter, desc: "PR", priority: 5},
			{key: "g", desc: "checkout", priority: 4},
			{key: "M", desc: "squash-merge", priority: 1},
		}
	case panelPeers:
		return []footerHint{{key: keyEnter, desc: "jump", priority: 5}}
	case panelStatus, panelStashes, panelNotes:
		return nil
	}

	return nil
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
