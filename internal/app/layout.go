package app

import (
	"strings"

	"charm.land/lipgloss/v2"
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

// frameLeftPad returns the indent that centers a contentWidth block within
// termWidth.
func frameLeftPad(termWidth int) int {
	return max((termWidth-contentWidth(termWidth))/frameSides, 0)
}

// frame indents every non-empty line of content so the block sits centered in
// termWidth, then pads each line out to the full frame width. The padding
// matters: a view whose lines are shorter than the previous view's leaves the
// old tail on screen otherwise, because the renderer only repaints the cells a
// line actually covers.
func frame(content string, termWidth int) string {
	pad := frameLeftPad(termWidth)
	prefix := strings.Repeat(" ", pad)
	lineWidth := pad + contentWidth(termWidth)

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
	text = truncate(text, width)
	if gap := width - lipgloss.Width(text); gap > 0 {
		text += strings.Repeat(" ", gap)
	}

	return text
}

// repoColumn identifies one column of the repo list table.
type repoColumn int

const (
	colName repoColumn = iota
	colBranch
	colStatus
	colPeers
	colPR
	colPRs
	colTemplate
	colModified
)

// colGutter is the gap rendered between two adjacent columns.
const colGutter = 2

// cursorWidth is the leading cursor plus selection-mark gutter on every row.
const cursorWidth = 2

// repoColSpec describes one repo table column: the width it needs at minimum,
// the share of surplus width it absorbs, and how readily it is dropped when
// the terminal is too narrow to hold every column. A dropRank of zero marks a
// column that is never dropped; higher ranks are dropped first.
type repoColSpec struct {
	col      repoColumn
	header   string
	minWidth int
	weight   int
	dropRank int
}

// repoColSpecs lists the repo table's columns in render order.
//
//nolint:mnd // the numbers are this table's data: column geometry, not constants used elsewhere
var repoColSpecs = []repoColSpec{
	{col: colName, header: "NAME", minWidth: 12, weight: 4},
	{col: colBranch, header: "BRANCH", minWidth: 10, weight: 3},
	{col: colStatus, header: "STATUS", minWidth: 12},
	{col: colPeers, header: "PEERS", minWidth: 5, dropRank: 3},
	{col: colPR, header: "PR", minWidth: 8, weight: 2, dropRank: 2},
	{col: colPRs, header: "PRs", minWidth: 6, dropRank: 4},
	{col: colTemplate, header: "TEMPLATE", minWidth: 10, weight: 1, dropRank: 5},
	{col: colModified, header: "MODIFIED", minWidth: 12, dropRank: 1},
}

// repoLayout is a resolved repo table layout: the columns that fit, in render
// order, each with its final width.
type repoLayout struct {
	cols   []repoColSpec
	widths map[repoColumn]int
}

// width returns the resolved width of col, or zero when it was dropped.
func (l repoLayout) width(col repoColumn) int {
	return l.widths[col]
}

// layoutRepoCols fits the repo table into width. Columns are dropped by
// descending dropRank until the remaining minimums fit, then the surplus is
// shared between the flexible columns in proportion to their weight.
func layoutRepoCols(width int) repoLayout {
	cols := fittingCols(width)

	widths := make(map[repoColumn]int, len(cols))
	totalWeight := 0
	for _, c := range cols {
		widths[c.col] = c.minWidth
		totalWeight += c.weight
	}

	surplus := width - minRowWidth(cols)
	if surplus > 0 && totalWeight > 0 {
		spent := 0
		for _, c := range cols {
			if c.weight == 0 {
				continue
			}
			share := surplus * c.weight / totalWeight
			widths[c.col] += share
			spent += share
		}
		widths[colName] += surplus - spent
	}

	return repoLayout{cols: cols, widths: widths}
}

// fittingCols drops the most expendable columns until the remaining minimum
// widths fit within width.
func fittingCols(width int) []repoColSpec {
	cols := make([]repoColSpec, len(repoColSpecs))
	copy(cols, repoColSpecs)

	for minRowWidth(cols) > width {
		victim, rank := -1, 0
		for i, c := range cols {
			if c.dropRank > rank {
				victim, rank = i, c.dropRank
			}
		}
		if victim < 0 {
			break
		}
		cols = append(cols[:victim], cols[victim+1:]...)
	}

	return cols
}

// minRowWidth returns the narrowest row that can hold cols: the cursor, every
// column at its minimum, and a gutter between each pair.
func minRowWidth(cols []repoColSpec) int {
	total := cursorWidth
	for i, c := range cols {
		if i > 0 {
			total += colGutter
		}
		total += c.minWidth
	}

	return total
}
