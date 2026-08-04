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

	colWorktreePath   = "PATH"
	colWorktreeBranch = "BRANCH"
	colWorktreeState  = "STATUS"

	colPRNumber = "NUMBER"
	colPRTitle  = "TITLE"
	colPRState  = "STATE"
	colPRReview = "REVIEW"
	colPRBranch = "BRANCH"

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

	worktreeColSpecs = []table.Column{
		{Key: colWorktreePath, Title: colWorktreePath, Min: 16, Weight: 2},
		{Key: colWorktreeBranch, Title: colWorktreeBranch, Min: 14, Weight: 1},
		{Key: colWorktreeState, Title: colWorktreeState, Min: 8, Priority: 1},
	}

	prColSpecs = []table.Column{
		{Key: colPRNumber, Title: colPRNumber, Min: 7},
		{Key: colPRTitle, Title: colPRTitle, Min: 20, Weight: 3},
		{Key: colPRState, Title: colPRState, Min: 10, Priority: 3},
		{Key: colPRReview, Title: colPRReview, Min: 16, Priority: 1},
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

// fitDetailCols fits a detail-tab table into the content width available at
// termWidth, leaving room for the cursor gutter that leads every row.
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

// detailHeader renders a detail-tab header row, indented past the cursor
// gutter and carrying the marker for any columns collapse hid.
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

// plainStyle is the unstyled base for tables that color individual cells only.
var plainStyle = lipgloss.NewStyle()

// rowStyleFor returns the base style for a table row, highlighted when it
// holds the cursor.
func rowStyleFor(selected bool) lipgloss.Style {
	if selected {
		return styles.SelectedRowStyle
	}

	return styles.TableRowStyle
}

// rowCursorFor returns the two-cell leader marking the selected row.
func rowCursorFor(selected bool) string {
	if selected {
		return "> "
	}

	return "  "
}
