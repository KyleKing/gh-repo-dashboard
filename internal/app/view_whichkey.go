package app

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// whichKeyMaxWidth keeps a long pull request title in a verb menu from widening the panel.
const whichKeyMaxWidth = 44

func whichKeyStyles() keyhint.PanelStyles {
	return keyhint.PanelStyles{
		Frame: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Skin.Edge.GetForeground()).
			Padding(0, 1),
		On:    styles.Skin.Cursor,
		Count: styles.Skin.Subtitle,
		Styles: keyhint.Styles{
			Key:  styles.Skin.Key,
			Text: styles.Skin.Body,
			Head: styles.Skin.Heading,
			Off:  styles.Skin.Muted,
		},
	}
}

func (m Model) whichKey(hints []keyhint.Hint) []string {
	return keyhint.Panel(whichKeyStyles(), hints, min(m.width, whichKeyMaxWidth))
}

func (m Model) whichKeyPanel() []string {
	switch {
	case m.panelActions && m.viewMode != ViewModePalette:
		return m.whichKey(m.actionHints())
	case m.viewMode == ViewModeFilter:
		return m.whichKey(m.filterPanelHints())
	case m.viewMode == ViewModeSort:
		return m.whichKey(m.sortPanelHints())
	}

	return nil
}

func (m Model) actionHints() []keyhint.Hint {
	menu := m.actionMenu()
	hints := []keyhint.Hint{{What: menu.title, Head: true}}

	if menu.target != "" {
		hints = append(hints, keyhint.Hint{What: menu.target, Off: true})
	}

	if len(menu.actions) == 0 {
		return append(hints, keyhint.Hint{What: "Nothing to do here", Off: true})
	}

	for _, action := range menu.actions {
		hints = append(hints, keyhint.Hint{Key: action.key, What: action.name})
	}

	return hints
}

func (m Model) filterOf(mode models.FilterMode) models.ActiveFilter {
	for _, f := range m.activeFilters {
		if f.Mode == mode {
			return f
		}
	}

	return models.ActiveFilter{Mode: mode}
}

// filterPanelHints counts each mode as if turned on alongside every filter already active.
func (m Model) filterPanelHints() []keyhint.Hint {
	hints := []keyhint.Hint{{What: "Filter", Head: true}}

	for _, mode := range models.SelectableFilterModes() {
		state := m.filterOf(mode)

		name := mode.String()
		if state.Enabled && state.Inverted {
			name = notPrefix + name
		}

		hints = append(hints, keyhint.Hint{
			Key: mode.ShortKey(), What: name, Count: strconv.Itoa(m.countForFilter(mode)), On: state.Enabled,
		})
	}

	hints = append(hints, m.scopeHint(), keyhint.Hint{Key: "*", What: "clear"})

	if m.predicateText != "" {
		hints = append(hints, keyhint.Hint{Key: "x", What: "drop " + m.predicateText, On: true})
	}

	return hints
}

// scopeKey toggles the fleet scope inside the filter panel.
const scopeKey = "f"

func (m Model) scopeHint() keyhint.Hint {
	if reason := m.fleet.unavailable(); reason != "" {
		return keyhint.Hint{Key: scopeKey, What: "fleet (" + reason + ")", Off: true}
	}

	hint := keyhint.Hint{Key: scopeKey, What: "Fleet", On: m.fleetScope}
	if m.fleetScope {
		active, total := m.fleetCounts()
		hint.Count = strconv.Itoa(active) + "/" + strconv.Itoa(total)
	}

	return hint
}

func (m Model) sortPanelHints() []keyhint.Hint {
	hints := []keyhint.Hint{{What: "Sort", Head: true}}

	for _, s := range buildSortModalRows(m.activeSorts) {
		hint := keyhint.Hint{Key: s.ShortKey(), What: s.Mode.String()}
		if s.IsEnabled() {
			hint.What += " " + sortArrow(s.Direction)
			hint.Count = strconv.Itoa(s.Priority + 1)
			hint.On = true
		}

		hints = append(hints, hint)
	}

	if len(m.enabledSorts()) > 1 && m.activeSorts[m.sortCursor].IsEnabled() {
		hints = append(hints, keyhint.Hint{
			Key: keyBracketPair, What: "move " + m.activeSorts[m.sortCursor].Mode.String(),
		})
	}

	return append(hints, keyhint.Hint{Key: "*", What: "reset"})
}

const notPrefix = "NOT "

func (m Model) filterChips() []string {
	var chips []string

	for _, f := range m.activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			label := f.Mode.String()
			if f.Inverted {
				label = notPrefix + label
			}

			chips = append(chips, label)
		}
	}

	if m.predicateText != "" {
		chips = append(chips, m.predicateText)
	}

	if m.searchText != "" {
		chips = append(chips, strconv.Quote(m.searchText))
	}

	return chips
}

// scopeChip names the fleet scope and how many repos its activity filter hid,
// so a repo missing from the list is never unexplained.
func (m Model) scopeChip() string {
	if !m.fleetScope {
		return ""
	}

	active, total := m.fleetCounts()

	return "fleet" + compactSignalSep + strconv.Itoa(active) + " active of " + strconv.Itoa(total)
}

// fleetCounts counts repos the way the header does, leaving linked worktrees out.
func (m Model) fleetCounts() (int, int) {
	active := len(m.listableRepos())

	return active, active + m.fleetHidden
}

func (m Model) chipsLine() string {
	chips := m.filterChips()
	if len(chips) == 0 && m.scopeChip() == "" {
		return ""
	}

	rendered := make([]string, 0, len(chips)+1)
	if scope := m.scopeChip(); scope != "" {
		rendered = append(rendered, styles.Skin.BadgeActive.Render(scope))
	}

	for _, chip := range chips {
		rendered = append(rendered, styles.Skin.Badge.Render(chip+" ×"))
	}

	return ansi.Truncate(strings.Join(rendered, " "), repoTableWidth(m.width), table.Ellipsis)
}

func (m Model) chipsHeight() int {
	if len(m.filterChips()) == 0 && m.scopeChip() == "" {
		return 0
	}

	return 1
}
