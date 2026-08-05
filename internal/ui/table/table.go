// Package table fits a set of columns into an available terminal width and
// pads cells to it. Every measurement is in display cells via lipgloss.Width,
// so wide glyphs never shift a row.
package table

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"
)

// Align selects which side of its column a cell's text sits against.
type Align int

// Cell alignments. Text reads better left-aligned; counts and ages read
// better right-aligned against the next gutter.
const (
	AlignLeft Align = iota
	AlignRight
)

// Gutter is the number of blank cells rendered between two adjacent columns.
const Gutter = 2

// Ellipsis replaces the tail of a cell that does not fit its column.
const Ellipsis = "…"

// Column describes one column of a table.
//
// Priority orders collapse: the lowest positive Priority is hidden first, so
// a column's Priority is its information value. Zero means the column is
// never hidden and the table overflows instead. Max of zero is unbounded.
type Column struct {
	Key      string
	Title    string
	Min      int
	Max      int
	Weight   int
	Priority int
	Align    Align
}

// Layout is a resolved table: the columns that survived collapse, in render
// order, each with a final width.
type Layout struct {
	Columns []Column
	Hidden  int

	widths map[string]int
}

// Width returns the resolved width of the named column, or zero when it was
// hidden or is not part of the table.
func (l Layout) Width(key string) int {
	return l.widths[key]
}

// Total returns the display width the laid-out row occupies, including
// gutters and the hidden-column marker.
func (l Layout) Total() int {
	total := 0
	for i, c := range l.Columns {
		if i > 0 {
			total += Gutter
		}

		total += l.widths[c.Key]
	}

	if l.Hidden > 0 {
		total += Gutter + lipgloss.Width(l.Marker())
	}

	return total
}

// Marker announces how many columns collapse hid, or is empty when none were.
func (l Layout) Marker() string {
	if l.Hidden == 0 {
		return ""
	}

	return Ellipsis + "+" + strconv.Itoa(l.Hidden)
}

// Fit resolves cols into width display cells. Columns are hidden by ascending
// Priority until the remaining minimums fit, then the surplus is shared
// between the weighted columns in proportion to their Weight and capped at
// their Max.
func Fit(cols []Column, width int) Layout {
	layout := collapse(cols, width)

	layout.widths = make(map[string]int, len(layout.Columns))
	for _, c := range layout.Columns {
		layout.widths[c.Key] = c.Min
	}

	distribute(layout, width-minWidth(layout.Columns, layout.Hidden))

	return layout
}

// collapse hides the least valuable columns until the rest fit in width,
// stopping once only never-hidden columns remain. The returned layout has no
// resolved widths yet.
func collapse(cols []Column, width int) Layout {
	visible := make([]Column, len(cols))
	copy(visible, cols)

	hidden := 0
	for minWidth(visible, hidden) > width {
		victim := -1
		for i, c := range visible {
			if c.Priority > 0 && (victim < 0 || c.Priority < visible[victim].Priority) {
				victim = i
			}
		}

		if victim < 0 {
			break
		}

		visible = append(visible[:victim], visible[victim+1:]...)
		hidden++
	}

	return Layout{Columns: visible, Hidden: hidden}
}

// minWidth returns the narrowest row that holds cols at their minimums, with
// a gutter between each pair and room for the hidden-column marker.
func minWidth(cols []Column, hidden int) int {
	total := 0
	for i, c := range cols {
		if i > 0 {
			total += Gutter
		}

		total += c.Min
	}

	if hidden > 0 {
		total += Gutter + lipgloss.Width(Ellipsis+"+"+strconv.Itoa(hidden))
	}

	return total
}

// distribute hands surplus cells to the weighted columns, repeating while any
// column that hit its Max frees up cells for the rest.
func distribute(l Layout, surplus int) {
	for surplus > 0 {
		weight := 0
		for _, c := range l.Columns {
			if c.Weight > 0 && !atMax(c, l.widths[c.Key]) {
				weight += c.Weight
			}
		}

		if weight == 0 {
			return
		}

		spent := 0
		for _, c := range l.Columns {
			if c.Weight == 0 || atMax(c, l.widths[c.Key]) {
				continue
			}

			share := max(surplus*c.Weight/weight, 1)
			if c.Max > 0 {
				share = min(share, c.Max-l.widths[c.Key])
			}
			share = min(share, surplus-spent)

			l.widths[c.Key] += share
			spent += share
		}

		if spent == 0 {
			return
		}

		surplus -= spent
	}
}

func atMax(c Column, width int) bool {
	return c.Max > 0 && width >= c.Max
}

// Pad truncates text to width display cells, appending an ellipsis when one
// fits, and pads the remainder to exactly width on the side align implies.
func Pad(text string, width int, align Align) string {
	text = Truncate(text, width)

	gap := width - lipgloss.Width(text)
	if gap <= 0 {
		return text
	}

	if align == AlignRight {
		return strings.Repeat(" ", gap) + text
	}

	return text + strings.Repeat(" ", gap)
}

// Truncate shortens text to at most width display cells, marking the cut with
// an ellipsis whenever the result still leaves room for content.
func Truncate(text string, width int) string {
	if lipgloss.Width(text) <= width {
		return text
	}

	markWidth := lipgloss.Width(Ellipsis)
	if width <= markWidth {
		return clip(text, width)
	}

	return clip(text, width-markWidth) + Ellipsis
}

// clip cuts text at the last grapheme-cluster boundary that still fits in
// width. Clusters, not runes: an emoji presentation sequence like "⬆️" is a
// base rune plus a zero-width variation selector, so measuring the two
// separately reports one cell for something the terminal paints in two, and
// cutting between them changes the glyph.
func clip(text string, width int) string {
	if width <= 0 {
		return ""
	}

	used := 0
	rest := text
	for rest != "" {
		cluster, remainder, _, _ := uniseg.FirstGraphemeClusterInString(rest, -1)

		w := lipgloss.Width(cluster)
		if used+w > width {
			return text[:len(text)-len(rest)]
		}

		used += w
		rest = remainder
	}

	return text
}

// Header renders the column titles at their resolved widths, followed by the
// hidden-column marker when collapse dropped any. Callers style the result.
func Header(l Layout) string {
	cells := make([]string, 0, len(l.Columns))
	for _, c := range l.Columns {
		cells = append(cells, Pad(c.Title, l.widths[c.Key], c.Align))
	}

	if marker := l.Marker(); marker != "" {
		cells = append(cells, marker)
	}

	return Join(cells)
}

// Join concatenates already-padded cells with the standard gutter.
func Join(cells []string) string {
	return strings.Join(cells, strings.Repeat(" ", Gutter))
}
