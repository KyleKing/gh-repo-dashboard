package app

// fetchKind names one of the per-repo fetches the list starts after discovery.
// Each fills one cell, and each needs a placeholder that says "still coming"
// rather than "nothing here" while it is outstanding.
type fetchKind int

// Fetch kinds, one per column the list fills after the summary lands.
const (
	fetchPR fetchKind = iota
	fetchPRCount
	fetchTemplate
	fetchCI
)

type fetchKey struct {
	path string
	kind fetchKind
}

// pendingGlyph marks a cell whose value is still being fetched, against emDash
// for one known to be absent. It occupies the same single cell emDash does, so
// a column cannot shift as data lands. Where a full-width row has room for a
// word, readingLabel says the same thing.
const (
	pendingGlyph = "·"
	readingLabel = "reading…"
)

// startFetch records that path's kind fetch is in flight.
func (m *Model) startFetch(path string, kind fetchKind) {
	if m.fetching == nil {
		m.fetching = make(map[fetchKey]bool)
	}
	m.fetching[fetchKey{path: path, kind: kind}] = true
}

// finishFetch records that path's kind fetch has come back, successfully or
// not, so a failed fetch stops reading as one still in flight.
func (m *Model) finishFetch(path string, kind fetchKind) {
	delete(m.fetching, fetchKey{path: path, kind: kind})
}

func (m *Model) fetchPending(path string, kind fetchKind) bool {
	return m.fetching[fetchKey{path: path, kind: kind}]
}

// fetchesInFlight counts the per-repo fetches still outstanding across the
// fleet, which is what keeps the spinner ticking and the progress badge up.
func (m *Model) fetchesInFlight() int {
	return len(m.fetching)
}

// summaryPending reports whether path's own summary is still being read, the
// state in which every cell drawn from it is unknown rather than empty.
func (m *Model) summaryPending(path string) bool {
	_, loaded := m.summaries[path]

	return !loaded
}

// absentCell is the placeholder for a table cell with no value, and section is
// the same decision for a region row wide enough to spell it out. Both take the
// pending predicate rather than reading it themselves, so callers keying off a
// fetch and callers keying off the summary share one answer.
func absentCell(pending bool) string {
	if pending {
		return pendingGlyph
	}

	return emDash
}

func section(pending bool, value string) string {
	if pending {
		return readingLabel
	}

	return value
}
