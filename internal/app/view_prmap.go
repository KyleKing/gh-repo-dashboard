package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

const (
	colMapRepo     = "REPO"
	colMapPR       = "PR"
	colMapTitle    = "TITLE"
	colMapState    = "STATE"
	colMapActivity = "ACTIVITY"
	colMapLocation = "LOCAL"
)

// prMapColSpecs lays out the fleet map. LOCAL is the column the view exists
// for, so it outlasts everything but the repo and the pull request number.
//
//nolint:mnd // the numbers are this table's geometry, not constants used elsewhere
var prMapColSpecs = []table.Column{
	{Key: colMapRepo, Title: colMapRepo, Min: 16, Weight: 2},
	{Key: colMapPR, Title: colMapPR, Min: 7},
	{Key: colMapTitle, Title: colMapTitle, Min: 20, Weight: 3, Priority: 3},
	{Key: colMapState, Title: colMapState, Min: 8, Priority: 1},
	{Key: colMapActivity, Title: colMapActivity, Min: 20, Weight: 1, Priority: 2},
	{Key: colMapLocation, Title: colMapLocation, Min: 18, Weight: 1, Priority: 4},
}

func (m Model) renderPRMap() string {
	var b strings.Builder

	entries := m.buildPRMap()

	b.WriteString(m.renderPRMapBreadcrumbs(entries))
	b.WriteString("\n\n")
	b.WriteString(m.renderPRMapTable(entries))

	footer := m.prMapFooter()
	contentLines := strings.Count(b.String(), "\n")
	if padding := m.height - contentLines - 2; padding > 0 { //nolint:mnd // the footer and its blank line
		b.WriteString(strings.Repeat("\n", padding))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

func (m Model) renderPRMapBreadcrumbs(entries []prMapEntry) string {
	title := styles.TitleStyle.Render("Open PRs across " + strconv.Itoa(len(m.filteredPaths)) + " repos")
	badges := []string{styles.Badge(prMapSummary(entries), styles.CountBadgeStyle)}

	if pending := m.prMapPending(); pending > 0 {
		badges = append(badges, styles.Badge("loading "+strconv.Itoa(pending), styles.CountBadgeStyle))
	}

	return joinWithinWidth(title, badges, listWidth(m.width))
}

func (m Model) renderPRMapTable(entries []prMapEntry) string {
	if len(entries) == 0 {
		if m.prMapPending() > 0 {
			return m.loadingPlaceholder("Loading pull requests")
		}

		return m.emptyPlaceholder("No open pull requests and no unpushed branches",
			"Every visible repo is level with its remote.")
	}

	layout := fitDetailCols(prMapColSpecs, listWidth(m.width))
	rows := []string{detailHeader(layout)}

	window := visibleRange(m.prMapCursor, len(entries), m.height-prMapChromeHeight)
	for i := window.start; i < window.end; i++ {
		entry := entries[i]
		selected := i == m.prMapCursor
		style := rowStyleFor(selected)

		values := map[string]string{
			colMapRepo:     entry.Repo,
			colMapPR:       localOnlyLabel,
			colMapTitle:    "local branch " + entry.Branch,
			colMapState:    emDash,
			colMapActivity: emDash,
			colMapLocation: entry.Location,
		}
		cellStyles := map[string]lipgloss.Style{
			colMapLocation: withSelection(styles.BranchStyle, selected),
		}

		if entry.HasPR() {
			values[colMapPR] = "#" + strconv.Itoa(entry.PR.Number)
			values[colMapTitle] = entry.PR.Title
			values[colMapState] = prStateCell(entry.PR)
			values[colMapActivity] = entry.PR.ActivitySummary()
			cellStyles[colMapState] = withSelection(prStateStyle(entry.PR), selected)
			cellStyles[colMapActivity] = withSelection(styles.SubtitleStyle, selected)
		}

		if entry.Location == emDash {
			cellStyles[colMapLocation] = style
		}

		rows = append(rows, detailRow(rowCursorFor(selected), layout, renderCells(layout, values, cellStyles, &style)))
	}

	return strings.Join(rows, "\n")
}

// prMapChromeHeight is how many lines the map spends on its header, its column
// header, and its footer, leaving the rest for rows.
const prMapChromeHeight = 5

// prMapFooter names the map's actions, collapsing by the shared hint rules.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func (m Model) prMapFooter() string {
	hints := []footerHint{
		{key: keyNavPair, desc: descNav, priority: 2},
		{key: keyEnter, desc: "open repo", priority: 8},
		{key: "o", desc: "browser", priority: 3},
		{key: keyEsc, desc: "back", priority: 9},
	}

	parts := make([]string, 0, len(hints))
	for _, h := range fittingHints(hints, listWidth(m.width)) {
		parts = append(parts, styles.FooterKeyStyle.Render(h.key)+styles.FooterDescStyle.Render(" "+h.desc))
	}

	return strings.Join(parts, "  ")
}
