package app

import (
	"strings"

	"charm.land/lipgloss/v2"
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

// panelContent is one panel resolved against the current repo: its title, the
// lines it would show given unlimited room, and how much of the available
// height it has earned.
type panelContent struct {
	id    panelID
	key   string
	title string
	// heading is the label on the section's rule, where title names it in the
	// legend and the verb menu.
	heading string
	count   int
	// relevance drives height: actionable state scores high, a clean or empty
	// set scores the minimum, so a busy repo arrives with its problems open.
	relevance int
	rows      []string
	// selectable is false for Status, which reports state rather than listing
	// items the cursor can land on.
	selectable bool
	// elide cuts the middle of rows too long for the height rather than their
	// end, for text whose newest lines are the last.
	elide bool
}

// Relevance bounds. The scale is arbitrary; only the ratios matter, since
// height is shared in proportion to it.
const (
	relevanceIdle    = 1
	relevancePresent = 3
	relevanceUrgent  = 9
)

// panelRuleHeight is the row a section's labeled rule takes before any of its
// rows fit.
const panelRuleHeight = 1

// panelFloor is the fewest lines a section is drawn in: its rule, plus its
// first row when it has one. An empty section collapses to the rule alone.
func panelFloor(p *panelContent) int {
	return panelRuleHeight + min(len(p.rows), 1)
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

// distributePanelHeights shares available lines between the sections. Every
// section keeps its rule plus its first row so none is ever fully hidden, every
// section that wants more gets up to an equal share before relevance is
// consulted so a low-scoring list is never starved beside a high-scoring one,
// the focused section is served next so its selection stays visible, and what
// remains is split in proportion to relevance.
func distributePanelHeights(panels []panelContent, focused, available int) []int {
	heights := make([]int, len(panels))
	if len(panels) == 0 {
		return heights
	}

	floors := 0
	for i := range panels {
		heights[i] = panelFloor(&panels[i])
		floors += heights[i]
	}

	surplus := available - floors
	if surplus <= 0 {
		return heights
	}

	surplus = grantFairShares(heights, panels, available, surplus)
	surplus = grantWant(heights, panels, focused, surplus)
	shareByRelevance(heights, panels, surplus)

	return heights
}

// panelWant is how many lines beyond its floor a section would use given room.
func panelWant(p *panelContent) int {
	return len(p.rows) - min(len(p.rows), 1)
}

// panelRoom is how many more lines a section would still take at its height.
func panelRoom(p *panelContent, height int) int {
	return panelWant(p) - (height - panelFloor(p))
}

// grantFairShares gives every section that wants them the lines an even split
// would allow, before relevance is consulted at all. This is what keeps
// relevance from being a winner-take-all score.
func grantFairShares(heights []int, panels []panelContent, available, surplus int) int {
	for i := range panels {
		fair := max(available/len(panels)-panelFloor(&panels[i]), 0)
		take := min(panelWant(&panels[i]), fair, surplus)
		heights[i] += take
		surplus -= take
	}

	return surplus
}

// grantWant tops one section up to everything it would show, used for the
// focused section so its selection stays visible.
func grantWant(heights []int, panels []panelContent, i, surplus int) int {
	if i < 0 || i >= len(panels) {
		return surplus
	}

	take := min(max(panelRoom(&panels[i], heights[i]), 0), surplus)
	heights[i] += take

	return surplus - take
}

// shareByRelevance splits what is left between the panels still short of their
// content, in proportion to how much each has to say.
func shareByRelevance(heights []int, panels []panelContent, surplus int) {
	for surplus > 0 {
		weight := 0
		for i := range panels {
			if panelRoom(&panels[i], heights[i]) > 0 {
				weight += panels[i].relevance
			}
		}
		if weight == 0 {
			return
		}

		spent := 0
		for i := range panels {
			room := panelRoom(&panels[i], heights[i])
			if room <= 0 {
				continue
			}

			share := min(max(surplus*panels[i].relevance/weight, 1), room, surplus-spent)
			heights[i] += share
			spent += share
		}

		if spent == 0 {
			return
		}
		surplus -= spent
	}
}

// panelRule is the labeled rule that opens a section. The accent runs the whole
// rule while the section column holds the keyboard in it, and the jump key sits
// at the right end where the eye already is.
func panelRule(p *panelContent, count string, focused bool, width int) string {
	label := p.heading
	if p.selectable {
		label += compactSignalSep + count
	}

	face, line := styles.Skin.Heading, styles.Skin.Hairline
	if focused {
		face, line = styles.Skin.Accent.Bold(true), styles.Skin.Accent
	}

	tail := max(width-lipgloss.Width(label)-sectionRuleLead-sectionRuleSpaces-lipgloss.Width(p.key)-1, 0)

	return line.Render(strings.Repeat("─", sectionRuleLead)+" ") + face.Render(label) +
		line.Render(" "+strings.Repeat("─", tail)+" ") + styles.Skin.Muted.Render(p.key)
}
