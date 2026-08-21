// Package region renders the block a list opens beneath itself: a rule naming
// what is open, a head of label/value facts, a second rule, a body of free
// text, and a divider captioning what sat above it. It knows nothing about
// what a list holds, so the same shape can close over a repository, a pull
// request, or anything a later list wants to expand in place.
//
// Rendering is total: Render always returns exactly the number of lines asked
// for, so a region whose content is still loading cannot move the table above
// it as data lands.
package region

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// Styles are the faces a region draws with, passed in so the package never
// reaches for an application's palette.
type Styles struct {
	// Rule draws the ─ runs and the head's labels.
	Rule lipgloss.Style
	// Label draws a name sitting inside a rule.
	Label lipgloss.Style
}

// Fact is one label/value pair of the head.
type Fact struct {
	Label string
	Value string
}

// Region is one expandable block. Every field is optional: a region with no
// Head renders its body straight under the opening rule, and one with no
// Section runs the two together.
type Region struct {
	// Title names the opening rule, Section the rule between the head and the
	// body, and Caption sits right-aligned in the closing divider.
	Title string
	Head  []Fact
	// HeadColumns is the most fact columns the head may pack onto one line
	// where width allows. It belongs to the caller because only the caller
	// knows how long its values run: facts holding a word or two read well
	// side by side, and facts holding a sentence do not. Zero and one both
	// mean one fact per line.
	HeadColumns int
	Section     string
	Body        []string
	Caption     string
}

// ruleLead is the ─ run on the short side of a rule, and ruleSpaces the blank
// on each side of its label.
const (
	ruleLead   = 2
	ruleSpaces = 2
)

// factGutter separates a fact's value from the label of the fact beside it.
const factGutter = 3

// factMinWidth is the narrowest a fact column may be before the head drops to
// a single column, below which a label and its value have no room to sit on
// one line. factMaxWidth caps how far apart a wide terminal pushes two facts,
// since a pair the eye has to travel between reads worse than a pair that
// does not fill the line.
const (
	factMinWidth = 28
	factMaxWidth = 52
)

// Render draws the region into exactly height lines of width cells. A body
// longer than the room left over is elided in its middle, since the lines that
// matter are usually at its ends.
func (r Region) Render(s Styles, width, height int) []string {
	if height <= 0 {
		return nil
	}

	lines := make([]string, 0, height)
	if r.Title != "" {
		lines = append(lines, openingRule(s, r.Title, width))
	}

	lines = append(lines, r.headLines(s, width)...)

	if r.Section != "" {
		lines = append(lines, openingRule(s, r.Section, width))
	}

	room := height - len(lines) - 1
	lines = append(lines, padBottom(elideMiddle(s, r.Body, room), room)...)

	return fit(append(lines, closingRule(s, r.Caption, width)), height)
}

// headLines lays the facts out in as many columns as width allows, so a
// handful of short facts reads as two lines on a wide terminal and one line
// each on a narrow one.
func (r Region) headLines(s Styles, width int) []string {
	if len(r.Head) == 0 {
		return nil
	}

	cols := max(min(min(width/factMinWidth, r.HeadColumns), len(r.Head)), 1)
	label := labelWidth(r.Head)
	cell := min((width-factGutter*(cols-1))/cols, factMaxWidth)
	// The last fact on a line runs to the edge instead of stopping at the cap,
	// so capping the gap between columns never costs a long value its room.
	last := width - (cols-1)*(cell+factGutter)

	lines := make([]string, 0, (len(r.Head)+cols-1)/cols)

	for start := 0; start < len(r.Head); start += cols {
		row := r.Head[start:min(start+cols, len(r.Head))]

		cells := make([]string, 0, cols)
		for i, f := range row {
			room := cell
			if i == len(row)-1 {
				room = last
			}

			cells = append(cells, s.Rule.Render(table.Pad(f.Label, label, table.AlignLeft))+
				" "+table.Truncate(f.Value, room-label-1))
		}

		lines = append(lines, strings.TrimRight(strings.Join(padCells(cells, cell), gutter), " "))
	}

	return lines
}

// gutter separates one fact column from the next.
var gutter = strings.Repeat(" ", factGutter) //nolint:gochecknoglobals // a derived constant, never assigned to

// padCells widens every cell but the last to the column width, so the labels
// of one row line up with the labels of the next.
func padCells(cells []string, width int) []string {
	for i := range cells[:len(cells)-1] {
		cells[i] = table.Pad(cells[i], width, table.AlignLeft)
	}

	return cells
}

func labelWidth(facts []Fact) int {
	widest := 0
	for _, f := range facts {
		widest = max(widest, lipgloss.Width(f.Label))
	}

	return widest
}

// openingRule heads a section, with its name at the left where the eye starts.
func openingRule(s Styles, label string, width int) string {
	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-ruleLead-ruleSpaces, 0))

	return s.Rule.Render(strings.Repeat("─", ruleLead)+" ") +
		s.Label.Render(label) +
		s.Rule.Render(" "+rule)
}

// closingRule captions the region on the right, where the text above it ended.
func closingRule(s Styles, label string, width int) string {
	if label == "" {
		return s.Rule.Render(strings.Repeat("─", width))
	}

	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-ruleSpaces-ruleLead, 0))

	return s.Rule.Render(rule+" ") +
		s.Label.Render(label) +
		s.Rule.Render(" "+strings.Repeat("─", ruleLead))
}

// elisionEnds is how many ends of a body survive a middle elision.
const elisionEnds = 2

// elideMiddle drops the middle of a body too long for its room, saying how
// many lines went, so the ends a reader scans first both survive.
func elideMiddle(s Styles, lines []string, height int) []string {
	if len(lines) <= height || height < 1 {
		return lines
	}

	kept := height - 1
	head := kept / elisionEnds
	tail := kept - head

	out := make([]string, 0, height)
	out = append(out, lines[:head]...)
	out = append(out, s.Rule.Render("  ⋯ "+strconv.Itoa(len(lines)-head-tail)+" more lines ⋯"))

	return append(out, lines[len(lines)-tail:]...)
}

// padBottom grows lines to exactly height by adding blanks below them, so a
// body shorter than its room starts under the rule above it and the slack
// collects against the divider that closes the region.
func padBottom(lines []string, height int) []string {
	if len(lines) >= height {
		return lines[:max(height, 0)]
	}

	return append(lines, make([]string, height-len(lines))...)
}

// fit trims a rendered region to height, keeping the closing divider, which is
// the line that tells a reader the region ended.
func fit(lines []string, height int) []string {
	if len(lines) <= height {
		return lines
	}

	return append(lines[:height-1], lines[len(lines)-1])
}
