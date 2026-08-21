//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// wideGlyph occupies two terminal cells and wideStandIn occupies the same two
// cells with plain ASCII. A table padded by rune count instead of display
// width renders the two at different geometries.
const (
	wideGlyph    = "🚀"
	wideStandIn  = "xx"
	ansiEscape   = '\x1b'
	maskFilled   = '#'
	maskEmptyRun = '.'
)

// cellMask reduces a rendered frame to the shape its glyphs occupy: styling is
// dropped and every rune expands to one mask cell per terminal cell it covers.
// Two frames whose content differs only in glyph width share a mask exactly
// when the renderer pads by display width.
func cellMask(rendered string) string {
	var b strings.Builder

	inEscape := false
	for _, r := range rendered {
		switch {
		case r == ansiEscape:
			inEscape = true
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		case r == '\n':
			b.WriteRune(r)
		default:
			cell := maskFilled
			if r == ' ' {
				cell = maskEmptyRun
			}

			b.WriteString(strings.Repeat(string(cell), max(lipgloss.Width(string(r)), 1)))
		}
	}

	return b.String()
}

// assertSameGeometry fails when swapping a wide glyph for an equally wide ASCII
// stand-in changes where the table's cells land.
func assertSameGeometry(t *testing.T, render func(glyph string) string) {
	t.Helper()

	wide, plain := cellMask(render(wideGlyph)), cellMask(render(wideStandIn))
	if wide == plain {
		return
	}

	wideLines, plainLines := strings.Split(wide, "\n"), strings.Split(plain, "\n")
	for i := range min(len(wideLines), len(plainLines)) {
		if wideLines[i] != plainLines[i] {
			t.Errorf("row %d drifts under a wide glyph:\n wide: %s\nplain: %s", i, wideLines[i], plainLines[i])
		}
	}

	if len(wideLines) != len(plainLines) {
		t.Errorf("row count differs: %d with a wide glyph, %d without", len(wideLines), len(plainLines))
	}
}

func TestBranchListAlignsUnderWideGlyphs(t *testing.T) {
	t.Parallel()

	assertSameGeometry(t, func(glyph string) string {
		m := New(nil, 1)
		m.branches = []models.BranchInfo{
			{Name: "main", IsCurrent: true},
			{Name: "feat/" + glyph + "-rocket"},
			{Name: "plain-ascii"},
		}

		return renderPanel(m, panelBranches)
	})
}

func TestStashListAlignsUnderWideGlyphs(t *testing.T) {
	t.Parallel()

	now := time.Now()

	assertSameGeometry(t, func(glyph string) string {
		m := New(nil, 1)
		m.stashes = []models.StashDetail{
			{Index: 0, Message: "On main: ship " + glyph + " the parser", Date: now},
			{Index: 1, Message: "On main: plain ascii message", Date: now},
		}

		return renderPanel(m, panelStashes)
	})
}

func TestPRListAlignsUnderWideGlyphs(t *testing.T) {
	t.Parallel()

	assertSameGeometry(t, func(glyph string) string {
		m := New(nil, 1)
		m.prs = []forge.PullRequest{
			{Number: 1, Title: glyph + " emoji title", State: "OPEN", HeadRef: "feature-1"},
			{Number: 22, Title: "plain title", State: "OPEN", HeadRef: "feature-2"},
			{Number: 333, Title: "draft " + glyph, State: "OPEN", IsDraft: true, HeadRef: "feature-3"},
		}

		return renderPanel(m, panelPeers)
	})
}

func TestWorktreeListAlignsUnderWideGlyphs(t *testing.T) {
	t.Parallel()

	assertSameGeometry(t, func(glyph string) string {
		m := New(nil, 1)
		m.worktrees = []models.WorktreeInfo{
			{Path: "/repos/app", Branch: "main"},
			{Path: "/repos/app-" + glyph, Branch: "feat/" + glyph},
		}

		return renderPanel(m, panelPeers)
	})
}

func TestPadCellClampsWideGlyphs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		width int
	}{
		{"wide glyph fits exactly", wideGlyph + wideGlyph, 4},
		{"wide glyph overflows by one cell", wideGlyph + wideGlyph + wideGlyph, 5},
		{"ascii shorter than width", "ab", 6},
		{"warning marker suffix", "abc" + warnSuffix, 8},
		{"current branch marker", currentBranchPrefix + "main", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lipgloss.Width(padCell(tt.text, tt.width)); got != tt.width {
				t.Errorf("padCell(%q, %d) is %d cells wide, want %d", tt.text, tt.width, got, tt.width)
			}
		})
	}
}
