package app

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/table"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// Column keys for the detail tabs, PR views, and batch results. Keys double
// as rendered headers except where the header reads better shortened.
const (
	colBranchName  = "BRANCH"
	colUpstream    = "UPSTREAM"
	colBranchState = "STATUS"
	colBranchPR    = "PR"
	colChecks      = "CHECKS"
	colCheckedOut  = "CHECKED OUT"
	colLastCommit  = "LAST COMMIT"

	colStashIndex   = "INDEX"
	colStashMessage = "MESSAGE"
	colStashDate    = "DATE"

	colCheckoutName   = "CHECKOUT"
	colCheckoutKind   = "KIND"
	colCheckoutBranch = "BRANCH"
	colCheckoutState  = "STATUS"
	colCheckoutCommit = "LAST COMMIT"

	colPRNumber   = "NUMBER"
	colPRTitle    = "TITLE"
	colPRState    = "STATE"
	colPRActivity = "ACTIVITY"
	colPRBranch   = "BRANCH"
	colPRAuthor   = "AUTHOR"
	colPRUpdated  = "UPDATED"

	colCheckName     = "CHECK"
	colCheckState    = "STATUS"
	colCheckDuration = "DURATION"

	colCommitHash    = "COMMIT"
	colCommitSubject = "SUBJECT"
	colCommitAuthor  = "AUTHOR"
	colCommitDate    = "DATE"

	colBatchName    = "REPO"
	colBatchMessage = "RESULT"
)

// Collapse priorities below are information value under a narrow terminal, so
// the lowest positive number hides first and zero never hides. On the branch
// tab UPSTREAM goes first because it almost always mirrors the branch name,
// and CHECKED OUT survives longest because a branch checked out elsewhere is
// the hazard that loses commits.
//
//nolint:mnd // the numbers are each table's geometry, not constants reused elsewhere
var (
	branchColSpecs = []table.Column{
		{Key: colBranchName, Title: colBranchName, Min: 14, Weight: 3},
		{Key: colUpstream, Title: colUpstream, Min: 12, Priority: 1},
		{Key: colBranchState, Title: colBranchState, Min: 10},
		{Key: colBranchPR, Title: colBranchPR, Min: 10, Priority: 4},
		{Key: colChecks, Title: colChecks, Min: 12, Priority: 3},
		{Key: colCheckedOut, Title: colCheckedOut, Min: 16, Weight: 1, Priority: 5},
		{Key: colLastCommit, Title: colLastCommit, Min: 12, Priority: 2},
	}

	checkColSpecs = []table.Column{
		{Key: colCheckName, Title: colCheckName, Min: 20, Weight: 3},
		{Key: colCheckState, Title: colCheckState, Min: 12},
		{Key: colCheckDuration, Title: colCheckDuration, Min: 8, Priority: 1},
	}

	commitColSpecs = []table.Column{
		{Key: colCommitHash, Title: colCommitHash, Min: 7},
		{Key: colCommitSubject, Title: colCommitSubject, Min: 20, Weight: 3},
		{Key: colCommitAuthor, Title: colCommitAuthor, Min: 10, Priority: 2},
		{Key: colCommitDate, Title: colCommitDate, Min: 12, Priority: 1},
	}

	batchColSpecs = []table.Column{
		{Key: colBatchName, Title: colBatchName, Min: 16, Weight: 2},
		{Key: colBatchMessage, Title: colBatchMessage, Min: 20, Weight: 3},
	}
)

func fitDetailCols(specs []table.Column, termWidth int) table.Layout {
	return table.Fit(specs, contentWidth(termWidth)-cursorWidth)
}

// renderCells pads and styles every visible column of one row. Columns absent
// from cellStyles fall back to base.
func renderCells(
	layout table.Layout, values map[string]string, cellStyles map[string]lipgloss.Style, base *lipgloss.Style,
) []string {
	cells := make([]string, 0, len(layout.Columns))
	for _, c := range layout.Columns {
		style, ok := cellStyles[c.Key]
		if !ok {
			style = *base
		}

		cells = append(cells,
			style.Render(table.PadTrim(values[c.Key], layout.Width(c.Key), table.AlignLeft, c.Trim)))
	}

	return cells
}

func detailHeader(layout table.Layout) string {
	return styles.HeaderStyle.Render(strings.Repeat(" ", cursorWidth) + table.Header(layout))
}

// detailRow joins already-styled cells behind a two-cell cursor, leaving the
// header's hidden-column marker an empty column to sit above.
func detailRow(cursor string, layout table.Layout, cells []string) string {
	if marker := layout.Marker(); marker != "" {
		cells = append(cells, strings.Repeat(" ", lipgloss.Width(marker)))
	}

	return cursor + table.Join(cells)
}

// joinWithinWidth appends badges to head while they fit in limit display
// cells, dropping the rest rather than letting the line run off the frame.
// Badges are ordered most-important-first by their callers.
func joinWithinWidth(head string, badges []string, limit int) string {
	line := head
	gap := "  "

	for _, badge := range badges {
		candidate := line + gap + badge
		if lipgloss.Width(candidate) > limit {
			break
		}

		line, gap = candidate, " "
	}

	return line
}

var plainStyle = lipgloss.NewStyle()

func rowStyleFor(selected bool) lipgloss.Style {
	if selected {
		return styles.SelectedRowStyle
	}

	return styles.TableRowStyle
}

func rowCursorFor(selected bool) string {
	if selected {
		return "> "
	}

	return "  "
}
