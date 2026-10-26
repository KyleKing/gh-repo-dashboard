package app

import "github.com/kyleking/aragonite/tui/table"

// Width limits shared by every full-screen view. Content grows with the
// terminal up to maxContentWidth, past which lines get long enough that
// scanning a row costs more than the extra density is worth.
const (
	frameGutter     = 2
	frameSides      = 2
	maxContentWidth = 140
	minContentWidth = 60
)

// contentWidth returns the width a view may render into for the given terminal
// width. It never returns less than minContentWidth, so a terminal narrower
// than that overflows rather than collapsing to an unusable sliver.
func contentWidth(termWidth int) int {
	return max(min(termWidth-frameSides*frameGutter, maxContentWidth), minContentWidth)
}

// Width of the dense views: the repo table and the focused repo grid. The
// single-column views cap at maxContentWidth because a row past that costs more
// to scan than the density is worth; these two spend surplus width on columns
// and on a second pane, so they keep a proportional margin and a much later cap.
const (
	gridWidthPercent = 90
	gridMaxWidth     = 200
)

// wideContentWidth is the width the focused grid renders into.
func wideContentWidth(termWidth int) int {
	return max(min(termWidth*gridWidthPercent/percentDenominator, gridMaxWidth), minContentWidth)
}

// listWidth is the width the repo table renders into: the wider of the two
// rules. Past roughly 156 cells the grid's proportional rule wins and the table
// spends the surplus on its columns; below that the single-column width does,
// because a proportional margin there costs the table cells it needs and frees
// nothing.
func listWidth(termWidth int) int {
	return max(contentWidth(termWidth), wideContentWidth(termWidth))
}

// padCell truncates text to width and pads it to exactly width display cells.
// Padding is applied before styling so a background color fills the column.
func padCell(text string, width int) string {
	return table.Pad(text, width, table.AlignLeft)
}

// Column keys for the repo list table, doubling as the rendered headers.
const (
	colName     = "NAME"
	colBranch   = "BRANCH"
	colNeeds    = "NEEDS"
	colPR       = "PRs"
	colModified = "MODIFIED"
)

// gutterWidth is the batch-mark gutter on every row.
const gutterWidth = 1

// repoColSpecs lists the repo table's columns. Each flexible column stops growing
// at its Max, so surplus width stays empty rather than stretching NEEDS.
//
//nolint:mnd // the numbers are this table's data: column geometry, not constants used elsewhere
var repoColSpecs = []table.Column{
	{Key: colName, Title: colName, Min: 16, Max: 28, Weight: 3},
	{Key: colBranch, Title: colBranch, Min: 14, Max: 24, Weight: 2},
	{Key: colNeeds, Title: colNeeds, Min: 20, Max: 32, Weight: 4},
	{Key: colPR, Title: colPR, Min: 12, Priority: 2},
	{Key: colModified, Title: colModified, Min: 12, Priority: 1},
}

// layoutRepoCols fits the repo table into width, leaving room for the mark
// gutter that leads every row.
func layoutRepoCols(width int) table.Layout {
	return table.Fit(repoColSpecs, width-gutterWidth)
}
