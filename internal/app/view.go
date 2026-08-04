package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
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
	nonListRowHeight       = 6
	visibleWindowCenter    = 2
	helpKeyColWidth        = 20
	modalMarkColWidth      = 4
	modalKeyColWidth       = 3
	filterLabelColWidth    = 15
	descriptionTruncLen    = 60
	detailLabelWidth       = 18
	detailLabelWidthPR     = 16
	prBodyMaxLen           = 400
	prCommentMaxLen        = 240
	statusBarHeight        = 2
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

func (m Model) renderScreen() string {
	if m.width == 0 {
		return ""
	}

	content := m.renderView()
	if !m.selfCentering() {
		content = frame(content, m.width)
	}

	if m.commandMode {
		return overlayBottomLine(content, frame(m.commandInput.View(), m.width), m.height)
	}
	if m.statusMessage != "" {
		line := frame(styles.StatusMessageStyle.Render(m.statusMessage), m.width)

		return overlayBottomLine(content, line, m.height)
	}

	return content
}

// selfCentering reports whether the current view already positions itself
// within the full terminal, in which case the shared frame must not indent it
// a second time.
func (m Model) selfCentering() bool {
	switch m.viewMode {
	case ViewModeFilter, ViewModeSort, ViewModeConfirm:
		return true
	default:
		return false
	}
}

// overlayBottomLine pins line onto the last row of content, padding or
// truncating content to keep the overall height stable.
func overlayBottomLine(content, line string, height int) string {
	lines := strings.Split(content, "\n")
	if height < 1 {
		return content
	}
	switch {
	case len(lines) >= height:
		lines = lines[:height-1]
	default:
		for len(lines) < height-1 {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n") + "\n" + line
}

func (m Model) renderView() string {
	switch m.viewMode {
	case ViewModeHelp:
		return m.renderHelp()
	case ViewModeRepoDetail:
		return m.renderRepoDetail()
	case ViewModeBranchDetail:
		return m.renderBranchDetail()
	case ViewModePRDetail:
		return m.renderPRDetail()
	case ViewModeFilter:
		return m.renderFilterModal()
	case ViewModeSort:
		return m.renderSortModal()
	case ViewModeBatchProgress:
		return m.renderBatchProgress()
	case ViewModeConfirm:
		return m.renderConfirmModal()
	default:
		return m.renderRepoList()
	}
}

// withSelection applies the shared selected-row background to s when selected is true.
func withSelection(s lipgloss.Style, selected bool) lipgloss.Style {
	if selected {
		return s.Background(styles.Surface0)
	}

	return s
}

// centerModal centers content on screen as a single block. Content is first
// left-padded to a uniform width because lipgloss.Place centers each line of
// a multi-line string independently based on that line's own width, which
// would otherwise stagger rows of differing length (e.g. table rows).
func centerModal(m Model, content string) string {
	width := lipgloss.Width(content)
	content = lipgloss.NewStyle().Width(width).Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderBreadcrumbs() string {
	switch m.viewMode {
	case ViewModeRepoDetail:
		return m.renderRepoDetailBreadcrumbs()
	case ViewModeBranchDetail:
		return m.renderBranchDetailBreadcrumbs()
	default:
		return m.renderRepoListBreadcrumbs()
	}
}

// truncate shortens s to at most maxLen terminal cells, marking the cut with
// an ellipsis when there is room for one.
func truncate(s string, maxLen int) string {
	return table.Truncate(s, maxLen)
}
