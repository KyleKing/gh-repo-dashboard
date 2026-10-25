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

	return identity + "\n" + m.compactNeedsLine(s, n)
}

func (m Model) compactNeedsLine(s models.RepoSummary, n needs) string {
	width := listWidth(m.width) - len(compactIndent)
	age := ""

	if !s.LastModified.IsZero() {
		age = display.RepoRelativeModified(s.RepoSummary)
	}

	if m.summaryPending(s.Path) {
		return compactIndent + styles.Skin.Muted.Render(pendingGlyph)
	}

	if len(n.tokens()) == 0 && !n.ciPending {
		return compactIndent + styles.Skin.Muted.Render(table.Truncate(age, width))
	}

	room := width - lipgloss.Width(age) - len(needsGap)
	cell := m.renderNeeds(n, max(room, 0), plainStyle, false)

	return compactIndent + strings.TrimRight(cell, " ") + needsGap + styles.Skin.Muted.Render(age)
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}
