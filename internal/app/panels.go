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
		{Key: colBranchName, Title: colBranchName, Min: 12, Weight: 3, Trim: table.TrimLeft},
		{Key: colBranchState, Title: colBranchState, Min: 6},
		{Key: colBranchPR, Title: colBranchPR, Min: 8, Priority: 3},
		{Key: colChecks, Title: colChecks, Min: 12, Priority: 2},
		{Key: colCheckedOut, Title: colCheckedOut, Min: 14, Weight: 1, Priority: 1},
		{Key: colLastCommit, Title: colLastCommit, Min: 10, Weight: 1, Priority: 4},
	}

	peerPanelSpecs = []table.Column{
		{Key: colCheckoutName, Title: colCheckoutName, Min: 12, Weight: 2},
		{Key: colCheckoutKind, Title: colCheckoutKind, Min: 8, Priority: 1},
		{Key: colCheckoutBranch, Title: colCheckoutBranch, Min: 12, Weight: 1, Priority: 2, Trim: table.TrimLeft},
	}

	stashPanelSpecs = []table.Column{
		{Key: colStashMessage, Title: colStashMessage, Min: 16, Weight: 3},
		{Key: colStashDate, Title: colStashDate, Min: 10, Priority: 1},
	}
)

func fitPanelCols(specs []table.Column, width int) table.Layout {
	return table.FitCompact(specs, max(width-cursorWidth, minPanelTableWidth))
}

// minPanelTableWidth keeps a table from collapsing to nothing in a pane too
// narrow for even its mandatory columns; the row overflows instead.
const minPanelTableWidth = 20

// distributePanelHeights shares available lines between the panels. Every panel
// keeps its border plus one content line so none is ever fully hidden, every
// panel that wants them gets up to an equal share before relevance is consulted
// so a low-scoring list is never starved beside a high-scoring one, the focused
// panel is served next so its selection stays visible, and what remains is
// split in proportion to relevance.
func distributePanelHeights(panels []panelContent, focused, available int) []int {
	heights := make([]int, len(panels))
	if len(panels) == 0 {
		return heights
	}

	for i := range heights {
		heights[i] = panelMinHeight
	}

	surplus := available - panelMinHeight*len(panels)
	if surplus <= 0 {
		return heights
	}

	surplus = grantFairShares(heights, panels, available, surplus)
	surplus = grantWant(heights, panels, focused, surplus)
	shareByRelevance(heights, panels, surplus)

	// Lines nothing asked for stay unspent, so the column ends where its
	// content does. Stretching it to the bottom of the grid only pads boxes
	// with blank rows: every panel that had somewhere to put a line has already
	// been given it by this point.
	return heights
}

// panelWant is how many lines beyond its first a panel would use given room.
func panelWant(p *panelContent) int {
	return max(len(p.rows)-1, 0)
}

// panelRoom is how many more lines a panel would still take at its height.
func panelRoom(p *panelContent, height int) int {
	return panelWant(p) - (height - panelMinHeight)
}

// grantFairShares gives every panel that wants them the lines an even split
// would allow, before relevance is consulted at all. This is what keeps
// relevance from being a winner-take-all score: Peers showing one of six
// checkouts beside a half-empty Branches box was the shape it prevents.
func grantFairShares(heights []int, panels []panelContent, available, surplus int) int {
	fair := max(available/len(panels)-panelMinHeight, 0)

	for i := range panels {
		take := min(panelWant(&panels[i]), fair, surplus)
		heights[i] += take
		surplus -= take
	}

	return surplus
}

// grantWant tops one panel up to everything it would show, used for the focused
// panel so its selection stays visible.
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

// markHotkey brackets the letter key names inside title. The bracketed letter
// is the key as typed, so "Branches" reads "[b]ranches" rather than promising
// a shift the binding does not want. Where recasing would strand an uppercase
// letter mid-word ("PRs"), or the title does not contain the key at all, the
// key is prefixed instead.
func markHotkey(title, key string) string {
	if key == "" {
		return title
	}

	i := strings.Index(strings.ToLower(title), strings.ToLower(key))
	if i < 0 {
		return "[" + key + "] " + title
	}

	matched, rest := title[i:i+len(key)], title[i+len(key):]
	if matched != key && stranded(matched, rest) {
		return "[" + key + "] " + title
	}

	return title[:i] + "[" + key + "]" + rest
}

// stranded reports whether recasing the matched letters would leave the rest of
// the word shouting, which is how an acronym title reads once its initial is
// lowercased ("[p]Rs").
func stranded(matched, rest string) bool {
	word, _, _ := strings.Cut(rest, " ")

	return matched != strings.ToLower(matched) && word != strings.ToLower(word)
}
