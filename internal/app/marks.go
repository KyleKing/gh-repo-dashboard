package app

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
)

// Marking rows is the half of the vim paradigm the operators were missing.
// Text objects name a set by what its repos are ("dirty", "behind"); marks
// name one by pointing at its rows. An operator takes either, so "fetch these
// four" and "fetch everything dirty" are the same sentence with a different
// object.
//
// Marks live in Model.selectedPaths, which ":select" and the universal find
// already write, so all three ways of choosing repos land in one place.

// markToggleKey marks or unmarks the row under the cursor, and visualKey
// starts the line-wise range that motions extend.
const (
	markToggleKey = "x"
	visualKey     = "V"
)

// toggleMark marks or unmarks the row under the cursor and steps past it, so
// marking a run of repos is one key held down rather than two alternating.
func (m Model) toggleMark() (Model, bool) {
	if m.viewMode != ViewModeRepoList || m.cursor >= len(m.filteredPaths) {
		return m, false
	}

	path := m.filteredPaths[m.cursor]
	if m.selectedPaths == nil {
		m.selectedPaths = make(map[string]bool)
	}

	if m.selectedPaths[path] {
		delete(m.selectedPaths, path)
	} else {
		m.selectedPaths[path] = true
	}

	m.cursor = min(m.cursor+1, len(m.filteredPaths)-1)

	return m, true
}

// startVisual anchors a line-wise range at the cursor. Every motion from here
// extends it, and an operator run while it is live acts on it.
func (m Model) startVisual() (Model, bool) {
	if m.viewMode != ViewModeRepoList || len(m.filteredPaths) == 0 {
		return m, false
	}

	m.visualMode = true
	m.visualAnchor = m.cursor

	return m, true
}

// visualRange is the rows the live range covers, empty when there is none.
func (m Model) visualRange() []string {
	if !m.visualMode || len(m.filteredPaths) == 0 {
		return nil
	}

	lo, hi := min(m.visualAnchor, m.cursor), max(m.visualAnchor, m.cursor)
	lo, hi = max(lo, 0), min(hi, len(m.filteredPaths)-1)

	return m.filteredPaths[lo : hi+1]
}

// isMarked reports whether a row draws its mark, which is the live range while
// one is open and the toggled marks otherwise. The range shows over the marks
// rather than merged into them, so leaving visual mode restores what was
// marked before it opened.
func (m Model) isMarked(path string) bool {
	if m.visualMode {
		for _, p := range m.visualRange() {
			if p == path {
				return true
			}
		}

		return false
	}

	return m.selectedPaths[path]
}

// markedPaths is what an operator acts on without being asked: the live range
// while one is open, else the toggled marks. Empty means nothing is marked and
// the operator goes on to ask for a text object.
func (m Model) markedPaths() []string {
	if marked := m.visualRange(); len(marked) > 0 {
		return marked
	}

	paths := make([]string, 0, len(m.selectedPaths))
	for _, path := range m.filteredPaths {
		if m.selectedPaths[path] {
			paths = append(paths, path)
		}
	}

	return paths
}

// clearMarks drops the live range and every toggled mark, which is what esc
// does once neither the docks nor the expanded region want it.
func (m Model) clearMarks() (Model, bool) {
	if !m.visualMode && len(m.selectedPaths) == 0 {
		return m, false
	}

	m.visualMode = false
	m.selectedPaths = make(map[string]bool)

	return m, true
}

// markSummary names the marks for a footer or a menu title, empty when there
// are none.
func (m Model) markSummary() string {
	marked := len(m.markedPaths())
	if marked == 0 {
		return ""
	}

	if m.visualMode {
		return strconv.Itoa(marked) + " in range"
	}

	return strconv.Itoa(marked) + " marked"
}

// handleMarkKey answers the two keys that mark rows. Handled is false if msg
// is neither, so the caller falls through to the rest of its bindings.
func (m Model) handleMarkKey(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case markToggleKey:
		return m.toggleMark()
	case visualKey:
		return m.startVisual()
	}

	return m, false
}
