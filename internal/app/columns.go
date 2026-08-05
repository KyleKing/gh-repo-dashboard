package app

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
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

	stashColSpecs = []table.Column{
		{Key: colStashIndex, Title: colStashIndex, Min: 9},
		{Key: colStashMessage, Title: colStashMessage, Min: 20, Weight: 1},
		{Key: colStashDate, Title: colStashDate, Min: 12, Priority: 1},
	}

	checkoutColSpecs = []table.Column{
		{Key: colCheckoutName, Title: colCheckoutName, Min: 16, Weight: 2},
		{Key: colCheckoutKind, Title: colCheckoutKind, Min: 8, Priority: 2},
		{Key: colCheckoutBranch, Title: colCheckoutBranch, Min: 14, Weight: 1},
		{Key: colCheckoutState, Title: colCheckoutState, Min: 10},
		{Key: colCheckoutCommit, Title: colCheckoutCommit, Min: 12, Priority: 1},
	}

	prColSpecs = []table.Column{
		{Key: colPRNumber, Title: colPRNumber, Min: 7},
		{Key: colPRTitle, Title: colPRTitle, Min: 20, Weight: 3},
		{Key: colPRState, Title: colPRState, Min: 10, Priority: 4},
		{Key: colChecks, Title: colChecks, Min: 12, Priority: 3},
		{Key: colPRActivity, Title: colPRActivity, Min: 20, Weight: 1, Priority: 1},
		{Key: colPRBranch, Title: colPRBranch, Min: 14, Weight: 1, Priority: 2},
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

		cells = append(cells, style.Render(padCell(values[c.Key], layout.Width(c.Key))))
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
