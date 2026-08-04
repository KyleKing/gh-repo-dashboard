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

	sections := []struct {
		title string
		keys  []struct{ key, desc string }
	}{
		{
			"Navigation",
			[]struct{ key, desc string }{
				{"j/k, Up/Down", "Move up/down"},
				{"h/l, Left/Right", "Switch tabs (detail view)"},
				{"g/G", "Go to top/bottom"},
				{"enter, space", "Select/enter"},
				{"esc, backspace", "Go back"},
				{keyTab, "Next tab (detail view)"},
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
				{"c", "Switch to the selected branch"},
				{"p", "Push branch and its tags (--follow-tags)"},
				{"N", "Create a PR for the selected branch"},
				{"M", "Squash-merge the PR and delete its branch"},
			},
		},
		{
			"Batch Actions",
			[]struct{ key, desc string }{
				{"F", "Fetch all (filtered repos)"},
				{"P", "Prune remote (filtered repos)"},
				{"C", "Cleanup merged (filtered repos)"},
			},
		},
		{
			"Command Mode",
			[]struct{ key, desc string }{
				{":", "Command prompt (:filter dirty and has_pr, :help)"},
				{"@:", "Repeat the last command"},
				{":history", "Recent commands, newest first"},
				{"F/P/C + object", "Run an operator over a text object (Fdr fetches dirty repos)"},
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

	for _, section := range sections {
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
// the repo it will run against.
func (m Model) renderConfirmModal() string {
	if m.pendingAction == nil {
		return m.renderRepoList()
	}

	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render(m.pendingAction.prompt))
	b.WriteString("\n\n")
	b.WriteString(styles.TableRowStyle.Render(m.pendingAction.detail))
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render("in " + filepath.Base(m.selectedRepo)))
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

		for i := startIdx; i < len(m.batchResults); i++ {
			result := m.batchResults[i]
			icon := styles.SuccessStyle.Render("✓")
			if !result.Success {
				icon = styles.ErrorStyle.Render("✗")
			}
			name := padCell(filepath.Base(result.Path), batchNameTruncLen)
			msg := truncate(result.Message, messageTruncLen)

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
