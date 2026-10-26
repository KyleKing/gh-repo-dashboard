package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/overlay"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// emDash is the placeholder rendered for empty/unknown values.
const emDash = "—"

// mainBranchName and featureBranchName are the conventional branch names used in test fixtures.
const (
	mainBranchName    = "main"
	featureBranchName = "feature"
)

// Layout constants for the detail panes and modals. Table column geometry
// lives in columns.go and is resolved at render time by the column engine.
const (
	notesMarkerWidth       = 2
	visibleWindowCenter    = 2
	helpKeyColWidth        = 20
	descriptionTruncLen    = 60
	detailLabelWidth       = 18
	detailLabelWidthPR     = 16
	prBodyMaxLines         = 80
	prChecksMaxRows        = 12
	prCheckNameMinWidth    = 12
	prCommentMaxLines      = 24
	diffMaxLines           = 500
	emptyStateVPad         = 2
	emptyStateHPad         = 4
	infoPaddingLeft        = 2
	branchDetailMaxCommits = 10
	notesSeparatorWidth    = 40
	confirmModalVPad       = 1
	confirmModalHPad       = 3
)

// View renders the TUI for the current model state.
func (m Model) View() tea.View {
	v := tea.NewView(m.renderScreen())
	v.AltScreen = true

	return v
}

// withSelection paints the shared selection background behind s when selected
// is true.
func withSelection(s lipgloss.Style, selected bool) lipgloss.Style {
	if selected {
		return styles.Selected(s)
	}

	return s
}

// centerModal centers content in the frame's body as a single block, and
// clips it rather than letting an overlarge modal push itself off the screen.
// The modals here carry their own border, so the overlay adds no frame of its
// own.
func centerModal(m Model, content string) string {
	return overlay.Center(content, m.width, m.bodyHeight(), overlay.Styles{})
}

// truncate shortens s to at most maxLen terminal cells, marking the cut with
// an ellipsis when there is room for one.
func truncate(s string, maxLen int) string {
	return table.Truncate(s, maxLen)
}
