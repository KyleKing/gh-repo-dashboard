package app

import (
	"fmt"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// panelID identifies one data set in the focused repo view. Every panel with
// something to show is on screen at once, so nothing about a repo is hidden
// behind a mode.
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

// panelKeys are the jump keys. Each is fixed to its panel rather than to a
// grid position, so hiding an empty panel never moves another panel's key out
// from under the fingers.
var panelKeys = map[panelID]string{
	panelBranches: "b",
	panelNotes:    "n",
	panelPeers:    "e",
	panelPRs:      "p",
	panelStashes:  "t",
	panelStatus:   "s",
}

// panelAlwaysShown names the panels the grid keeps even with nothing to list.
// Status reports state rather than listing anything, and a repo always has a
// branch, so an empty Branches panel means the load failed rather than that
// there is nothing to see.
func panelAlwaysShown(id panelID) bool {
	return id == panelStatus || id == panelBranches
}

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

	spreadSurplus(heights, panels, focused, surplus)

	return heights
}

// spreadSurplus hands the lines nothing asked for to the focused panel, so the
// column still reaches the bottom of the grid rather than ending ragged beside
// a full-height detail pane. It all goes to one box on purpose: split across
// every panel in proportion to relevance, a quiet repo whose empty panels were
// dropped would carry dozens of blank rows inside two borders, where under the
// cursor the same slack reads as room the list can grow into.
func spreadSurplus(heights []int, panels []panelContent, focused, surplus int) {
	if surplus <= 0 || len(heights) == 0 {
		return
	}

	if focused < 0 || focused >= len(panels) {
		focused = len(heights) - 1
	}

	heights[focused] += surplus
}

// panelTitle labels a panel's border with its jump key and item count. The key
// is bracketed inside the name rather than named in a footer legend, so the
// key sits on the box it opens.
func panelTitle(p *panelContent, focused bool) string {
	label := markHotkey(p.title, p.key)
	if p.selectable {
		label += fmt.Sprintf(" (%d)", p.count)
	}

	if focused {
		return styles.FooterKeyStyle.Render(label)
	}

	return styles.SubtitleStyle.Render(label)
}

// markHotkey brackets the letter key names inside title, matching without
// regard to case so a lowercase key can mark an uppercase initial ("PRs" for
// "p"). A key the title does not contain is prefixed instead.
func markHotkey(title, key string) string {
	if key == "" {
		return title
	}

	if i := strings.Index(strings.ToLower(title), strings.ToLower(key)); i >= 0 {
		return title[:i] + "[" + title[i:i+len(key)] + "]" + title[i+len(key):]
	}

	return "[" + key + "] " + title
}
