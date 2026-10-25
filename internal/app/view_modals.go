package app

import (
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func (m Model) confirmScreen() screen {
	if m.pendingAction == nil {
		return m.repoListScreen()
	}

	return screen{
		title: appTitle,
		body:  m.renderConfirmModal(),
		hints: []keyhint.Hint{
			{Key: "y/enter", What: "confirm", Priority: 2},
			{Key: "n/esc", What: "cancel", Priority: 1},
		},
	}
}

// renderConfirmModal asks for confirmation of a parked write action, naming
// what it will run against.
func (m Model) renderConfirmModal() string {
	var b strings.Builder

	b.WriteString(styles.Skin.Title.Render(m.pendingAction.prompt))
	b.WriteString("\n\n")
	b.WriteString(styles.Skin.Body.Render(m.pendingAction.detail))
	b.WriteString("\n")
	scope := m.pendingAction.scope
	if scope == "" {
		scope = "in " + filepath.Base(m.selectedRepo)
	}
	b.WriteString(styles.Skin.Subtitle.Render(scope))

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Skin.Accent.GetForeground()).
		Padding(confirmModalVPad, confirmModalHPad).
		Render(b.String())

	return centerModal(m, content)
}

// actionScreen draws the verb menu in the body of the screen it was opened
// from, which keeps that screen's header, so what the verbs act on stays named.
func (m Model) actionScreen() screen {
	s := m.baseScreen()
	s.body = m.renderActionModal()
	s.hints = []keyhint.Hint{{Key: keyEsc, What: descBack, Priority: 1}}

	return s
}

// renderActionModal lists the current view's verbs over it. The menu names
// what it acts on, and that name is a pull request title as often as a branch,
// so it wraps inside a box rather than running a footer line off the screen.
func (m Model) renderActionModal() string {
	width := min(panelActionModalWidth, max(m.width-panelActionModalFrame, minPanelTableWidth))

	menu := m.actionMenu()

	lines := []string{styles.Skin.Title.Render(menu.title)}
	lines = append(lines, wrapLines(menu.target, width)...)
	lines = append(lines, "")

	if len(menu.actions) == 0 {
		lines = append(lines, styles.Skin.Subtitle.Render("Nothing to do here"))
	}

	for _, action := range menu.actions {
		lines = append(lines,
			styles.Skin.Key.Render(padCell(action.key, modalKeyColWidth))+"  "+
				styles.Skin.Subtitle.Render(action.name))
	}

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Skin.Accent.GetForeground()).
		Padding(confirmModalVPad, confirmModalHPad).
		Render(strings.Join(lines, "\n"))

	return centerModal(m, content)
}

// countForFilter previews the fleet-wide count with mode forced to
// enabled and non-inverted, combined with every other currently active
// filter, predicate, and search, so a dock row shows what turning it on
// would yield rather than that mode's count in isolation.
func (m Model) countForFilter(mode models.FilterMode) int {
	hypothetical := make([]models.ActiveFilter, 0, len(m.activeFilters)+1)

	found := false

	for _, f := range m.activeFilters {
		if f.Mode == mode {
			f.Enabled, f.Inverted = true, false
			found = true
		}

		hypothetical = append(hypothetical, f)
	}

	if !found {
		hypothetical = append(hypothetical, models.ActiveFilter{Mode: mode, Enabled: true})
	}

	matched := filters.FilterReposMulti(m.listableRepos(), m.summaries, hypothetical)
	if m.searchText != "" {
		matched = filters.SearchRepos(matched, m.summaries, m.searchText)
	}

	if m.predicate == nil {
		return len(matched)
	}

	count := 0

	for _, path := range matched {
		if summary, ok := m.summaries[path]; ok && m.predicate(summary) {
			count++
		}
	}

	return count
}

// compactSortPriorities closes gaps in sortsByPriority's Priority values in
// place, so they stay a contiguous 0..n-1 sequence after a sort is disabled.
func compactSortPriorities(sortsByPriority []models.ActiveSort) {
	for i := range sortsByPriority {
		hasPriority := slices.ContainsFunc(sortsByPriority, func(s models.ActiveSort) bool {
			return s.Priority == i
		})
		if hasPriority {
			continue
		}

		for k := range sortsByPriority {
			if sortsByPriority[k].Priority > i {
				sortsByPriority[k].Priority--
			}
		}
	}
}

func buildSortModalRows(activeSorts []models.ActiveSort) []models.ActiveSort {
	sortsByPriority := make([]models.ActiveSort, 0)
	for _, s := range activeSorts {
		if s.IsEnabled() {
			sortsByPriority = append(sortsByPriority, s)
		}
	}

	compactSortPriorities(sortsByPriority)

	inactiveSorts := make([]models.ActiveSort, 0)
	for _, s := range activeSorts {
		if !s.IsEnabled() {
			inactiveSorts = append(inactiveSorts, s)
		}
	}

	displaySorts := make([]models.ActiveSort, 0, len(sortsByPriority)+len(inactiveSorts))
	displaySorts = append(displaySorts, sortsByPriority...)
	displaySorts = append(displaySorts, inactiveSorts...)

	return displaySorts
}
