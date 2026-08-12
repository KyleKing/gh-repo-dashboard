package app

// Width thresholds from docs/design/layout-and-density.md. Density is derived
// on every render rather than stored, so a resize needs no state to migrate.
// Past wideMinWidth the table hides no columns; the width it renders into
// above that is a continuous rule rather than a step (see listWidth).
const (
	standardMinWidth = 100
	wideMinWidth     = 160
)

// compactLayout reports whether width can only carry stacked two-line records.
// Eight columns cannot be read at 80 cells, so compact is its own layout rather
// than a shrunken table.
func compactLayout(width int) bool {
	return width < standardMinWidth
}
