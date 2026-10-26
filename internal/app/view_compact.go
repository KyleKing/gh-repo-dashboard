package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

const compactRowHeight = 2

const compactSignalSep = " · "

const compactIndent = "    "

const (
	colCompactName   = "NAME"
	colCompactBranch = "BRANCH"
	colCompactPR     = "PRs"
)

// compactColSpecs lays out the identity line. Nothing collapses here: three
// columns always fit the widths this layout serves.
//
//nolint:mnd // the numbers are this layout's geometry, not constants used elsewhere
var compactColSpecs = []table.Column{
	{Key: colCompactName, Title: colCompactName, Min: 20, Weight: 3},
	{Key: colCompactBranch, Title: colCompactBranch, Min: 12, Weight: 2},
	{Key: colCompactPR, Title: colCompactPR, Min: 12},
}

// renderCompactRow renders one repo as two lines: identity, then its NEEDS tokens.
func (m Model) renderCompactRow(s models.RepoSummary, n needs, selected bool, layout table.Layout) string {
	base := rowStyleFor(selected)
	cells := []string{
		nameStyle(n, base, selected).Render(padCell(s.Name(), layout.Width(colCompactName))),
		withSelection(m.branchStyle(s), selected).Render(padCell(s.Branch, layout.Width(colCompactBranch))),
		m.prsCell(n, layout.Width(colCompactPR), base, selected),
	}
	identity := rowGutter(m.isMarked(s.Path), selected) + paintRow(layout, cells, base)

	return identity + "\n" + m.compactNeedsLine(s, n, selected)
}

// compactNeedsLine is the record's second line. The selected row's tint runs
// across both lines, so it is padded to the table's width.
func (m Model) compactNeedsLine(s models.RepoSummary, n needs, selected bool) string {
	base := rowStyleFor(selected)
	width := listWidth(m.width) - len(compactIndent)
	age := ""

	if !s.LastModified.IsZero() {
		age = display.RepoRelativeModified(s.RepoSummary)
	}

	muted := withSelection(styles.Skin.Muted, selected)
	line := base.Render(compactIndent)

	fill := plainStyle
	if selected {
		fill = base
	}

	switch {
	case m.summaryPending(s.Path):
		line += muted.Render(pendingGlyph)
	case len(n.tokens()) == 0 && !n.ciPending:
		line += muted.Render(table.Truncate(age, width))
	default:
		room := width - lipgloss.Width(age) - len(needsGap)
		line += m.renderNeeds(n, max(room, 0), fill, selected) + fill.Render(needsGap) + muted.Render(age)
	}

	if !selected {
		return line
	}

	return line + base.Render(strings.Repeat(" ", max(listWidth(m.width)+gutterWidth-lipgloss.Width(line), 0)))
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}
