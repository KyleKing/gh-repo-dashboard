package app

import (
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// placeholderBox is the bordered block both loading and empty states render
// into, so a pane keeps the same footprint once its data arrives.
func placeholderBox(body string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Surface1).
		Padding(emptyStateVPad, emptyStateHPad).
		Foreground(styles.Subtext0).
		Render(body)
}

// loadingPlaceholder renders a pane that is still waiting on data, naming the
// work in progress (for example "Loading branches").
func (m *Model) loadingPlaceholder(what string) string {
	return placeholderBox(m.spinner.View() + " " + what + "...")
}

// emptyPlaceholder renders a settled pane that genuinely holds nothing. The
// hint explains what would fill it, or is empty when the headline says enough.
// Distinct from loadingPlaceholder so a slow fetch never reads as no results.
func emptyPlaceholder(headline, hint string) string {
	if hint == "" {
		return placeholderBox(headline)
	}

	return placeholderBox(headline + "\n\n" + hint)
}
