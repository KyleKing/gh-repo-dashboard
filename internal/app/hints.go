package app

import (
	"strings"

	"github.com/kyleking/aragonite/tui/keyhint"
)

const (
	keyNavPair = "j/k"
	descNav    = "nav"
	descBack   = "back"
	descOpen   = "open"
	descClose  = "close"
	descScroll = "scroll"
	keysEnds   = "gg/G"
	descEnds   = "the ends"
	nameRun    = "run"

	pendingHintPriority = 100
)

func (m Model) listHints() []keyhint.Hint {
	switch m.viewMode { //nolint:exhaustive // every other view shows the list's own keys
	case ViewModeFilter:
		return filterPanelFooter(m.predicateText != "")
	case ViewModeSort:
		return sortPanelFooter()
	}

	hints := listKeyHints(m.detailHints(), m.markSummary())

	if m.pendingOperator == "" {
		return hints
	}

	pending := keyhint.Hint{
		Key:      m.pendingOperator + m.pendingObject,
		What:     "pending (ar/br/dr/pr/sr, " + strings.ToLower(m.pendingOperator) + "=all, esc cancels)",
		Priority: pendingHintPriority,
	}

	return append([]keyhint.Hint{pending}, hints...)
}

// detailHints is the v hint, which exists only where v does something. It
// outranks the rest of the second tier while the region is open, because the
// key that closes a region taking most of the screen must stay on offer.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func (m Model) detailHints() []keyhint.Hint {
	switch {
	case !m.detailToggleable():
		return nil
	case m.detailOpen:
		return []keyhint.Hint{{Key: "v", What: "hide", Priority: 11}}
	}

	return []keyhint.Hint{{Key: "v", What: "detail", Priority: 3}}
}

// listKeyHints lists every repo-list hint in render order.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func listKeyHints(detail []keyhint.Hint, marked string) []keyhint.Hint {
	// The mark hint states the count once there is one, since that count is
	// what every operator is about to act on and nothing else on screen says
	// it. With nothing marked it names the key's job instead.
	mark := keyhint.Hint{Key: markToggleKey, What: "mark", Priority: 2}
	if marked != "" {
		mark = keyhint.Hint{Key: markToggleKey, What: marked, Priority: 12}
	}

	hints := []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 4},
		{Key: keyEnter, What: nameSelect, Priority: 8},
	}
	hints = append(hints, detail...)

	return append(hints, []keyhint.Hint{
		mark,
		{Key: actionLeader, What: nameActions, Priority: 7},
		{Key: tabKeyTasks, What: "tasks", Priority: 3},
		{Key: "f", What: nameFilter, Priority: 6},
		{Key: "s", What: nameSort, Priority: 5},
		{Key: "/", What: "search", Priority: 5},
		{Key: ":", What: "command", Priority: 4},
		{Key: "r", What: nameRefresh, Priority: 1},
		{Key: "?", What: nameHelp, Priority: 9},
		{Key: "q", What: nameQuit, Priority: 10},
	}...)
}

// filterPanelFooter lists the keys that act while the filter panel is open,
// beside the mode keys the panel itself shows.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func filterPanelFooter(hasPredicate bool) []keyhint.Hint {
	hints := []keyhint.Hint{
		{Key: "*", What: "clear", Priority: 4},
		{Key: keyEsc, What: descClose, Priority: 7},
		{Key: "q", What: nameQuit, Priority: 8},
	}
	if hasPredicate {
		hints = append(hints, keyhint.Hint{Key: "x", What: "drop predicate", Priority: 3})
	}

	return hints
}

// sortPanelFooter lists the keys that act while the sort panel is open.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func sortPanelFooter() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: keyBracketPair, What: "reorder", Priority: 4},
		{Key: "*", What: "reset", Priority: 3},
		{Key: keyEsc, What: descClose, Priority: 7},
		{Key: "q", What: nameQuit, Priority: 8},
	}
}
