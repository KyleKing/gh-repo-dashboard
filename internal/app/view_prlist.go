package app

import (
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/markdown"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// prListChromeHeight is what the tab bar, heading, blank lines, and footer
// spend before a row fits.
const prListChromeHeight = 9

// viewSearchIndent lines a saved view's query up under its name in the picker.
const viewSearchIndent = 2

const (
	scopeThisRepo   = "this repo"
	scopeEverywhere = "everywhere"
	descDetail      = "detail"
	descView        = "view"
)

//nolint:mnd // the numbers are this table's geometry, not constants reused elsewhere
var prSearchColSpecs = []table.Column{
	{Key: colPRNumber, Title: colPRNumber, Min: 6},
	{Key: colPRTitle, Title: colPRTitle, Min: 20, Weight: 3},
	{Key: colPRAuthor, Title: colPRAuthor, Min: 10, Priority: 3},
	{Key: colChecks, Title: colChecks, Min: 10, Priority: 2},
	{Key: colPRActivity, Title: colPRActivity, Min: 16, Weight: 1, Priority: 1},
	{Key: colPRState, Title: colPRState, Min: 7, Priority: 4},
	{Key: colPRUpdated, Title: colPRUpdated, Min: 10, Priority: 5},
}

// prSearchRepoCol leads the table only for a search that spans repositories,
// since a repo-scoped view would spend the width repeating one name.
//
//nolint:mnd // the numbers are this table's geometry, not constants reused elsewhere
var prSearchRepoCol = table.Column{
	Key: colBatchName, Title: colBatchName, Min: 12, Weight: 1, Priority: 6, Trim: table.TrimLeft,
}

func prSearchCols(fleet bool) []table.Column {
	if !fleet {
		return prSearchColSpecs
	}

	return append([]table.Column{prSearchRepoCol}, prSearchColSpecs...)
}

func (m Model) renderPRList() string {
	var b strings.Builder

	width := listWidth(m.width)

	b.WriteString(m.renderTabBar(width))
	b.WriteString("\n\n")
	b.WriteString(m.renderPRListHeading(width))
	b.WriteString("\n\n")
	b.WriteString(m.renderPRListBody(width))
	b.WriteString("\n\n")
	b.WriteString(styles.FooterStyle.Render(m.prListFooter(width)))

	return b.String()
}

// renderPRListHeading names the view being shown, its scope, and how many rows
// answered it, so a short list is never mistaken for a failed read.
func (m Model) renderPRListHeading(width int) string {
	view := m.currentPRView()

	scope := scopeThisRepo
	if m.prFleet {
		scope = scopeEverywhere
	}

	head := styles.TitleStyle.Render(view.Name) + " " + styles.SubtitleStyle.Render(scope)

	var badge string

	switch {
	case m.prSearchLoading:
		badge = styles.SubtitleStyle.Render("searching…")
	case m.prSearchError != "":
		badge = styles.ErrorStyle.Render(m.prSearchError)
	case m.prPredicateText != "":
		counts := strconv.Itoa(len(m.visiblePRs())) + "/" + strconv.Itoa(len(m.prSearch))
		badge = styles.SubtitleStyle.Render(counts + " open")
	default:
		badge = styles.SubtitleStyle.Render(strconv.Itoa(len(m.prSearch)) + " open")
	}

	if m.prPredicateText != "" {
		badge += "  " + styles.Badge(m.prPredicateText, styles.FilterBadgeStyle)
	}

	return truncate(head+"  "+badge, width) + "\n" +
		styles.SubtitleStyle.Render(truncate(m.effectiveQuery(), width))
}

// effectiveQuery is the query as it will actually run: the session-scoped
// ":pr-query" override when one is set, else the current view's own Search,
// rewritten for a widened search where the subject and sort differ from what
// the view says.
func (m Model) effectiveQuery() string {
	query := m.prQueryText()
	if !m.prFleet {
		return query
	}

	return strings.Join(github.FleetSearchArgs(query), " ")
}

func (m Model) renderPRListBody(width int) string {
	visible := m.visiblePRs()
	if len(visible) == 0 {
		return styles.SubtitleStyle.Render(m.prListEmptyLabel())
	}

	bodyHeight := max(m.height-prListChromeHeight, 1)
	previewHeight := m.prPreviewHeight(bodyHeight)
	tableHeight := bodyHeight - previewHeight

	layout := table.Fit(prSearchCols(m.prFleet), width-cursorWidth)
	window := visibleRange(m.prSearchCursor, len(visible), tableHeight)

	lines := []string{detailHeader(layout)}
	for i := window.start; i < window.end; i++ {
		lines = append(lines, renderPRSearchRow(&visible[i], layout, i == m.prSearchCursor, width))
	}

	list := strings.Join(lines, "\n")
	if previewHeight == 0 {
		return list
	}

	return list + "\n" + fitBlock(m.renderPRPreviewBlock(width, previewHeight), previewHeight)
}

// Preview region geometry, the PRs tab's counterpart to the Repos tab's own
// expanded region: a smaller share since a pull request's preview reads
// shorter than a repo's full status.
const (
	prPreviewPercent = 40
	prPreviewMinRows = 4
)

// prPreviewHeight is how many of the body's lines the preview region holds,
// or zero when it is closed.
func (m Model) prPreviewHeight(body int) int {
	if !m.prPreviewOpen {
		return 0
	}

	room := body - prPreviewMinRows
	if room < prPreviewMinRows {
		return 0
	}

	return min(body*prPreviewPercent/percentDenominator, room)
}

// prPreviewMaxDescLines caps how much of a description the region renders,
// on top of whatever the region's own height already clips to.
const prPreviewMaxDescLines = 6

// renderPRPreviewBlock renders the cursor row's preview closed by a divider
// captioning it, mirroring the Repos tab's own notesDivider so a region reads
// the same way in both places.
func (m Model) renderPRPreviewBlock(width, height int) string {
	pr, ok := m.selectedSearchPR()
	if !ok {
		return ""
	}

	repoLabel := pr.Repo
	if repoLabel == "" {
		repoLabel = filepath.Base(m.prSearchRepo())
	}

	lines := padBottom(m.renderPRPreviewLines(pr, width), height-1)

	return strings.Join(append(lines, notesDivider(repoLabel, "#"+strconv.Itoa(pr.Number), width)), "\n")
}

// renderPRPreviewLines renders the row's description, reviewers, and CI
// checks, or a placeholder while the detail behind them is still loading.
func (m Model) renderPRPreviewLines(pr models.PRInfo, width int) []string {
	repo, found := m.searchPRRepoPath(pr)
	if !found {
		return []string{styles.SubtitleStyle.Render("no local checkout to preview from")}
	}

	detail, loaded := m.prPreview[prPreviewKey(repo, pr.Number)]
	if !loaded {
		return []string{styles.SubtitleStyle.Render(readingLabel)}
	}

	reviewers := "none requested"
	if len(detail.Reviewers) > 0 {
		reviewers = strings.Join(detail.Reviewers, ", ")
	}

	reviewStyle := styles.SubtitleStyle
	switch detail.ReviewStatus() {
	case models.ReviewApproved:
		reviewStyle = styles.CleanStyle
	case models.ReviewChangesRequested:
		reviewStyle = styles.ErrorStyle
	}

	// The preview's own detail read carries no CI rollup (only the full detail
	// page's CheckDetails list does), so the checks summary comes from the
	// row's own aggregate instead, already populated when the search ran.
	const summaryLines = 3

	lines := make([]string, 0, summaryLines+prPreviewMaxDescLines)
	lines = append(lines,
		styles.SubtitleStyle.Render("Reviewers: ")+reviewers,
		styles.SubtitleStyle.Render("Review: ")+reviewStyle.Render(detail.ReviewStatus())+
			"   "+styles.SubtitleStyle.Render("Checks: ")+formatChecksCell(&pr),
		"",
	)

	return append(lines, markdown.Render(orDash(strings.TrimSpace(detail.Body)), width, prPreviewMaxDescLines)...)
}

func (m Model) prListEmptyLabel() string {
	switch {
	case m.prSearchLoading:
		return "searching…"
	case m.prSearchError != "":
		return m.prSearchError
	case m.prPredicateText != "" && len(m.prSearch) > 0:
		return "No pull requests match " + m.prPredicateText
	}

	return "Nothing matches this view"
}

func renderPRSearchRow(pr *models.PRInfo, layout table.Layout, selected bool, width int) string {
	style := rowStyleFor(selected)

	values := map[string]string{
		colBatchName:  orDash(pr.Repo),
		colPRActivity: pr.ActivitySummary(),
		colPRNumber:   "#" + strconv.Itoa(pr.Number),
		colPRTitle:    pr.Title,
		colPRAuthor:   orDash(pr.Author),
		colPRState:    prStateCell(pr),
		colChecks:     formatChecksCell(pr),
		colPRUpdated:  relativeOrDash(pr.UpdatedAt),
	}
	cellStyles := map[string]lipgloss.Style{
		colBatchName: withSelection(styles.SubtitleStyle, selected),
		colPRState:   withSelection(prStateStyle(pr), selected),
		colChecks:    withSelection(checksCellStyle(pr, style), selected),
		colPRUpdated: withSelection(styles.SubtitleStyle, selected),
	}

	row := detailRow(rowCursorFor(selected), layout, renderCells(layout, values, cellStyles, &style))
	if badge := reviewerBadge(pr); badge != "" {
		row = joinWithinWidth(row, []string{badge}, width)
	}

	return row
}

// reviewerBadge flags an open, non-draft pull request that has nobody
// currently requested to review it, or names how many reviewers are
// requested otherwise. A merged, closed, or draft row carries neither, since
// reviewer assignment stops mattering once there's no review left to give.
func reviewerBadge(pr *models.PRInfo) string {
	if pr.NeedsReviewer() {
		return styles.Badge("needs reviewer", styles.WarningStyle)
	}
	if pr.State != models.PRStatusOpen || pr.IsDraft || len(pr.Reviewers) == 0 {
		return ""
	}

	return styles.Badge(strconv.Itoa(len(pr.Reviewers))+" "+plural(len(pr.Reviewers), "reviewer", "reviewers"),
		styles.SubtitleStyle)
}

func (m Model) prListFooter(width int) string {
	previewHint := "preview"
	if m.prPreviewOpen {
		previewHint = "hide preview"
	}

	hints := []footerHint{
		{key: keyNavPair, desc: descNav, priority: panelHintPriority},
		{key: keyEnter, desc: descDetail, priority: panelHintPriority - navHintStep},
		{key: "v", desc: previewHint, priority: panelHintPriority - navHintStep},
		{key: "f", desc: descView, priority: panelHintPriority - navHintStep},
		{key: keyBracketPair, desc: "cycle views", priority: panelHintPriority - navHintStep*3},
		{key: "*", desc: m.prScopeHint(), priority: panelHintPriority - navHintStep*2},
		{key: panelActionLeader, desc: "act", priority: panelHintPriority - navHintStep*2},
		{key: "?", desc: nameHelp, priority: panelHintPriority - navHintStep*5},
		{key: keyEsc, desc: "repos", priority: panelHintPriority - navHintStep*4},
	}

	parts := make([]string, 0, len(hints))
	for _, h := range fittingHints(hints, width) {
		parts = append(parts, styles.FooterKeyStyle.Render(h.key)+styles.FooterDescStyle.Render(" "+h.desc))
	}

	return strings.Join(parts, "  ")
}

func (m Model) prScopeHint() string {
	if m.prFleet {
		return scopeThisRepo
	}

	return scopeEverywhere
}

// renderPRViewModal lists the saved views so one can be picked by name rather
// than cycled into.
func (m Model) renderPRViewModal() string {
	views := models.PRViews()

	lines := make([]string, 0, 2*len(views)+4)
	lines = append(lines, styles.TitleStyle.Render("Pull request views"), "")
	for i, view := range views {
		cursor := "  "
		if i == m.prViewIndex {
			cursor = "> "
		}

		rowStyle := styles.TableRowStyle
		if i == m.prViewIndex {
			rowStyle = styles.SelectedRowStyle
		}

		lines = append(lines,
			cursor+styles.HelpKeyStyle.Render(padCell(strconv.Itoa(i+1), modalKeyColWidth))+"  "+
				rowStyle.Render(view.Name),
			strings.Repeat(" ", modalKeyColWidth+cursorWidth+viewSearchIndent)+
				styles.SubtitleStyle.Render(view.Search))
	}

	lines = append(lines, "",
		styles.FooterKeyStyle.Render("enter/number")+styles.FooterDescStyle.Render(" show")+"  "+
			styles.FooterKeyStyle.Render(keyEsc)+styles.FooterDescStyle.Render(" back"))

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Blue).
		Padding(confirmModalVPad, confirmModalHPad).
		Render(strings.Join(lines, "\n"))

	return centerModal(m, content)
}
