package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

func (m Model) renderHelp() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Help"))
	b.WriteString("\n\n")

	sectionStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		Bold(true).
		PaddingLeft(1)

	for _, section := range helpSections() {
		b.WriteString(sectionStyle.Render(section.title))
		b.WriteString("\n")
		for _, k := range section.keys {
			fmt.Fprintf(&b, "  %s  %s\n",
				styles.HelpKeyStyle.Render(padCell(k.key, helpKeyColWidth)),
				styles.HelpDescStyle.Render(k.desc))
		}
		b.WriteString("\n")
	}

	contentLines := strings.Count(b.String(), "\n")
	footerHeight := 1
	paddingNeeded := m.height - contentLines - footerHeight - 1
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(styles.FooterStyle.Render("Press ? or esc to close"))

	return b.String()
}

type helpSection struct {
	title string
	keys  []struct{ key, desc string }
}

func helpSections() []helpSection {
	return []helpSection{
		{
			"Navigation",
			[]struct{ key, desc string }{
				{"j/k, Up/Down", "Move up/down"},
				{"gg/G", "Go to top/bottom"},
				{keyEnter, "Select/enter"},
				{"esc, backspace", "Go back"},
				{"R/P", "Switch tabs: Repos and PRs, as bracketed in the bar"},
				{"h/l, " + keyTab, "Move between panels (focused view)"},
				{"s/b/e/t/n", "Jump straight to a panel, as bracketed in its title"},
				{"space, ;", "Universal find (#12 PRs, b/s/n/r types, * fleet-wide)"},
			},
		},
		{
			"Filtering & Sorting",
			[]struct{ key, desc string }{
				{"f", "Filter menu (enter/key cycles, *=reset)"},
				{"s", "Sort menu (enter/key cycles, [/]=reorder, *=reset)"},
				{"/", "Search repositories (r:/b: to scope name or branch)"},
				{":filter", "Boolean expression over the dock modes plus clean/https/ssh/" +
					"has_upstream/config_override/error/template_drift (and/or/not, parens)"},
			},
		},
		{
			"Branch & PR Actions",
			[]struct{ key, desc string }{
				{"]", "Parallel checkouts of the selected repo"},
				{panelActionLeader, "Verbs for the focused panel's selection (focused view)"},
				{panelActionLeader + "s", "Switch to the selected branch"},
				{panelActionLeader + "p", "Push branch and its tags (--follow-tags)"},
				{panelActionLeader + "n", "Create a PR for the selected branch"},
				{panelActionLeader + "c", "Check the selected PR's branch out here"},
				{"f, [/], *", "PRs tab: pick a saved view, cycle views, widen the scope"},
				{panelActionLeader + "m", "Squash-merge the PR and delete its branch"},
			},
		},
		{
			"Batch Actions",
			[]struct{ key, desc string }{
				{panelActionLeader + "f", "Fetch all, then a text object"},
				{panelActionLeader + "p", "Prune remote, then a text object"},
				{panelActionLeader + "c", "Cleanup merged, then a text object"},
				{panelActionLeader + "r", "Refresh PR data, then a text object"},
				{"the verb again", "Run it over the filtered set (!ff)"},
			},
		},
		{
			"Command Mode",
			[]struct{ key, desc string }{
				{":", "Command prompt (:filter dirty and has_pr, :help)"},
				{"@:", "Repeat the last command"},
				{":history", "Recent commands, newest first"},
				{":prs", "Map open PRs to the branches and checkouts holding them"},
				{"! + verb + object", "Run an operator over a text object (!fdr fetches dirty repos)"},
				{textObjectKeys(), textObjectNames()},
			},
		},
		{
			"General",
			[]struct{ key, desc string }{
				{"r/ctrl+r", "Refresh all data (clears cache)"},
				{"?", "Toggle help"},
				{"q, ctrl+c", "Quit"},
			},
		},
	}
}

// renderConfirmModal asks for confirmation of a parked write action, naming
// what it will run against.
func (m Model) renderConfirmModal() string {
	if m.pendingAction == nil {
		return m.renderRepoList()
	}

	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render(m.pendingAction.prompt))
	b.WriteString("\n\n")
	b.WriteString(styles.TableRowStyle.Render(m.pendingAction.detail))
	b.WriteString("\n")
	scope := m.pendingAction.scope
	if scope == "" {
		scope = "in " + filepath.Base(m.selectedRepo)
	}
	b.WriteString(styles.SubtitleStyle.Render(scope))
	b.WriteString("\n\n")
	b.WriteString(styles.FooterKeyStyle.Render("y/enter") + styles.FooterDescStyle.Render(" confirm  "))
	b.WriteString(styles.FooterKeyStyle.Render("n/esc") + styles.FooterDescStyle.Render(" cancel"))

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Blue).
		Padding(confirmModalVPad, confirmModalHPad).
		Render(b.String())

	return centerModal(m, content)
}

// renderActionModal lists the current view's verbs over it. The menu names
// what it acts on, and that name is a pull request title as often as a branch,
// so it wraps inside a box rather than running a footer line off the screen.
func (m Model) renderActionModal() string {
	width := min(panelActionModalWidth, max(m.width-panelActionModalFrame, minPanelTableWidth))

	menu := m.actionMenu()

	lines := []string{styles.TitleStyle.Render(menu.title)}
	lines = append(lines, wrapLines(menu.target, width)...)
	lines = append(lines, "")

	if len(menu.actions) == 0 {
		lines = append(lines, styles.SubtitleStyle.Render("Nothing to do here"))
	}

	for _, action := range menu.actions {
		lines = append(lines,
			styles.HelpKeyStyle.Render(padCell(action.key, modalKeyColWidth))+"  "+
				styles.HelpDescStyle.Render(action.name))
	}

	lines = append(lines, "",
		styles.FooterKeyStyle.Render(keyEsc)+styles.FooterDescStyle.Render(" back"))

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Blue).
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

func (m Model) renderBatchProgress() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render(m.batchTask))
	b.WriteString("\n\n")

	progressWidth := 40
	filled := 0
	if m.batchTotal > 0 {
		filled = (m.batchProgress * progressWidth) / m.batchTotal
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", progressWidth-filled)
	progressStr := fmt.Sprintf("[%s] %d/%d", bar, m.batchProgress, m.batchTotal)
	b.WriteString(progressStr)
	b.WriteString("\n\n")

	if len(m.batchResults) > 0 {
		b.WriteString(styles.HeaderStyle.Render("Results"))
		b.WriteString("\n")

		maxShow := 15
		startIdx := 0
		if len(m.batchResults) > maxShow {
			startIdx = len(m.batchResults) - maxShow
		}

		layout := fitDetailCols(batchColSpecs, m.width)
		for i := startIdx; i < len(m.batchResults); i++ {
			result := m.batchResults[i]
			icon := styles.SuccessStyle.Render("✓")
			if !result.Success {
				icon = styles.ErrorStyle.Render("✗")
			}
			name := padCell(filepath.Base(result.Path), layout.Width(colBatchName))
			msg := padCell(result.Message, layout.Width(colBatchMessage))

			row := fmt.Sprintf("  %s %s  %s", icon, name, styles.SubtitleStyle.Render(msg))
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.batchRunning {
		b.WriteString(styles.SubtitleStyle.Render("Running... please wait"))
	} else {
		b.WriteString(styles.FooterStyle.Render("Press esc to close"))
	}

	return b.String()
}
