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
// overviewEmpty is what a panel with nothing to list shows. The overview's own
// rows use emDash instead: a column of dashes reads as "nothing here" at a
// glance, where a column of the word "none" has to be read.
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
func (m Model) renderOverview(s models.RepoSummary, opts overviewOpts) string {
	width := opts.width
	rows := m.overviewRows(s, opts.compact)

	lines := make([]string, 0, len(rows)+overviewHeaderLines)

	if opts.standalone {
		lines = append(lines,
			styles.HeaderStyle.Render(table.Truncate(s.Name(), width)),
			styles.SubtitleStyle.Render(table.Truncate(overviewIdentity(s), width)),
			"",
			styles.BranchStyle.Render(table.Truncate(s.Branch, width))+" "+
				styles.SubtitleStyle.Render(table.Truncate(s.StatusSummary(), width)),
			"",
		)
	}

	for _, row := range rows {
		lines = append(lines,
			styles.SubtitleStyle.Render(table.Pad(row.label, overviewLabelCol, table.AlignLeft))+
				" "+table.Truncate(row.value, width-overviewLabelCol-1))
	}

	return strings.Join(lines, "\n")
}

// overviewHeaderLines is how many lines the pane's identity block occupies
// above the label/value rows.
const overviewHeaderLines = 5

// overviewOpts configures one mount of the overview pane. The standalone field
// means the pane carries its own identity block, which the wide preview panel
// needs (it sits beside a list, not under a breadcrumb) and the focused view
// does not.
type overviewOpts struct {
	width      int
	compact    bool
	standalone bool
}

// overviewFullRows is how many rows the non-compact pane holds.
const overviewFullRows = 8

// Row and panel labels shared between the overview pane and the panel grid, so
// a rename cannot leave the two disagreeing.
const (
	rowNameSync     = "Sync"
	rowNameFiles    = "Files"
	tabNameBranches = "Branches"
	tabNameStashes  = "Stashes"
	tabNamePRs      = "PRs"
	tabNameNotes    = "Notes"
)

type overviewRow struct {
	label string
	value string
}

func overviewIdentity(s models.RepoSummary) string {
	parts := []string{s.VCSType.String()}
	if s.RemoteProtocol != "" {
		parts = append(parts, s.RemoteProtocol)
	}

	return strings.Join(parts, compactSignalSep)
}

// overviewRows builds the panel's body in a fixed order, so the same line
// always sits at the same height whatever the repo's state. Each row
// summarizes one panel, so the pane doubles as a table of contents.
//
// The compact layout keeps only Sync and Files: at that width the rest costs
// more rows than the answers are worth.
// Each row names what it is waiting on rather than what it would say if the
// answer were in: a row read from the repo's own summary waits on that summary,
// Peers waits on every summary because a peer is another repo, and the rows
// with a fetch of their own wait on that fetch as well.
func (m Model) overviewRows(s models.RepoSummary, compact bool) []overviewRow {
	unread := m.summaryPending(s.Path)

	rows := make([]overviewRow, 0, overviewFullRows)
	rows = append(rows,
		overviewRow{label: rowNameSync, value: section(unread, overviewSync(s))},
		overviewRow{label: rowNameFiles, value: section(unread, overviewFiles(s))},
	)

	if compact {
		return rows
	}

	return append(rows,
		overviewRow{label: "Peers", value: section(m.loading, overviewPeers(m.PeerCheckouts(s.Path)))},
		overviewRow{label: tabNameStashes, value: section(unread, overviewStashes(s))},
		overviewRow{label: tabNameNotes, value: section(unread, overviewNotes(s.NotesFiles))},
		overviewRow{
			label: "Template",
			value: section(unread || m.fetchPending(s.Path, fetchTemplate), formatCopierCell(s, overviewMaxWidth)),
		},
		overviewRow{label: tabNamePRs, value: section(unread || m.fetchPending(s.Path, fetchPR), overviewPRs(s))},
		overviewRow{label: "CI", value: section(unread || m.fetchPending(s.Path, fetchCI), m.overviewCI(s))},
	)
}

