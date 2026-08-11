package app

import (
	"fmt"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// panelID identifies one data set in the focused repo view. Every panel is
// always on screen, so nothing about a repo is hidden behind a mode.
type panelID int

// Panels in grid order. Status leads because it answers "what is the state of
// the repo I am standing in" before any list does.
const (
	panelStatus panelID = iota
	panelBranches
	panelPRs
	panelPeers
	panelStashes
	panelNotes
)

// panelContent is one panel resolved against the current repo: its title, the
// lines it would show given unlimited room, and how much of the available
// height it has earned.
type panelContent struct {
	id    panelID
	key   string
	title string
	count int
	// relevance drives height: actionable state scores high, a clean or empty
	// set scores the minimum, so a busy repo arrives with its problems open.
	relevance int
	rows      []string
	// selectable is false for Status, which reports state rather than listing
	// items the cursor can land on.
	selectable bool
}

// Relevance bounds. The scale is arbitrary; only the ratios matter, since
// height is shared in proportion to it.
const (
	relevanceIdle    = 1
	relevancePresent = 3
	relevanceUrgent  = 9
)

// A panel spends panelChromeHeight on its border and panelTitleHeight on its
// title before a single row fits, so panelRowsHeight is what a box of a given
// outer height has left for content.
const (
	panelChromeHeight = 2
	panelTitleHeight  = 1
)

func panelRowsHeight(height int) int {
	return height - panelChromeHeight - panelTitleHeight
}

// panelMinHeight is the shortest a panel can be drawn: its chrome plus one row.
const panelMinHeight = panelChromeHeight + panelTitleHeight + 1

// Panel tables are narrower than the full-width tab tables were, so each one
// carries only the columns that survive a half-width pane.
//
//nolint:mnd // the numbers are each panel's geometry, not constants reused elsewhere
var (
	branchPanelSpecs = []table.Column{
		{Key: colBranchName, Title: colBranchName, Min: 12, Weight: 3},
		{Key: colBranchState, Title: colBranchState, Min: 6},
		{Key: colBranchPR, Title: colBranchPR, Min: 8, Priority: 3},
		{Key: colChecks, Title: colChecks, Min: 12, Priority: 2},
		{Key: colCheckedOut, Title: colCheckedOut, Min: 14, Weight: 1, Priority: 1},
		{Key: colLastCommit, Title: colLastCommit, Min: 10, Weight: 1, Priority: 4},
	}

	prPanelSpecs = []table.Column{
		{Key: colPRNumber, Title: colPRNumber, Min: 5},
		{Key: colPRTitle, Title: colPRTitle, Min: 16, Weight: 3},
		{Key: colChecks, Title: colChecks, Min: 10, Priority: 2},
		{Key: colPRActivity, Title: colPRActivity, Min: 16, Weight: 1, Priority: 1},
		{Key: colPRState, Title: colPRState, Min: 7, Priority: 3},
	}

	peerPanelSpecs = []table.Column{
		{Key: colCheckoutName, Title: colCheckoutName, Min: 12, Weight: 2},
		{Key: colCheckoutKind, Title: colCheckoutKind, Min: 8, Priority: 1},
		{Key: colCheckoutBranch, Title: colCheckoutBranch, Min: 12, Weight: 1},
	}

	stashPanelSpecs = []table.Column{
		{Key: colStashMessage, Title: colStashMessage, Min: 16, Weight: 3},
		{Key: colStashDate, Title: colStashDate, Min: 10, Priority: 1},
	}
)

func fitPanelCols(specs []table.Column, width int) table.Layout {
	return table.Fit(specs, max(width-cursorWidth, minPanelTableWidth))
}

// minPanelTableWidth keeps a table from collapsing to nothing in a pane too
// narrow for even its mandatory columns; the row overflows instead.
const minPanelTableWidth = 20

// distributePanelHeights shares available lines between the panels. Every
// panel keeps its border plus one content line so none is ever fully hidden,
// the focused panel is served next so its selection stays visible, and what
// remains is split in proportion to relevance.
func distributePanelHeights(panels []panelContent, focused, available int) []int {
	heights := make([]int, len(panels))
	if len(panels) == 0 {
		return heights
	}

	minimum := panelMinHeight
	for i := range heights {
		heights[i] = minimum
	}

	surplus := available - minimum*len(panels)
	if surplus <= 0 {
		return heights
	}

	want := func(i int) int { return max(len(panels[i].rows)-1, 0) }

	if focused >= 0 && focused < len(panels) {
		take := min(want(focused), surplus)
		heights[focused] += take
		surplus -= take
	}

	for surplus > 0 {
		weight := 0
		for i := range panels {
			if want(i) > heights[i]-minimum {
				weight += panels[i].relevance
			}
		}
		if weight == 0 {
			break
		}

		spent := 0
		for i := range panels {
			room := want(i) - (heights[i] - minimum)
			if room <= 0 {
				continue
			}

			share := min(max(surplus*panels[i].relevance/weight, 1), room, surplus-spent)
			heights[i] += share
			spent += share
		}

		if spent == 0 {
			break
		}
		surplus -= spent
	}

	spreadSurplus(heights, panels, surplus)

	return heights
}

// spreadSurplus hands out the lines nothing asked for, in proportion to
// relevance, so the panel column reaches the bottom of the grid instead of
// ending ragged beside a full-height detail pane.
func spreadSurplus(heights []int, panels []panelContent, surplus int) {
	if surplus <= 0 {
		return
	}

	weight := 0
	for i := range panels {
		weight += panels[i].relevance
	}

	if weight > 0 {
		for i := range panels {
			share := min(surplus*panels[i].relevance/weight, surplus)
			heights[i] += share
			surplus -= share
		}
	}

	for i := 0; surplus > 0; i = (i + 1) % len(heights) {
		heights[i]++
		surplus--
	}
}

// panelTitle labels a panel's border with its jump key and item count.
func panelTitle(p *panelContent, focused bool) string {
	label := p.key + " " + p.title
	if p.selectable {
		label += fmt.Sprintf(" (%d)", p.count)
	}

	if focused {
		return styles.FooterKeyStyle.Render(label)
	}

	return styles.SubtitleStyle.Render(label)
}
