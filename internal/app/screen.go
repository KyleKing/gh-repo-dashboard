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

const frameChrome = 2

type screen struct {
	title string
	facts []frame.Fact
	body  string
	hints []keyhint.Hint
}

func (m Model) bodyHeight() int {
	return max(m.height-frameChrome, 1)
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

	rendered := frame.Render(styles.Skin, frame.Frame{
		Title:  s.title,
		Facts:  s.facts,
		Tabs:   m.frameTabs(),
		Body:   padLines(body, m.bodyHeight(), m.width),
		Hints:  s.hints,
		Status: statusLine(m.statusMessage),
	}, m.width, m.height)

	if m.commandMode {
		return replaceLastLine(rendered, m.commandLine(), m.width)
	}

	return rendered
}

func (m Model) currentScreen() screen {
	if m.panelActions && m.viewMode != ViewModePalette {
		return m.actionScreen()
	}

	return m.baseScreen()
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
	case ViewModeBatchProgress:
		return m.batchScreen()
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

	rows := keyhint.Column(legendStyles(), hints, min(completionBoxWidth, m.width-boxMargin))
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
