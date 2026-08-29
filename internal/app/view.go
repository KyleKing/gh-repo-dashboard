package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	modalMarkColWidth      = 4
	modalKeyColWidth       = 3
	filterLabelColWidth    = 15
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
	panelActionModalWidth  = 56
	// A modal spends panelActionModalFrame on its border and padding before any
	// text fits, which is what keeps it inside a narrow terminal.
	panelActionModalFrame = 2*confirmModalHPad + 2
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

	frameW := m.frameWidth()

	content := m.renderView()
	if !m.selfCentering() {
		content = frame(content, m.width, frameW)
	}

	if m.commandMode {
		line := frame(m.commandInput.View(), m.width, frameW)
		if candidates := m.renderCompletionCandidates(frameW); candidates != "" {
			line = frame(candidates, m.width, frameW) + "\n" + line
		}

		return overlayBottomLine(content, line, m.height)
	}
	if m.statusMessage != "" {
		line := frame(styles.StatusMessageStyle.Render(m.statusMessage), m.width, frameW)

		return overlayBottomLine(content, line, m.height)
	}

	return content
}

// frameWidth is the width the frame centers and pads to. The repo list and the
// focused grid are the dense views and share the later cap; everything else
// stays within the single-column content width.
func (m Model) frameWidth() int {
	switch m.viewMode {
	case ViewModeRepoList, ViewModePRList, ViewModePRMap, ViewModeFilter, ViewModeSort:
		return listWidth(m.width)
	case ViewModeRepoDetail:
		return m.gridWidth()
	default:
		return contentWidth(m.width)
	}
}

// selfCentering reports whether the current view already positions itself
// within the full terminal, in which case the shared frame must not indent it
// a second time.
func (m Model) selfCentering() bool {
	switch m.viewMode {
	case ViewModeConfirm:
		return true
	default:
		return false
	}
}

// overlayBottomLine pins line (itself one line or several) onto the bottom
// rows of content, padding or truncating content to keep the overall height
// stable.
func overlayBottomLine(content, line string, height int) string {
	blockHeight := strings.Count(line, "\n") + 1

	lines := strings.Split(content, "\n")
	if height < blockHeight {
		return content
	}

	room := height - blockHeight

	switch {
	case len(lines) >= room:
		lines = lines[:room]
	default:
		for len(lines) < room {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n") + "\n" + line
}

// maxRenderedCompletions caps how many candidates the command bar lists at
// once. A bare ":" matches every registered command, and a predicate atom's
// prefix can match most of the atom table, so an uncapped list would swallow
// most of the screen rather than sitting as a hint above the command line.
const maxRenderedCompletions = 8

// renderCompletionCandidates renders what the command bar's current text
// could complete to, live as each key lands, so the grammar (command names,
// then that command's own arguments) is visible before Tab is ever pressed
// rather than only revealed by pressing it. Each candidate carries its own
// one-line description underneath its name, the same explanation the full
// help overlay gives it, so the meaning of an atom or a command never has to
// be memorized or looked up separately.
func (m Model) renderCompletionCandidates(width int) string {
	candidates, ok := m.commandCompletionCandidates()
	if !ok || len(candidates) == 0 {
		return ""
	}

	shown, hidden := candidates, 0
	if len(shown) > maxRenderedCompletions {
		hidden = len(shown) - maxRenderedCompletions
		shown = shown[:maxRenderedCompletions]
	}

	lines := make([]string, 0, len(shown)+1)
	for _, c := range shown {
		line := styles.SubtitleStyle.Bold(true).Render(c.Name)
		if c.Description != "" {
			line += styles.SubtitleStyle.Render("  " + c.Description)
		}
		lines = append(lines, table.Truncate(line, width))
	}
	if hidden > 0 {
		lines = append(lines, styles.SubtitleStyle.Render(fmt.Sprintf("+%d more", hidden)))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderView() string {
	switch m.viewMode {
	case ViewModeHelp:
		return m.renderHelp()
	case ViewModePRMap:
		return m.renderPRMap()
	case ViewModePalette:
		return m.renderPalette()
	case ViewModePRList:
		if m.prViewMenu {
			return m.renderPRViewModal()
		}
		if m.panelActions {
			return m.renderActionModal()
		}

		return m.renderPRList()
	case ViewModeRepoDetail:
		return m.renderRepoDetail()
	case ViewModeBranchDetail:
		return m.renderBranchDetail()
	case ViewModePRDetail:
		return m.renderPRDetail()
	case ViewModeFilter, ViewModeSort:
		return m.renderRepoList()
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
