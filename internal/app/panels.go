package app

import (
	"strings"

	"github.com/kyleking/aragonite/tui/preview"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// panelID identifies one section of the focused repo view. Every section is on
// screen at once, so nothing about a repo is hidden behind a mode.
type panelID int

// panelStatus is the repo itself rather than a section: it has no rule or rows
// and is reached by its jump key, where the detail pane describes the repo and
// the verb menu offers the repo-wide actions.
const (
	panelStatus panelID = iota
	panelBranches
	panelPeers
	panelStashes
	panelNotes
	panelPRs
	panelCI
)

// panelKeys are the jump keys. Each is fixed to its panel rather than to a
// grid position, so hiding an empty panel never moves another panel's key out
// from under the fingers.
var panelKeys = map[panelID]string{
	panelBranches: "b",
	panelCI:       "c",
	panelNotes:    "n",
	panelPeers:    "e",
	panelPRs:      "p",
	panelStashes:  "t",
	panelStatus:   "s",
}

// panelOrder is the order sections stack in, which the legend and the jump-key
// list follow. PRs and CI lead because they are what waits on someone.
var panelOrder = []panelID{panelPRs, panelCI, panelBranches, panelPeers, panelStashes, panelNotes, panelStatus}

// panelTitles names each panel for the legend, which cannot ask a panel built
// against a repo.
var panelTitles = map[panelID]string{
	panelStatus:   tabNameStatus,
	panelBranches: tabNameBranches,
	panelCI:       tabNameCI,
	panelPeers:    tabNamePeers,
	panelPRs:      tabNamePRs,
	panelStashes:  tabNameStashes,
	panelNotes:    tabNameNotes,
}

// panelJumpKeys lists every panel's jump key in grid order.
func panelJumpKeys() string {
	keys := make([]string, 0, len(panelOrder))
	for _, id := range panelOrder {
		keys = append(keys, panelKeys[id])
	}

	return strings.Join(keys, "/")
}

// panelContent is one panel resolved against the current repo: the section the
// shared layout draws, its jump key id, and the title the legend and verb
// menu name it by where Heading labels the rule.
type panelContent struct {
	preview.Section
	id    panelID
	title string
}

// previewStyles is the app's skin as the pane faces.
var previewStyles = preview.SkinStyles(styles.Skin)

// sectionsOf resolves panels as the shared layout's sections.
func sectionsOf(panels []panelContent) []preview.Section {
	sections := make([]preview.Section, len(panels))
	for i, p := range panels {
		sections[i] = p.Section
	}

	return sections
}

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

	peerPanelSpecs = []table.Column{
		{Key: colCheckoutName, Title: colCheckoutName, Min: 12, Weight: 2},
		{Key: colCheckoutKind, Title: colCheckoutKind, Min: 8, Priority: 1},
		{Key: colCheckoutBranch, Title: colCheckoutBranch, Min: 12, Weight: 1, Priority: 2},
	}

	prPanelSpecs = []table.Column{
		{Key: colPRTitle, Title: colPRTitle, Min: 16, Weight: 3},
		{Key: colPRState, Title: colPRState, Min: 6},
	}

	ciPanelSpecs = []table.Column{
		{Key: colCIName, Title: colCIName, Min: 14, Weight: 3},
		{Key: colCIState, Title: colCIState, Min: 10, Priority: 1},
		{Key: colCIAge, Title: colCIAge, Min: 6, Priority: 2},
	}

	stashPanelSpecs = []table.Column{
		{Key: colStashMessage, Title: colStashMessage, Min: 16, Weight: 3},
		{Key: colStashDate, Title: colStashDate, Min: 10, Priority: 1},
	}
)

func fitPanelCols(specs []table.Column, width int) table.Layout {
	return table.FitCompact(specs, max(width-gutterWidth, minPanelTableWidth))
}

// minPanelTableWidth keeps a table from collapsing to nothing in a pane too
// narrow for even its mandatory columns; the row overflows instead.
const minPanelTableWidth = 20
