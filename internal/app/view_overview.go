package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// Preview panel geometry. The panel only appears once the terminal is wide
// enough to spare the width, and it never grows past overviewMaxWidth because
// beyond that the list loses more than the panel gains.
const (
	overviewMinWidth = 34
	overviewMaxWidth = 48
	overviewSepWidth = 3
	overviewLabelCol = 10
)

// overviewMinListWidth is the width the repo list keeps for itself before any
// is spent on a panel: the standard breakpoint's floor, which is the narrowest
// a full eight-column table reads well at.
const overviewMinListWidth = standardMinWidth

// splitWidth is the total the list, rule, and panel share. It caps at the
// single-column content width plus a full panel, so a very wide terminal
// leaves margin rather than stretching rows past the point of scanning.
func splitWidth(termWidth int) int {
	available := max(termWidth-frameSides*frameGutter, minContentWidth)

	return min(available, maxContentWidth+overviewSepWidth+overviewMaxWidth)
}

// overviewSeparator is the rule drawn between the list and the panel, and
// overviewEmpty is what a section with nothing to report shows.
const (
	overviewSeparator = " │ "
	overviewEmpty     = "none"
)

// panelWidth returns the preview panel's width, or zero when the layout has
// no panel or the terminal cannot spare one without starving the list.
func panelWidth(termWidth, height int) int {
	if breakpointFor(termWidth, height) != breakpointWide {
		return 0
	}

	spare := splitWidth(termWidth) - overviewMinListWidth - overviewSepWidth
	if spare < overviewMinWidth {
		return 0
	}

	return min(spare, overviewMaxWidth)
}

// listWidth returns the width the repo list may render into, which is the full
// content width unless a preview panel is taking part of it.
func listWidth(termWidth, height int) int {
	panel := panelWidth(termWidth, height)
	if panel == 0 {
		return contentWidth(termWidth)
	}

	return splitWidth(termWidth) - panel - overviewSepWidth
}

// frameContentWidth is the width the frame centers and pads to: the list plus
// the panel when one is mounted, and the plain content width otherwise.
func frameContentWidth(termWidth, height int) int {
	panel := panelWidth(termWidth, height)
	if panel == 0 {
		return contentWidth(termWidth)
	}

	return listWidth(termWidth, height) + overviewSepWidth + panel
}

// renderOverview summarizes one repo from cached data only, so moving the
// cursor never blocks on a fetch. Sections whose data has not arrived render
// their placeholder rather than disappearing, keeping the panel's height
// stable as data fills in.
func (m Model) renderOverview(s models.RepoSummary, width int) string {
	rows := m.overviewRows(s)

	lines := make([]string, 0, len(rows)+overviewHeaderLines)
	lines = append(lines,
		styles.TitleStyle.Render(table.Truncate(s.Name(), width)),
		styles.SubtitleStyle.Render(table.Truncate(overviewIdentity(s), width)),
		"",
		styles.BranchStyle.Render(table.Truncate(s.Branch, width))+" "+
			styles.SubtitleStyle.Render(table.Truncate(s.StatusSummary(), width)),
		"",
	)

	for _, row := range rows {
		lines = append(lines,
			styles.SubtitleStyle.Render(table.Pad(row.label, overviewLabelCol, table.AlignLeft))+
				" "+table.Truncate(row.value, width-overviewLabelCol-1))
	}

	return strings.Join(lines, "\n")
}

// overviewHeaderLines is how many lines the panel's identity block occupies
// above the label/value rows.
const overviewHeaderLines = 5

// overviewRow is one label/value line of the panel.
type overviewRow struct {
	label string
	value string
}

// overviewIdentity names how the repo is tracked: its VCS and remote protocol.
func overviewIdentity(s models.RepoSummary) string {
	parts := []string{s.VCSType.String()}
	if s.RemoteProtocol != "" {
		parts = append(parts, s.RemoteProtocol)
	}

	return strings.Join(parts, compactSignalSep)
}

// overviewRows builds the panel's body in a fixed order, so the same line
// always sits at the same height whatever the repo's state.
func (m Model) overviewRows(s models.RepoSummary) []overviewRow {
	return []overviewRow{
		{label: "Peers", value: overviewPeers(m.PeerCheckouts(s.Path))},
		{label: "Stashes", value: overviewCount(s.StashCount)},
		{label: "Notes", value: overviewNotes(s.NotesFiles)},
		{label: "Template", value: formatCopierCell(s, overviewMaxWidth)},
		{label: "PR", value: formatPRCell(s)},
	}
}

func overviewPeers(peers []models.PeerCheckout) string {
	if len(peers) == 0 {
		return overviewEmpty
	}

	folders := make([]string, 0, len(peers))
	for _, peer := range peers {
		folders = append(folders, peer.Folder())
	}

	return "⧉" + strconv.Itoa(len(peers)) + " " + strings.Join(folders, ", ")
}

func overviewCount(count int) string {
	if count == 0 {
		return overviewEmpty
	}

	return strconv.Itoa(count)
}

func overviewNotes(notes []models.NoteFile) string {
	if len(notes) == 0 {
		return overviewEmpty
	}

	names := make([]string, 0, len(notes))
	for _, note := range notes {
		names = append(names, note.Name)
	}

	return strings.Join(names, ", ")
}

// joinListAndPanel places the preview panel to the right of the repo list,
// separated by a full-height vertical rule. Both blocks are padded to the same
// line count so the rule runs unbroken past whichever block is shorter.
func joinListAndPanel(list, panel string, listW, panelW int) string {
	listLines := strings.Split(list, "\n")
	panelLines := strings.Split(panel, "\n")

	height := max(len(listLines), len(panelLines))
	rule := styles.SubtitleStyle.Render(overviewSeparator)

	joined := make([]string, height)
	for i := range height {
		joined[i] = padRight(lineAt(listLines, i), listW) + rule + padRight(lineAt(panelLines, i), panelW)
	}

	return strings.Join(joined, "\n")
}

// lineAt returns lines[i], or an empty line past the end of the block.
func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}

	return ""
}

// padRight widens an already-styled line to width without truncating it,
// because cutting a styled line risks slicing an escape sequence in half.
func padRight(line string, width int) string {
	if gap := width - lipgloss.Width(line); gap > 0 {
		return line + strings.Repeat(" ", gap)
	}

	return line
}
