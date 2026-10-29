package app

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/markdown"
	"github.com/kyleking/aragonite/tui/preview"
	"github.com/kyleking/aragonite/tui/table"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// gridSpec is the focused view's geometry: the section column's share, the
// narrowest detail pane that still splits beside it, and the padding that
// keeps the two halves visually apart. The split is fixed, so moving focus
// never moves a column.
var gridSpec = preview.Grid{
	SidePercent: sidePercent,
	SideMin:     sideMin,
	DetailMin:   detailMin,
	StackMin:    stackMin,
	RulePad:     1,
	SectionGap:  1,
}

const (
	// The section column's share of a split grid.
	sidePercent = 40
	// The narrowest a section column is drawn at.
	sideMin = 36
	// The pane width below which the grid stacks instead.
	detailMin = 30
	// The height the pane needs before a stacked grid shows it.
	stackMin = 6
)

// panelSet builds the sections for the selected repo at the given width. A jj
// repo has no Stashes section at all, rather than one explaining its own
// absence. Every other section keeps its rule when it is empty, so the layout
// does not reflow as data lands.
func (m Model) panelSet(width int) []panelContent {
	summary := m.summaries[m.selectedRepo]

	built := []panelContent{
		m.prsPanel(summary, width),
		m.ciPanel(summary, width),
		m.branchesPanel(width),
		m.peersPanel(summary, width),
	}

	if summary.VCSType != vcs.TypeJJ {
		built = append(built, m.stashesPanel(width))
	}

	built = append(built, m.notesPanel(width))

	for i := range built {
		built[i].key = panelKeys[built[i].id]
		built[i].heading = cmp.Or(built[i].heading, strings.ToUpper(built[i].title))
	}

	return built
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

// panelIndex returns the position of id in panels, or -1 when it has no
// section: Status is the repo itself, and a jj repo has no Stashes.
func panelIndex(panels []panelContent, id panelID) int {
	for i, p := range panels {
		if p.id == id {
			return i
		}
	}

	return -1
}

// sectionPRs is the repo's open pull requests: the list the focused view loaded,
// or the one the repo list already carries while that load is in flight.
func (m Model) sectionPRs() []forge.PullRequest {
	if m.detailLoading {
		return m.summaries[m.selectedRepo].OpenPRs
	}

	return m.prs
}

func (m Model) prsPanel(summary models.RepoSummary, width int) panelContent {
	layout := fitPanelCols(prPanelSpecs, width)
	turn := prTurn(summary.OpenPRs, m.viewer)
	prs := m.sectionPRs()

	rows := make([]string, 0, len(prs))
	needsYou := false

	for i := range prs {
		pr := &prs[i]
		selected := m.panelSelected(panelPRs, i)
		style := rowStyleFor(selected)

		state, stateStyle := prRowState(pr, pr.Number == turn)
		if pr.Number == turn {
			needsYou = true
		}

		values := map[string]string{
			colPRTitle: "#" + strconv.Itoa(pr.Number) + " " + pr.Title,
			colPRState: state,
		}
		cellStyles := map[string]lipgloss.Style{colPRState: withSelection(stateStyle, selected)}

		rows = append(
			rows,
			detailRow(rowCursorFor(selected), layout, renderCells(layout, values, cellStyles, &style), style),
		)
	}

	relevance := preview.Idle
	if len(prs) > 0 {
		relevance = preview.Present
	}

	if needsYou {
		relevance = preview.Urgent
	}

	return panelContent{
		id: panelPRs, title: tabNamePRs, heading: "PULL REQUESTS", count: strconv.Itoa(len(prs)),
		relevance: relevance, rows: rows, selectable: true,
	}
}

// prRowState is the PR row's right cell: a draft, the review decision, a failing
// check count, or the viewer's turn, in the face that says how urgent it is.
func prRowState(pr *forge.PullRequest, yourTurn bool) (string, lipgloss.Style) {
	failing := display.ChecksSummary(pr.Checks) == forge.StatusFailing

	switch {
	case yourTurn:
		return "←you", styles.Skin.Accent
	case failing:
		return "✗ " + strconv.Itoa(pr.Checks.Failing), styles.Skin.Error
	case pr.IsDraft:
		return "draft", styles.Skin.Muted
	}

	return display.PRReviewGlyph(pr), styles.Skin.Muted
}

// ciWorkflows are the default branch's latest run of each workflow, failing
// first, then unfinished, then the rest by name.
func (m Model) ciWorkflows() []forge.CIWorkflowRun {
	ci := m.summaries[m.selectedRepo].CI
	if ci == nil {
		return nil
	}

	runs := slices.Clone(ci.Workflows)
	slices.SortStableFunc(runs, func(a, b forge.CIWorkflowRun) int {
		return cmp.Or(
			cmp.Compare(ciRank(a), ciRank(b)),
			strings.Compare(a.Workflow, b.Workflow),
		)
	})

	return runs
}

func ciRank(run forge.CIWorkflowRun) int {
	switch {
	case run.Conclusion == models.CIFailureConclusion:
		return 0
	case run.Status != forge.StatusCompleted:
		return 1
	}

	return 2 //nolint:mnd // third of three ranks
}

func ciRunGlyph(run *forge.CIWorkflowRun) string {
	switch {
	case run.Conclusion == models.CIFailureConclusion:
		return "✗"
	case run.Status != forge.StatusCompleted:
		return "…"
	}

	return "✓"
}

func (m Model) ciPanel(summary models.RepoSummary, width int) panelContent {
	layout := fitPanelCols(ciPanelSpecs, width)
	runs := m.ciWorkflows()

	rows := make([]string, 0, len(runs))
	failing := 0

	for i := range runs {
		run := &runs[i]
		selected := m.panelSelected(panelCI, i)
		style := rowStyleFor(selected)
		cellStyles := map[string]lipgloss.Style{}

		if run.Conclusion == models.CIFailureConclusion {
			failing++
			cellStyles[colCIName] = withSelection(styles.Skin.Error, selected)
		}

		state := run.Conclusion
		if run.Status != forge.StatusCompleted {
			state = run.Status
		}

		values := map[string]string{
			colCIName:  ciRunGlyph(run) + " " + run.Workflow,
			colCIState: state,
			colCIAge:   display.RelativeTimeCompact(run.UpdatedAt),
		}
		rows = append(
			rows,
			detailRow(rowCursorFor(selected), layout, renderCells(layout, values, cellStyles, &style), style),
		)
	}

	relevance := preview.Idle
	if len(runs) > 0 {
		relevance = preview.Present
	}

	if failing > 0 {
		relevance = preview.Urgent
	}

	heading := "CI"
	if summary.CI != nil && summary.CI.Branch != "" {
		heading += compactSignalSep + summary.CI.Branch
	}

	return panelContent{
		id: panelCI, title: tabNameCI, heading: heading, count: strconv.Itoa(len(runs)),
		relevance: relevance, rows: rows, selectable: true,
	}
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

	relevance := preview.Idle
	if len(m.branches) > 0 {
		relevance = preview.Present
	}
	if unpushed > 0 {
		relevance = preview.Urgent
	}

	return panelContent{
		id: panelBranches, title: tabNameBranches, count: strconv.Itoa(len(m.branches)),
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
		branchStyle := styles.Skin.Subtitle
		if !vcs.IsDefaultBranchName(checkout.Branch) {
			branchStyle = styles.Skin.Body
		}
		if conflicts[branch] {
			branch += " " + conflictMark
			branchStyle = styles.Skin.Warning
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

		cells := renderCells(layout, values, cellStyles, &style)
		rows = append(rows, detailRow(rowCursorFor(selected), layout, cells, style))
	}

	relevance := preview.Idle
	if len(checkouts) > 0 {
		relevance = preview.Present
	}
	if conflicted > 0 {
		relevance = preview.Urgent
	}

	return panelContent{
		id: panelPeers, title: tabNamePeers, count: strconv.Itoa(len(checkouts)),
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
			colStashDate:    display.StashRelativeDate(stash),
		}

		rows = append(rows, detailRow(rowCursorFor(selected), layout, renderCells(layout, values, nil, &style), style))
	}

	relevance := preview.Idle
	if len(m.stashes) > 0 {
		relevance = preview.Present
	}

	return panelContent{
		id: panelStashes, title: tabNameStashes, count: strconv.Itoa(len(m.stashes)),
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

		rows = append(rows, rowCursorFor(selected)+style.Render(
			table.TruncateMiddle(padCell(label, width-gutterWidth), width-gutterWidth),
		))
	}

	relevance := preview.Idle
	if len(rows) > 0 {
		// A note is something the last session left for this one, so it
		// outranks a list the repo itself can always answer.
		relevance = preview.Urgent
	}

	return panelContent{
		id: panelNotes, title: tabNameNotes, count: strconv.Itoa(len(m.notesFiles)),
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

func (m Model) repoDetailScreen() screen {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return screen{title: appTitle, body: "Repository not found"}
	}

	panels := m.panelSet(m.sectionWidth())

	return screen{
		title: "Repos › " + summary.Name(),
		facts: m.repoFacts(summary),
		body:  m.renderPanelGridOf(panels, panelIndex(panels, m.focusedPanel)),
		hints: m.panelHints(panels),
	}
}

// repoFacts is the header line: the repo's identity, then the same tokens the
// list's NEEDS column carries, and the open PR count.
func (m Model) repoFacts(summary models.RepoSummary) []frame.Fact {
	n := m.needsOf(summary)

	facts := []frame.Fact{{Text: summary.VCSType.String() + compactSignalSep + summary.Branch}}

	for _, token := range n.tokens() {
		facts = append(facts, frame.Fact{Text: token.text, Tone: token.tone.frameTone()})
	}

	if len(facts) == 1 && len(n.due) == 0 {
		facts = append(facts, frame.Fact{Text: "quiet"})
	}

	if n.open > 0 {
		facts = append(facts, frame.Fact{Text: strconv.Itoa(n.open) + " " + plural(n.open, "PR", "PRs")})
	}

	return facts
}

// gridWidth is the width the focused repo view renders into: the whole
// terminal, since it is one surface rather than a card on a margin.
func (m Model) gridWidth() int {
	return m.width
}

// sectionWidth is the width the section column renders its rows into.
func (m Model) sectionWidth() int {
	if gridSpec.Stacked(m.isCompact(), m.gridWidth()) {
		return m.gridWidth()
	}

	return gridSpec.SideWidth(m.gridWidth())
}

func (m Model) renderPanelGrid() string {
	if _, ok := m.summaries[m.selectedRepo]; !ok {
		return "Repository not found"
	}

	panels := m.panelSet(m.sectionWidth())

	return m.renderPanelGridOf(panels, panelIndex(panels, m.focusedPanel))
}

func (m Model) renderPanelGridOf(panels []panelContent, focused int) string {
	sections := m.previewSections(panels)
	dw, _ := gridSpec.DetailSize(m.isCompact(), m.gridWidth(), m.bodyHeight(),
		preview.Content{Sections: sections})

	return strings.Join(gridSpec.Render(previewStyles, m.isCompact(), m.gridWidth(), m.bodyHeight(),
		preview.Content{
			Sections: sections,
			Focused:  focused,
			Cursor:   m.detailCursor,
			Detail: preview.Detail{
				Title:   m.detailTitle(),
				Lines:   m.panelDetailLines(dw),
				Scroll:  m.detailScroll,
				Focused: m.detailFocused,
			},
		}), "\n")
}

// paneSize is the detail pane's interior at the current terminal size, which
// is the geometry a scroll offset clamps against.
type paneSize struct {
	width, height int
}

func (m Model) detailPaneSize() paneSize {
	w, h := gridSpec.DetailSize(m.isCompact(), m.gridWidth(), m.bodyHeight(),
		preview.Content{Sections: m.previewSections(m.panelSet(m.sectionWidth()))})

	return paneSize{width: w, height: h}
}

// renderSections draws every section at the height its relevance earned: a
// labeled rule, then the rows the cursor window leaves in view.
func (m Model) renderSections(panels []panelContent, focused, width, height int) []string {
	sections := m.previewSections(panels)

	return preview.Column(previewStyles, sections, focused, m.detailCursor, height, width, gridSpec.SectionGap)
}

// previewSections resolves the panels as the shared layout's sections, with
// the loading marker standing in for counts still arriving.
func (m Model) previewSections(panels []panelContent) []preview.Section {
	for i := range panels {
		if m.detailLoading && panels[i].id != panelPRs {
			panels[i].count = "…"
		}
	}

	return sectionsOf(panels)
}

// renderDetailPane is the detail half on its own: the item's title and scroll
// marker, then its lines windowed to the pane's interior.
func (m Model) renderDetailPane(width, height int) []string {
	d := preview.Detail{Title: m.detailTitle(), Lines: m.panelDetailLines(width), Scroll: m.detailScroll}
	return preview.Pad(append([]string{d.Head(previewStyles, width, height)}, d.Window(height)...), height+1, width)
}

// detailTitle labels the detail pane with the item it is describing.
func (m Model) detailTitle() string {
	panels := m.panelSet(m.sectionWidth())
	if i := panelIndex(panels, m.focusedPanel); i >= 0 {
		return m.panelDetailTitle(panels[i])
	}

	return m.summaries[m.selectedRepo].Name()
}

func (m Model) panelDetailTitle(p panelContent) string {
	switch p.id {
	case panelPRs:
		if pr, ok := m.selectedPanelPR(); ok {
			return "#" + strconv.Itoa(pr.Number) + " " + pr.Title
		}
	case panelCI:
		if run, ok := m.selectedPanelRun(); ok {
			return run.Workflow
		}
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
	case panelPRs:
		lines = m.prDetailLines(width)
	case panelCI:
		lines = m.ciDetailLines(width)
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
		lines = append(lines, detailField("newest edit", m.newestFile+" "+display.RelativeTime(m.newestFileTime)))
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

	var lines []string

	if branch.IsCurrent && m.summaries[m.selectedRepo].UncommittedCount() > 0 {
		lines = append(lines, m.workingDiffLines(m.selectedRepo, width)[1:]...)
		lines = append(lines, "")
	}

	if m.branchDetail.Branch.Name != branch.Name {
		return append(lines, styles.Skin.Subtitle.Render("loading commits…"))
	}

	lines = append(lines, styles.Skin.Subtitle.Bold(true).Render("recent commits"))
	for _, commit := range m.branchDetail.Commits {
		lines = append(lines, styles.Skin.Body.Render(
			truncate("  "+commit.ShortHash+" "+commit.Subject, width),
		))
	}

	return lines
}

// prDetailLines is the selected pull request: where it merges, its checks, and
// its description. The list row already carries the number, title, and state.
func (m Model) prDetailLines(width int) []string {
	pr, ok := m.selectedPanelPR()
	if !ok {
		return nil
	}

	detail := m.prDetail
	loaded := detail.Number == pr.Number

	meta := []string{pr.HeadRef + " → " + pr.BaseRef}
	if pr.IsDraft {
		meta = append([]string{"draft"}, meta...)
	}

	if author := cmp.Or(detail.Author, pr.Author); author != "" {
		meta = append(meta, "@"+author)
	}

	if when := cmp.Or(detail.UpdatedAt, pr.UpdatedAt); !when.IsZero() {
		meta = append(meta, "updated "+display.RelativeTime(when))
	}

	lines := []string{styles.Skin.Subtitle.Render(truncate(strings.Join(meta, compactSignalSep), width))}

	if !loaded {
		lines = append(lines, "", detailField("checks", formatChecksCell(&pr)), "",
			styles.Skin.Subtitle.Render("loading details…"))

		return lines
	}

	lines = append(lines, "")
	lines = append(lines, prCheckLines(detail.CheckDetails, width)...)

	if strings.TrimSpace(detail.Body) != "" {
		lines = append(lines, "")
		lines = append(lines, markdown.Render(detail.Body, width, prBodyMaxLines, styles.Markdown)...)
	}

	return lines
}

// prCheckLines lists the failing checks by name and folds the rest into a count,
// so a long passing run does not push the description off the pane.
func prCheckLines(checks []forge.CheckDetail, width int) []string {
	if len(checks) == 0 {
		return []string{detailField("checks", emDash)}
	}

	var (
		lines   []string
		settled int
	)

	for _, check := range checks {
		status := display.CheckStatusDisplay(check)
		if !isFailingCheck(status) {
			settled++

			continue
		}

		name := check.Name
		if check.Workflow != "" && check.Workflow != check.Name {
			name = check.Workflow + " / " + check.Name
		}

		lines = append(lines, styles.Skin.Error.Render(truncate("✗ "+name+" "+status, width)))
	}

	summary := strconv.Itoa(settled) + " other " + plural(settled, "check", "checks")
	if len(lines) == 0 {
		summary = strconv.Itoa(settled) + " " + plural(settled, "check", "checks") + ", none failing"
	}

	return append([]string{styles.Skin.Subtitle.Bold(true).Render("checks")},
		append(lines, styles.Skin.Subtitle.Render(summary))...)
}

//nolint:misspell // GitHub's own conclusion value is spelled "cancelled"
func isFailingCheck(status string) bool {
	switch status {
	case models.CIFailureConclusion, "error", "cancelled", "timed_out":
		return true
	}

	return false
}

// ciDetailLines is the selected workflow's run: the jobs that failed and where
// to read the log.
func (m Model) ciDetailLines(width int) []string {
	run, ok := m.selectedPanelRun()
	if !ok {
		return nil
	}

	lines := []string{
		detailField("state", cmp.Or(run.Conclusion, run.Status)),
		detailField("started", relativeOrDash(run.StartedAt)),
		detailField("updated", relativeOrDash(run.UpdatedAt)),
	}

	if len(run.FailingJobs) > 0 {
		lines = append(lines, "", styles.Skin.Subtitle.Bold(true).Render("failing jobs"))

		for _, job := range run.FailingJobs {
			lines = append(lines, styles.Skin.Error.Render(truncate("  ✗ "+job, width)))
		}
	}

	if run.URL != "" {
		lines = append(lines, "", styles.Skin.Subtitle.Render(truncate(run.URL, width)))
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
		label, texts = diffWord, m.uncommittedDiff
	}

	body, loaded := texts[repoPath]
	if !loaded {
		return []string{"", styles.Skin.Subtitle.Render("loading " + label + "…")}
	}

	lines := []string{"", styles.Skin.Subtitle.Bold(true).Render(label)}
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
		detailField("created", display.StashRelativeDate(stash)),
	}

	label, texts := "diffstat", m.stashDiffstat
	if m.stashFullDiff {
		label, texts = diffWord, m.stashDiff
	}

	body, loaded := texts[stash.Index]
	if !loaded {
		return append(lines, "", styles.Skin.Subtitle.Render("loading "+label+"…"))
	}

	lines = append(lines, "", styles.Skin.Subtitle.Bold(true).Render(label))
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

	return styles.Skin.Body.Render(truncate(indented, width))
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
		return []string{styles.Skin.Subtitle.Render("(empty file)")}
	}

	return preview.Wrap(styles.Skin.Body, content, width)
}

// Selected-item accessors. Each answers only for its own panel, so a cursor
// left over from another panel can never index the wrong list.
func (m Model) selectedPanelPR() (forge.PullRequest, bool) {
	prs := m.sectionPRs()
	if m.focusedPanel != panelPRs || m.detailCursor >= len(prs) {
		return forge.PullRequest{}, false
	}

	return prs[m.detailCursor], true
}

func (m Model) selectedPanelRun() (forge.CIWorkflowRun, bool) {
	runs := m.ciWorkflows()
	if m.focusedPanel != panelCI || m.detailCursor >= len(runs) {
		return forge.CIWorkflowRun{}, false
	}

	return runs[m.detailCursor], true
}

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

// panelHints names the keys the focused panel supports, so what is on offer
// always matches what is selected and where the keyboard currently is. The
// panel jump keys are absent because each panel's border already carries its
// own. Actions are dimmed rather than dropped where the panel has none.
func (m Model) panelHints(_ []panelContent) []keyhint.Hint {
	navDesc, backDesc := descNav, descBack
	if m.detailFocused {
		navDesc, backDesc = descScroll, "panels"
	}

	hints := []keyhint.Hint{
		{Key: keyNavPair, What: navDesc, Priority: panelHintPriority - navHintStep},
		{Key: keyEsc, What: backDesc, Priority: panelHintPriority - navHintStep*3},
		{Key: "space", What: nameFind, Priority: panelHintPriority - navHintStep*2},
	}

	if !m.detailFocused {
		hints = append(hints, keyhint.Hint{
			Key: keyEnter, What: nameDetail, Priority: panelHintPriority - navHintStep*2,
		})
	}

	hasActions := len(panelActionsFor(m.focusedPanel)) > 0

	return append(hints, keyhint.Hint{
		Key: actionLeader, What: nameActions, Priority: panelHintPriority - navHintStep, Off: !hasActions,
	})
}

func detailField(label, value string) string {
	return preview.Field(previewStyles, label, value)
}

func orDash(value string) string {
	if value == "" {
		return emDash
	}

	return value
}
