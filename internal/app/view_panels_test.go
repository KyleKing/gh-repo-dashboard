//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import "strings"

// renderPanel renders one panel's rows as a standalone block, so a test can
// assert on a single data set without reading it out of the whole grid.
func renderPanel(m Model, id panelID) string {
	m.focusedPanel = id
	for _, p := range m.panelSet(contentWidth(m.width)) {
		if p.id == id {
			return strings.Join(p.rows, "\n")
		}
	}

	return ""
}

// panelRenderer adapts renderPanel to the func(Model) string shape the table
// tests use.
func panelRenderer(id panelID) func(Model) string {
	return func(m Model) string { return renderPanel(m, id) }
}
