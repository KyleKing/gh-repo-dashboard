package app

import (
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// placeholderBox is the bordered block both loading and empty states render
// into, so a pane keeps the same footprint once its data arrives. The compact
// layout drops the vertical padding, because four spare rows out of 24 cost
// more than the breathing room is worth.
func placeholderBox(body string, compact bool) string {
	vPad := emptyStateVPad
	if compact {
		vPad = 0
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Surface1).
		Padding(vPad, emptyStateHPad).
		Foreground(styles.Subtext0).
		Render(body)
}

// loadingPlaceholder renders a pane that is still waiting on data, naming the
// work in progress (for example "Loading branches").
func (m *Model) loadingPlaceholder(what string) string {
	return placeholderBox(m.spinner.View()+" "+what+"...", m.isCompact())
}

// emptyPlaceholder renders a settled pane that genuinely holds nothing. The
// hint explains what would fill it, or is empty when the headline says enough.
// Distinct from loadingPlaceholder so a slow fetch never reads as no results.
// The hint is the first thing dropped when the terminal is short on rows.
func (m *Model) emptyPlaceholder(headline, hint string) string {
	if hint == "" || m.isCompact() {
		return placeholderBox(headline, m.isCompact())
	}

	return placeholderBox(headline+"\n\n"+hint, false)
}

// isCompact reports whether the model renders at the compact breakpoint.
func (m *Model) isCompact() bool {
	return breakpointFor(m.width, m.height) == breakpointCompact
}
