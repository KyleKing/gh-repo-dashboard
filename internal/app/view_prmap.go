package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
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

func (m Model) prMapScreen() screen {
	entries := m.buildPRMap()

	facts := []frame.Fact{{Text: prMapSummary(entries)}}
	if pending := m.prMapPending(); pending > 0 {
		facts = append(facts, frame.Fact{Text: "loading " + strconv.Itoa(pending), Tone: frame.Accent})
	}

	return screen{
		title: "Open PRs across " + strconv.Itoa(len(m.filteredPaths)) + " repos",
		facts: facts,
		body:  m.renderPRMapTable(entries),
		hints: prMapHints(),
	}
}

func (m Model) renderPRMapTable(entries []prMapEntry) string {
	if len(entries) == 0 {
		if m.prMapPending() > 0 {
			return m.loadingPlaceholder("Loading pull requests")
		}

		return m.emptyPlaceholder("No open pull requests and no unpushed branches",
			"Every visible repo is level with its remote.")
	}

	layout := fitDetailCols(prMapColSpecs, listWidth(m.viewWidth()))
	rows := []string{detailHeader(layout)}

	window := visibleRange(m.prMapCursor, len(entries), m.bodyHeight()-prMapHeaderRows)
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
			colMapLocation: withSelection(styles.Skin.Body, selected),
		}

		if entry.HasPR() {
			values[colMapPR] = "#" + strconv.Itoa(entry.PR.Number)
			values[colMapTitle] = entry.PR.Title
			values[colMapState] = prStateCell(entry.PR)
			values[colMapActivity] = display.PRActivitySummary(entry.PR)
			cellStyles[colMapState] = withSelection(prStateStyle(entry.PR), selected)
			cellStyles[colMapActivity] = withSelection(styles.Skin.Subtitle, selected)
		}

		if entry.Location == emDash {
			cellStyles[colMapLocation] = style
		}

		cells := renderCells(layout, values, cellStyles, &style)
		rows = append(rows, detailRow(rowCursorFor(selected), layout, cells, style))
	}

	return strings.Join(rows, "\n")
}

// prMapHeaderRows is the column header the map spends before its rows.
const prMapHeaderRows = 1

// prMapHints names the map's actions.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func prMapHints() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 2},
		{Key: keyEnter, What: "open repo", Priority: 8},
		{Key: "o", What: "open", Priority: 3},
		{Key: keyEsc, What: descBack, Priority: 9},
	}
}
