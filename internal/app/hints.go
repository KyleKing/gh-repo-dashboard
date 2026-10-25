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

	pendingHintPriority = 100
)

func (m Model) listHints() []keyhint.Hint {
	switch m.viewMode { //nolint:exhaustive // every other view shows the list's own keys
	case ViewModeFilter:
		return filterDockHints(m.predicateText != "")
	case ViewModeSort:
		return sortDockHints()
	}

	hints := listKeyHints(m.expandOpen, m.markSummary())

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

// listKeyHints lists every repo-list hint in render order. The expand hint
// outranks the rest of the second tier while the region is open, because the
// key that closes a region taking most of the screen must stay on offer.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func listKeyHints(expandOpen bool, marked string) []keyhint.Hint {
	expand := keyhint.Hint{Key: "v", What: "expand", Priority: 3}
	if expandOpen {
		expand = keyhint.Hint{Key: "v", What: "hide", Priority: 11}
	}

	// The mark hint states the count once there is one, since that count is
	// what every operator is about to act on and nothing else on screen says
	// it. With nothing marked it names the key's job instead.
	mark := keyhint.Hint{Key: markToggleKey, What: "mark", Priority: 2}
	if marked != "" {
		mark = keyhint.Hint{Key: markToggleKey, What: marked, Priority: 12}
	}

	return []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 4},
		{Key: keyEnter, What: nameSelect, Priority: 8},
		expand,
		mark,
		{Key: actionLeader, What: nameActions, Priority: 7},
		{Key: "f", What: nameFilter, Priority: 6},
		{Key: "s", What: nameSort, Priority: 5},
		{Key: "/", What: "search", Priority: 5},
		{Key: ":", What: "command", Priority: 4},
		{Key: "r", What: nameRefresh, Priority: 1},
		{Key: "?", What: nameHelp, Priority: 9},
		{Key: "q", What: nameQuit, Priority: 10},
	}
}

// filterDockHints lists the filter dock's own keys, so the grammar (cycle,
// invert, reset, clear the predicate) is visible while the dock is actually
// open rather than only behind the help overlay.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func filterDockHints(hasPredicate bool) []keyhint.Hint {
	hints := []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 5},
		{Key: "enter/key", What: "cycle", Priority: 6},
		{Key: "*", What: "reset", Priority: 4},
		{Key: keyEsc, What: descBack, Priority: 7},
		{Key: "q", What: nameQuit, Priority: 8},
	}
	if hasPredicate {
		hints = append(hints, keyhint.Hint{Key: "x", What: "clear predicate", Priority: 3})
	}

	return hints
}

// sortDockHints lists the sort dock's own keys.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func sortDockHints() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 5},
		{Key: "enter/key", What: "cycle", Priority: 6},
		{Key: keyBracketPair, What: "reorder", Priority: 4},
		{Key: "*", What: "reset", Priority: 3},
		{Key: keyEsc, What: descBack, Priority: 7},
		{Key: "q", What: nameQuit, Priority: 8},
	}
}