// overviewCI reports the default branch's CI rollup, naming the branch it
// belongs to so the line is not mistaken for the current branch's checks.
func (m Model) overviewCI(s models.RepoSummary) string {
	text, _ := m.ciCell(s, plainStyle, false)

	if branch := m.ciBranch[s.Path]; branch != "" {
		return text + " on " + branch
	}

	return text
}

// overviewSync reports the branch's position against its upstream. With no
// upstream there is nothing to be in sync with, so the row says so rather than
// claiming a position against a remote that does not exist.
func overviewSync(s models.RepoSummary) string {
	switch {
	case s.NoCommits:
		return "no commits"
	case s.Upstream == "":
		return "no upstream"
	}

	position := "in sync"
	switch {
	case s.Ahead > 0 && s.Behind > 0:
		position = "↑" + strconv.Itoa(s.Ahead) + " ↓" + strconv.Itoa(s.Behind)
	case s.Ahead > 0:
		position = "↑" + strconv.Itoa(s.Ahead)
	case s.Behind > 0:
		position = "↓" + strconv.Itoa(s.Behind)
	}

	return position + " vs " + s.Upstream
}

// overviewFiles reports the working tree alone. Unpushed commits belong to the
// Sync row, so a repo that is only ahead reads as clean here.
func overviewFiles(s models.RepoSummary) string {
	if s.UncommittedCount() == 0 {
		return "clean"
	}

	counts := []struct {
		count int
		label string
	}{
		{s.Staged, "staged"},
		{s.Unstaged, models.ModifiedFilesLabel(s.VCSType)},
		{s.Untracked, "untracked"},
		{s.Conflicted, "conflicted"},
	}

	parts := make([]string, 0, len(counts))
	for _, c := range counts {
		if c.count > 0 {
			parts = append(parts, strconv.Itoa(c.count)+" "+c.label)
		}
	}

	return strings.Join(parts, compactSignalSep)
}

// overviewStashes counts stashes, naming jj's absence of them rather than
// reporting zero as though the repo simply had none.
func overviewStashes(s models.RepoSummary) string {
	if s.VCSType == models.VCSTypeJJ {
		return "n/a"
	}

	return overviewCount(s.StashCount)
}

// overviewPRs reports the pull request open on the current branch.
func overviewPRs(s models.RepoSummary) string {
	if s.PRInfo == nil {
		return emDash
	}

	return formatPRCell(s) + " " + s.PRInfo.Title
}

func overviewPeers(peers []models.PeerCheckout) string {
	if len(peers) == 0 {
		return emDash
	}

	folders := make([]string, 0, len(peers))
	for _, peer := range peers {
		folders = append(folders, peer.Folder())
	}

	return "⧉ " + strconv.Itoa(len(peers)) + " " + strings.Join(folders, ", ")
}

func overviewCount(count int) string {
	if count == 0 {
		return emDash
	}

	return strconv.Itoa(count)
}

func overviewNotes(notes []models.NoteFile) string {
	if len(notes) == 0 {
		return emDash
	}

	// One note is the common case, and its first line says more than its
	// filename does; several fall back to naming them.
	if len(notes) == 1 && notes[0].FirstLine != "" {
		return notes[0].Name + ": " + notes[0].FirstLine
	}

	names := make([]string, 0, len(notes))
	for _, note := range notes {
		names = append(names, note.Name)
	}

	return strings.Join(names, ", ")
}

// joinListAndPanel places the preview panel to the right of the repo list,
// separated by a vertical rule that runs the full height of the body whatever
// either block contains.
func joinListAndPanel(list, panel string, listW, panelW, height int) string {
	listLines := strings.Split(list, "\n")
	panelLines := strings.Split(panel, "\n")

	rule := styles.SubtitleStyle.Render(overviewSeparator)

	joined := make([]string, height)
	for i := range height {
		joined[i] = padRight(lineAt(listLines, i), listW) + rule + padRight(lineAt(panelLines, i), panelW)
	}

	return strings.Join(joined, "\n")
}

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
