package app

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// Column keys for the detail panels. Keys double
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

	colCommitHash    = "COMMIT"
	colCommitSubject = "SUBJECT"
	colCommitAuthor  = "AUTHOR"
	colCommitDate    = "DATE"
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

	commitColSpecs = []table.Column{
		{Key: colCommitHash, Title: colCommitHash, Min: 7},
		{Key: colCommitSubject, Title: colCommitSubject, Min: 20, Weight: 3},
		{Key: colCommitAuthor, Title: colCommitAuthor, Min: 10, Priority: 2},
		{Key: colCommitDate, Title: colCommitDate, Min: 12, Priority: 1},
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
	return styles.Skin.Subtitle.Bold(true).Render(strings.Repeat(" ", cursorWidth) + table.Header(layout))
}

// detailRow puts already-styled cells behind the row's two-cell gutter.
func detailRow(cursor string, layout table.Layout, cells []string, face lipgloss.Style) string {
	return cursor + paintRow(layout, cells, face)
}

// paintRow joins already-styled cells with the gaps between them painted in
// face's background, so a selected row reads as one bar. A face with no
// background has nothing to paint, which keeps a long unselected list cheap.
func paintRow(layout table.Layout, cells []string, face lipgloss.Style) string {
	if _, bare := face.GetBackground().(lipgloss.NoColor); bare {
		if marker := layout.Marker(); marker != "" {
			cells = append(slices.Clone(cells), strings.Repeat(" ", lipgloss.Width(marker)))
		}

		return table.Join(cells)
	}

	return table.Row(layout, cells, lipgloss.NewStyle().Background(face.GetBackground()))
}

var plainStyle = lipgloss.NewStyle()

// rowStyleFor is the face a row's text is drawn in, with the selection behind
// it for the row under the cursor.
func rowStyleFor(selected bool) lipgloss.Style {
	return withSelection(styles.Skin.Body, selected)
}

// rowGutter is the two cells every row opens with: the cursor bar and the mark
// that says the row is part of a batch. Both stay two cells wide whether or not
// they are drawn, so a name never moves when the cursor does.
func rowGutter(marked, selected bool) string {
	if !marked && !selected {
		return strings.Repeat(" ", cursorWidth)
	}

	face := rowStyleFor(selected)

	bar := face.Render(" ")
	if selected {
		bar = withSelection(styles.Skin.Cursor, true).Render(skin.CursorBar)
	}

	mark := face.Render(" ")
	if marked {
		mark = withSelection(styles.Skin.Accent, selected).Render(markGlyph)
	}

	return bar + mark
}

// markGlyph is what the gutter's second cell shows for a marked row.
const markGlyph = "•"

func rowCursorFor(selected bool) string {
	return rowGutter(false, selected)
}
