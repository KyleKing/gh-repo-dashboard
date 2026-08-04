package app

// breakpoint names the density the terminal's width can carry. It is derived
// on every render rather than stored, so a resize needs no state to migrate.
type breakpoint int

// Layouts, narrowest first. The compact layout is its own UX rather than a
// shrunken table: eight columns cannot be read at 80 cells, so records stack.
const (
	breakpointCompact breakpoint = iota
	breakpointStandard
	breakpointWide
)

// Width thresholds from docs/design/layout-and-density.md. The wide layout
// also needs vertical room, so a short terminal falls back to standard even
// when it is wide enough.
const (
	standardMinWidth   = 100
	wideMinWidth       = 160
	widePanelMinHeight = 20
)

// breakpointFor selects the layout for a terminal of the given size.
func breakpointFor(width, height int) breakpoint {
	switch {
	case width >= wideMinWidth && height >= widePanelMinHeight:
		return breakpointWide
	case width >= standardMinWidth:
		return breakpointStandard
	default:
		return breakpointCompact
	}
}

// String names the layout for the footer and for test failure messages.
func (b breakpoint) String() string {
	const standard = "standard"

	switch b {
	case breakpointCompact:
		return "compact"
	case breakpointWide:
		return "wide"
	case breakpointStandard:
		return standard
	default:
		return standard
	}
}
