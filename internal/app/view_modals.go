package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
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
				{"/", "Search repositories"},
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

func (m Model) renderFilterModal() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Filter Repositories"))
	b.WriteString("\n\n")

	modes := models.SelectableFilterModes()

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Subtext0).
		Bold(true)

	header := fmt.Sprintf("  %s  %s  %s  %s",
		padCell("", modalMarkColWidth), padCell("Key", modalKeyColWidth),
		padCell("Filter", filterLabelColWidth), "Count")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	for i, mode := range modes {
		cursor := "  "
		if i == m.filterCursor {
			cursor = "> "
		}

		var filterState models.ActiveFilter
		for _, f := range m.activeFilters {
			if f.Mode == mode {
				filterState = f
				break
			}
		}

		checkbox := "   "
		if filterState.Enabled && filterState.Inverted {
			checkbox = "NOT"
		} else if filterState.Enabled {
			checkbox = " ✓ "
		}

		shortKey := mode.ShortKey()
		label := mode.String()
		count := m.countForFilter(mode)

		var rowStyle lipgloss.Style
		if i == m.filterCursor {
			rowStyle = styles.SelectedRowStyle
		} else {
			rowStyle = styles.TableRowStyle
		}

		checkStyle := lipgloss.NewStyle().Foreground(styles.Green)
		if filterState.Inverted {
			checkStyle = lipgloss.NewStyle().Foreground(styles.Peach)
		}

		keyStyle := lipgloss.NewStyle().
			Foreground(styles.Mauve).
			Bold(true)

		formattedCheck := padCell(checkbox, modalMarkColWidth)
		formattedKey := padCell(shortKey, modalKeyColWidth)
		formattedLabel := padCell(label, filterLabelColWidth)
		formattedCount := strconv.Itoa(count)

		row := fmt.Sprintf("%s%s  %s  %s  %s",
			cursor,
			checkStyle.Render(formattedCheck),
			keyStyle.Render(formattedKey),
			rowStyle.Render(formattedLabel),
			styles.SubtitleStyle.Render(formattedCount),
		)
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpLines := []string{
		styles.FooterKeyStyle.Render("enter/key") + styles.FooterDescStyle.Render(" cycle (off/on/NOT)"),
		styles.FooterKeyStyle.Render("*") + styles.FooterDescStyle.Render(" reset"),
		styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" close"),
	}
	b.WriteString(strings.Join(helpLines, "  "))

	content := b.String()

	return centerModal(m, content)
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

func (m Model) countForFilter(mode models.FilterMode) int {
	return len(filters.FilterRepos(m.repoPaths, m.summaries, mode))
}

// buildSortModalRows orders activeSorts for display: enabled sorts first (with
// their priority gaps compacted), then disabled sorts.
// CompactSortPriorities closes any gaps in sortsByPriority's Priority values
// (e.g. after a sort was disabled) so priorities are a contiguous 0..n-1
// sequence, in place.
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

func renderSortModalRow(sortState models.ActiveSort, isSelected bool) string {
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	indicator := "   "
	if sortState.IsEnabled() {
		indicator = fmt.Sprintf(" %d ", sortState.Priority+1)
	}

	shortKey := sortState.ShortKey()
	label := sortState.DisplayName()
	if !sortState.IsEnabled() {
		label = sortState.Mode.String()
	}

	rowStyle := styles.TableRowStyle
	if isSelected {
		rowStyle = styles.SelectedRowStyle
	}

	checkStyle := lipgloss.NewStyle().Foreground(styles.Green)
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Mauve).
		Bold(true)

	formattedIndicator := padCell(indicator, modalMarkColWidth)
	formattedKey := padCell(shortKey, modalKeyColWidth)

	return fmt.Sprintf("%s%s  %s  %s",
		cursor,
		checkStyle.Render(formattedIndicator),
		keyStyle.Render(formattedKey),
		rowStyle.Render(label),
	)
}

func (m Model) renderSortModal() string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Sort Repositories"))
	b.WriteString("\n\n")

	displaySorts := buildSortModalRows(m.activeSorts)

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Subtext0).
		Bold(true)

	header := fmt.Sprintf("  %s  %s  %s",
		padCell("", modalMarkColWidth), padCell("Key", modalKeyColWidth), "Sort By")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	cursorIndex := -1
	for i, s := range displaySorts {
		if s.Mode == m.activeSorts[m.sortCursor].Mode {
			cursorIndex = i
			break
		}
	}

	for i, sortState := range displaySorts {
		b.WriteString(renderSortModalRow(sortState, i == cursorIndex))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	helpLines := []string{
		styles.FooterKeyStyle.Render("enter/key") + styles.FooterDescStyle.Render(" cycle (off/ASC/DESC)"),
		styles.FooterKeyStyle.Render("[/]") + styles.FooterDescStyle.Render(" reorder"),
		styles.FooterKeyStyle.Render("*") + styles.FooterDescStyle.Render(" reset"),
		styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" close"),
	}
	b.WriteString(strings.Join(helpLines, "  "))

	content := b.String()

	return centerModal(m, content)
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
