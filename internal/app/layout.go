package app

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// Frame geometry shared by every full-screen view. Content grows with the
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

// frameLeftPad returns the indent that centers a contentW-wide block within
// termWidth.
func frameLeftPad(termWidth, contentW int) int {
	return max((termWidth-contentW)/frameSides, 0)
}

// frame indents every non-empty line of content so the block sits centered in
// termWidth, then pads each line out to the full frame width. The padding
// matters: a view whose lines are shorter than the previous view's leaves the
// old tail on screen otherwise, because the renderer only repaints the cells a
// line actually covers.
func frame(content string, termWidth, contentW int) string {
	pad := frameLeftPad(termWidth, contentW)
	prefix := strings.Repeat(" ", pad)
	lineWidth := pad + contentW

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line != "" {
			line = prefix + line
		}
		if gap := lineWidth - lipgloss.Width(line); gap > 0 {
			line += strings.Repeat(" ", gap)
		}
		lines[i] = line
	}

	return strings.Join(lines, "\n")
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
	colBranches = "BRs"
	colStatus   = "STATUS"
	colPeers    = "PEERS"
	colPR       = "PR"
	colPRs      = "PRs"
	colTemplate = "TEMPLATE"
	colCI       = "CI"
	colModified = "MODIFIED"
)

// cursorWidth is the leading cursor plus selection-mark gutter on every row.
const cursorWidth = 2

// repoColSpecs lists the repo table's columns in render order.
//
// Collapse priority is information value measured against the real fleet, so
// the lowest priority hides first: BRs already duplicates the count the
// expand region shows, PR is nearly always emDash on a default branch, while
// PEERS and TEMPLATE are the signals worth acting on and survive longest.
// Name, branch, and status carry no priority and never hide.
//
//nolint:mnd // the numbers are this table's data: column geometry, not constants used elsewhere
var repoColSpecs = []table.Column{
	{Key: colName, Title: colName, Min: 16, Weight: 3},
	{Key: colBranch, Title: colBranch, Min: 10, Weight: 2},
	{Key: colBranches, Title: colBranches, Min: 4, Priority: 1},
	{Key: colStatus, Title: colStatus, Min: 12},
	{Key: colPeers, Title: colPeers, Min: 5, Priority: 4},
	{Key: colPR, Title: colPR, Min: 8, Priority: 2},
	{Key: colPRs, Title: colPRs, Min: 6, Priority: 5},
	{Key: colTemplate, Title: colTemplate, Min: 10, Weight: 1, Priority: 6},
	{Key: colCI, Title: colCI, Min: 8, Priority: 7},
	{Key: colModified, Title: colModified, Min: 12, Priority: 3},
}

// layoutRepoCols fits the repo table into width, leaving room for the cursor
// gutter that leads every row.
func layoutRepoCols(width int) table.Layout {
	return table.Fit(repoColSpecs, width-cursorWidth)
}
