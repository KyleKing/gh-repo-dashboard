package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/overlay"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

const appTitle = "repo-dashboard"

// frameChrome is the header's row and the footer's; bodyLead is the blank row
// under the header, and screenEdge the margin each side of every screen.
const (
	frameChrome = 2
	bodyLead    = 1
	screenEdge  = 2
)

type screen struct {
	title string
	facts []frame.Fact
	body  string
	hints []keyhint.Hint
	// panel floats over the bottom-right of the body without moving a line of it.
	panel []string
}

func (m Model) bodyHeight() int {
	return max(m.height-frameChrome-bodyLead, 1)
}

// viewWidth is the width inside the screen's edge margin: what the frame and
// every screen under it render into.
func (m Model) viewWidth() int {
	return max(m.width-2*screenEdge, 1)
}

// bodyBudget is the body's share of the frame: the content rows plus the lead
// row the header's rule keeps above them.
func (m Model) bodyBudget() int {
	return m.bodyHeight() + bodyLead
}

func (m Model) renderScreen() string {
	if m.width == 0 {
		return ""
	}

	s := m.currentScreen()
	body := strings.Split(s.body, "\n")

	if m.commandMode {
		body = m.withCompletions(body)
	}

	body = padLines(append(make([]string, bodyLead), body...), m.bodyBudget(), m.viewWidth())
	if len(s.panel) > 0 {
		body = overlay.BottomRight(body, s.panel, m.viewWidth())
	}

	rendered := frame.Render(styles.Skin, frame.Frame{
		Title:  s.title,
		Facts:  s.facts,
		Tabs:   m.frameTabs(),
		Body:   body,
		Hints:  s.hints,
		Status: statusLine(m.statusMessage),
	}, m.viewWidth(), m.height)

	rendered = indent(rendered, screenEdge)

	if m.commandMode {
		return replaceLastLine(rendered, strings.Repeat(" ", screenEdge)+m.commandLine(), m.width)
	}

	return rendered
}

// indent shifts every line of rendered right by cells, opening the screen's
// left margin. The right margin comes free: the frame drew into the narrower
// content width, so nothing reaches the edge.
func indent(rendered string, cells int) string {
	lead := strings.Repeat(" ", cells)
	lines := strings.Split(rendered, "\n")

	for i := range lines {
		lines[i] = lead + lines[i]
	}

	return strings.Join(lines, "\n")
}

func (m Model) currentScreen() screen {
	s := m.baseScreen()

	if panel := m.whichKeyPanel(); panel != nil {
		s.panel = panel
	}

	if m.panelActions && m.viewMode != ViewModePalette {
		s.hints = []keyhint.Hint{{Key: keyEsc, What: descBack, Priority: 1}}
	}

	return s
}

func (m Model) baseScreen() screen {
	switch m.viewMode {
	case ViewModeHelp:
		return m.helpScreen()
	case ViewModePRMap:
		return m.prMapScreen()
	case ViewModePalette:
		return m.paletteScreen()
	case ViewModeRepoDetail:
		return m.repoDetailScreen()
	case ViewModeBranchDetail:
		return m.branchDetailScreen()
	case ViewModeTaskRun:
		return m.runScreen()
	case ViewModeTasks:
		return m.tasksScreen()
	case ViewModeTaskPreview:
		return m.previewScreen()
	case ViewModeTaskLog:
		return m.logScreen()
	case ViewModeConfirm:
		return m.confirmScreen()
	case ViewModeRepoList, ViewModeFilter, ViewModeSort:
		return m.repoListScreen()
	}

	return m.repoListScreen()
}

// padLines fits lines to height rows and pads each to width cells, because a
// shorter line leaves the previous frame's tail on screen.
func padLines(lines []string, height, width int) []string {
	out := make([]string, height)

	for i := range out {
		if i < len(lines) {
			out[i] = lines[i]
		}

		if gap := width - lipgloss.Width(out[i]); gap > 0 {
			out[i] += strings.Repeat(" ", gap)
		}
	}

	return out
}

func statusLine(message string) string {
	return strings.ReplaceAll(strings.TrimSpace(message), "\n", compactSignalSep)
}

func replaceLastLine(rendered, line string, width int) string {
	lines := strings.Split(rendered, "\n")
	lines[len(lines)-1] = truncate(line, width)

	return strings.Join(lines, "\n")
}

func (m Model) commandLine() string {
	return m.commandInput.View()
}

const maxCompletionRows = 10

func (m Model) withCompletions(body []string) []string {
	candidates, ok := m.commandCompletionCandidates()
	if !ok || len(candidates) == 0 {
		return body
	}

	hints := make([]keyhint.Hint, 0, len(candidates))
	for _, c := range candidates {
		hints = append(hints, keyhint.Hint{Key: c.Name, What: c.Description})
	}

	rows := keyhint.Column(legendStyles(), hints, min(completionBoxWidth, m.viewWidth()-boxMargin))
	inner := max(1, min(len(rows), min(maxCompletionRows, m.bodyHeight()-boxBorder)))
	box := overlay.Box(boxStyles(), "complete", rows, inner, 0)

	padded := padLines(body, m.bodyHeight(), 0)

	return overlay.Blit(padded, box, 0, len(padded)-len(box))
}

const (
	boxBorder          = 2
	boxMargin          = 4
	completionBoxWidth = 96
	helpBoxWidth       = 76
	pastEnd            = 1 << 30
	wheelStep          = 3
	halfPageDivisor    = 2
)

func legendStyles() keyhint.Styles {
	return keyhint.Styles{
		Key:  styles.Skin.Key,
		Text: styles.Skin.Subtitle,
		Head: styles.Skin.Heading,
		Off:  styles.Skin.Muted,
	}
}

func boxStyles() overlay.Styles {
	return overlay.Styles{Frame: styles.Skin.Edge, Elision: styles.Skin.Muted}
}
