package table_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

func cols() []table.Column {
	return []table.Column{
		{Key: "name", Title: "NAME", Min: 16, Weight: 3},
		{Key: "branch", Title: "BRANCH", Min: 10, Weight: 2},
		{Key: "status", Title: "STATUS", Min: 12},
		{Key: "pr", Title: "PR", Min: 8, Priority: 1},
		{Key: "modified", Title: "MODIFIED", Min: 12, Priority: 2},
		{Key: "peers", Title: "PEERS", Min: 5, Priority: 3},
		{Key: "template", Title: "TEMPLATE", Min: 10, Weight: 1, Priority: 5},
	}
}

func visibleKeys(l table.Layout) []string {
	keys := make([]string, 0, len(l.Columns))
	for _, c := range l.Columns {
		keys = append(keys, c.Key)
	}

	return keys
}

func TestFitKeepsEveryColumnWhenWidthAllows(t *testing.T) {
	t.Parallel()

	l := table.Fit(cols(), 200)
	if l.Hidden != 0 {
		t.Errorf("hid %d columns at width 200, want 0", l.Hidden)
	}

	if got := l.Total(); got != 200 {
		t.Errorf("total width = %d, want 200", got)
	}
}

func TestFitDistributesSurplusByWeight(t *testing.T) {
	t.Parallel()

	const width = 155

	l := table.Fit(cols(), width)
	base := map[string]int{"name": 16, "branch": 10, "template": 10}
	gained := map[string]int{}
	for key, min := range base {
		gained[key] = l.Width(key) - min
	}

	if gained["name"] <= gained["branch"] || gained["branch"] <= gained["template"] {
		t.Errorf("surplus not ordered by weight: name +%d, branch +%d, template +%d",
			gained["name"], gained["branch"], gained["template"])
	}

	fixed := map[string]int{"status": 12, "pr": 8, "modified": 12, "peers": 5}
	for key, want := range fixed {
		if got := l.Width(key); got != want {
			t.Errorf("unweighted column %s widened to %d, want %d", key, got, want)
		}
	}

	if got := l.Total(); got != width {
		t.Errorf("total width = %d, want %d", got, width)
	}
}

func TestFitRespectsMax(t *testing.T) {
	t.Parallel()

	capped := []table.Column{
		{Key: "name", Title: "NAME", Min: 10, Max: 14, Weight: 1},
		{Key: "notes", Title: "NOTES", Min: 10, Weight: 1},
	}

	l := table.Fit(capped, 100)
	if got := l.Width("name"); got != 14 {
		t.Errorf("capped column is %d wide, want 14", got)
	}

	if got := l.Total(); got != 100 {
		t.Errorf("total width = %d, want 100; surplus freed by the cap was dropped", got)
	}
}

func TestFitCollapsesByAscendingPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		width   int
		want    []string
		hidden  int
		overrun bool
	}{
		{name: "everything fits", width: 200, want: []string{
			"name", "branch", "status", "pr", "modified", "peers", "template",
		}},
		{name: "pr hides first", width: 84, want: []string{
			"name", "branch", "status", "modified", "peers", "template",
		}, hidden: 1},
		{name: "modified hides second", width: 70, want: []string{
			"name", "branch", "status", "peers", "template",
		}, hidden: 2},
		{name: "peers and template outlast the rest", width: 60, want: []string{
			"name", "branch", "status", "template",
		}, hidden: 3},
		{name: "never-hidden columns overrun rather than vanish", width: 20, want: []string{
			"name", "branch", "status",
		}, hidden: 4, overrun: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := table.Fit(cols(), tt.width)
			if got := strings.Join(visibleKeys(l), " "); got != strings.Join(tt.want, " ") {
				t.Errorf("visible columns = %q, want %q", got, strings.Join(tt.want, " "))
			}

			if l.Hidden != tt.hidden {
				t.Errorf("hidden = %d, want %d", l.Hidden, tt.hidden)
			}

			if !tt.overrun && l.Total() > tt.width {
				t.Errorf("total width %d exceeds %d", l.Total(), tt.width)
			}
		})
	}
}

func TestMarkerCountsHiddenColumns(t *testing.T) {
	t.Parallel()

	if got := table.Fit(cols(), 200).Marker(); got != "" {
		t.Errorf("marker = %q with nothing hidden, want empty", got)
	}

	l := table.Fit(cols(), 70)
	if got := l.Marker(); got != "…+2" {
		t.Errorf("marker = %q, want %q", got, "…+2")
	}

	if !strings.HasSuffix(table.Header(l), "…+2") {
		t.Errorf("header does not announce hidden columns: %q", table.Header(l))
	}
}

func TestPadMeasuresDisplayWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		width int
		align table.Align
		want  string
	}{
		{name: "left pads on the right", text: "ab", width: 5, want: "ab   "},
		{name: "right pads on the left", text: "ab", width: 5, align: table.AlignRight, want: "   ab"},
		{name: "exact fit is untouched", text: "abcde", width: 5, want: "abcde"},
		{name: "overflow gets an ellipsis", text: "abcdefgh", width: 5, want: "abcd…"},
		{name: "wide glyph counts two cells", text: "🚀🚀🚀", width: 5, want: "🚀🚀…"},
		{name: "wide glyph leaves an odd cell padded", text: "🚀ab", width: 6, want: "🚀ab  "},
		{name: "no room for an ellipsis", text: "abc", width: 1, want: "a"},
		{name: "zero width is empty", text: "abc", width: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := table.Pad(tt.text, tt.width, tt.align)
			if got != tt.want {
				t.Errorf("Pad(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}

			if w := lipgloss.Width(got); w != tt.width && tt.width > 0 {
				t.Errorf("Pad(%q, %d) is %d cells wide", tt.text, tt.width, w)
			}
		})
	}
}

func TestTruncateNeverSplitsAWideGlyph(t *testing.T) {
	t.Parallel()

	// The rocket needs two cells and the ellipsis one, so only "a" survives.
	if got := table.Truncate("a🚀b", 3); got != "a…" {
		t.Errorf("Truncate(%q, 3) = %q, want %q", "a🚀b", got, "a…")
	}

	//nolint:gosmopolitan // CJK is the point: every rune here is two cells wide
	if got := lipgloss.Width(table.Truncate("日本語テキスト", 7)); got > 7 {
		t.Errorf("truncated CJK text is %d cells wide, want at most 7", got)
	}
}

func TestPadKeepsEmojiPresentationRowsAligned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		width int
	}{
		{"emoji plus variation selector fits", "⬆️ bump mise-action", 12},
		{"cut lands on the emoji", "⬆️ bump", 3},
		{"cut lands inside the cluster", "⬆️ bump", 2},
		{"plain text", "bump mise-action", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			padded := table.Pad(tt.text, tt.width, table.AlignLeft)
			if got := lipgloss.Width(padded); got != tt.width {
				t.Errorf("Pad(%q, %d) measures %d cells, want %d: %q", tt.text, tt.width, got, tt.width, padded)
			}
		})
	}
}
