package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/table"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

const compactRowHeight = 2

const compactSignalSep = " · "

const compactIndent = "    "

const (
	colCompactName   = "NAME"
	colCompactBranch = "BRANCH"
	colCompactState  = "STATUS"
)

// compactColSpecs lays out the identity line. Nothing collapses here: three
// columns always fit the widths this layout serves.
//
//nolint:mnd // the numbers are this layout's geometry, not constants used elsewhere
var compactColSpecs = []table.Column{
	{Key: colCompactName, Title: colCompactName, Min: 20, Weight: 3},
	{Key: colCompactBranch, Title: colCompactBranch, Min: 12, Weight: 2},
	{Key: colCompactState, Title: colCompactState, Min: 12},
}

// renderCompactRow renders one repo as a two-line record: name, branch, and
// working-tree state on the first line, and whichever fleet signals the repo
// actually has on the second. A repo with no signals still gets the second
// line so the cursor can step by a fixed number of rows.
func (m Model) renderCompactRow(s models.RepoSummary, selected bool, layout table.Layout) string {
	base := rowStyleFor(selected)

	values := map[string]string{
		colCompactName:   s.Name(),
		colCompactBranch: s.Branch,
		colCompactState:  ui.RepoStatusSummary(s.RepoSummary),
	}
	cellStyles := map[string]lipgloss.Style{
		colCompactBranch: withSelection(styles.BranchStyle, selected),
		colCompactState:  withSelection(statusCellStyle(s, base), selected),
	}

	identity := base.Render(rowCursor(m.isMarked(s.Path), selected)) +
		table.Join(renderCells(layout, values, cellStyles, &base))

	signals := m.compactSignals(s)
	signalWidth := listWidth(m.width) - len(compactIndent)
	signalLine := styles.SubtitleStyle.Render(
		compactIndent + table.Truncate(strings.Join(signals, compactSignalSep), signalWidth))

	return identity + "\n" + signalLine
}

// compactSignals lists the non-empty fleet signals for a repo, in the order
// they earn attention: peers first because a shared branch loses commits,
// then review load, template drift, notes, and finally recency.
func (m Model) compactSignals(s models.RepoSummary) []string {
	var signals []string

	if peers := m.PeerCheckouts(s.Path); len(peers) > 0 {
		peerSignal := peerPrefix + strconv.Itoa(len(peers))
		if m.hasBranchConflict(s.Path) {
			peerSignal += " " + conflictMark
		}
		signals = append(signals, peerSignal)
	}

	if s.PRInfo != nil {
		signals = append(signals, formatPRCell(s))
	}

	if count, ok := m.prCount[s.Path]; ok && count > 0 {
		signals = append(signals, strconv.Itoa(count)+" "+plural(count, "PR", "PRs"))
	}

	if s.TemplateInfo != nil {
		signals = append(signals, formatCopierCell(s, listWidth(m.width)))
	}

	if s.StashCount > 0 {
		signals = append(signals, strconv.Itoa(s.StashCount)+" "+plural(s.StashCount, "stash", "stashes"))
	}

	if notes := len(s.NotesFiles); notes > 0 {
		signals = append(signals, strconv.Itoa(notes)+" "+plural(notes, "note", "notes"))
	}

	if s.LastModified.IsZero() {
		if len(signals) == 0 {
			return []string{"nothing to report"}
		}

		return signals
	}

	return append(signals, ui.RepoRelativeModified(s.RepoSummary))
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}
