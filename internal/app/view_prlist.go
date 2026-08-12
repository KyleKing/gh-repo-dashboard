package app

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// prListChromeHeight is what the tab bar, heading, blank lines, and footer
// spend before a row fits.
const prListChromeHeight = 8

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

	b.WriteString(m.renderTabBar())
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
	default:
		badge = styles.SubtitleStyle.Render(strconv.Itoa(len(m.prSearch)) + " open")
	}

	return truncate(head+"  "+badge, width) + "\n" +
		styles.SubtitleStyle.Render(truncate(m.effectiveQuery(view), width))
}

// effectiveQuery is the query as it will actually run, which for a widened
// search is not what the view says: the subject and the sort are rewritten for
// the command that answers it.
func (m Model) effectiveQuery(view models.PRView) string {
	if !m.prFleet {
		return view.Search
	}

	return strings.Join(github.FleetSearchArgs(view.Search), " ")
}

func (m Model) renderPRListBody(width int) string {
	if len(m.prSearch) == 0 {
		return styles.SubtitleStyle.Render(m.prListEmptyLabel())
	}

	layout := table.Fit(prSearchCols(m.prFleet), width-cursorWidth)
	height := max(m.height-prListChromeHeight, 1)
	window := visibleRange(m.prSearchCursor, len(m.prSearch), height)

	lines := []string{detailHeader(layout)}
	for i := window.start; i < window.end; i++ {
		lines = append(lines, renderPRSearchRow(&m.prSearch[i], layout, i == m.prSearchCursor))
	}

	return strings.Join(lines, "\n")
}

func (m Model) prListEmptyLabel() string {
	switch {
	case m.prSearchLoading:
		return "searching…"
	case m.prSearchError != "":
		return m.prSearchError
	}

	return "Nothing matches this view"
}

func renderPRSearchRow(pr *models.PRInfo, layout table.Layout, selected bool) string {
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

	return detailRow(rowCursorFor(selected), layout, renderCells(layout, values, cellStyles, &style))
}

func (m Model) prListFooter(width int) string {
	hints := []footerHint{
		{key: keyNavPair, desc: descNav, priority: panelHintPriority},
		{key: keyEnter, desc: descDetail, priority: panelHintPriority - navHintStep},
		{key: "f", desc: descView, priority: panelHintPriority - navHintStep},
		{key: "[/]", desc: "cycle views", priority: panelHintPriority - navHintStep*3},
		{key: "*", desc: m.prScopeHint(), priority: panelHintPriority - navHintStep*2},
		{key: panelActionLeader, desc: "act", priority: panelHintPriority - navHintStep*2},
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

// prSearchSummary is what the status line says once a search lands.
func prSearchSummary(count int, view models.PRView) string {
	return fmt.Sprintf("%d for %s", count, view.Name)
}
