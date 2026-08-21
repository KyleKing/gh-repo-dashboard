package region_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/region"
)

const width = 60

func plainStyles() region.Styles {
	return region.Styles{Rule: lipgloss.NewStyle(), Label: lipgloss.NewStyle()}
}

func block() region.Region {
	return region.Region{
		Title:   "acme/alpha · #11",
		Head:    []region.Fact{{Label: "Reviewers", Value: "erin"}, {Label: "Checks", Value: "3 ok"}},
		Section: "description",
		Body:    []string{"one", "two", "three"},
		Caption: "Bump the deps",
	}
}

// TestRenderIsTotal covers the invariant the table above a region depends on:
// whatever the region holds, it occupies exactly the height it was given, so
// data landing underneath the cursor cannot move the rows above it.
func TestRenderIsTotal(t *testing.T) {
	t.Parallel()

	for _, height := range []int{1, 2, 4, 8, 30} {
		got := block().Render(plainStyles(), width, height)
		if len(got) != height {
			t.Errorf("height %d: rendered %d lines", height, len(got))
		}
		for i, line := range got {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("height %d line %d is %d cells wide: %q", height, i, w, line)
			}
		}
	}
}

// TestClosingRuleSurvivesATightRegion covers the trim: a region with no room
// for its body still has to say where it ended, or the table below runs into
// the footer with nothing marking the seam.
func TestClosingRuleSurvivesATightRegion(t *testing.T) {
	t.Parallel()

	got := block().Render(plainStyles(), width, 3)
	if last := got[len(got)-1]; !strings.Contains(last, "Bump the deps") {
		t.Errorf("the closing caption should survive a trim, got %q", last)
	}
}

// TestBodyElidesItsMiddle covers what a long body loses: the ends a reader
// scans first survive and the count of what went is stated.
func TestBodyElidesItsMiddle(t *testing.T) {
	t.Parallel()

	long := block()
	long.Body = []string{"first", "b", "c", "d", "e", "f", "last"}

	joined := strings.Join(long.Render(plainStyles(), width, 8), "\n")
	for _, want := range []string{"first", "last", "more lines"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in:\n%s", want, joined)
		}
	}
}

// TestHeadPacksToTheColumnsTheCallerAllows covers the density knob: facts pair
// up only where the caller asked for it and the width can carry it.
func TestHeadPacksToTheColumnsTheCallerAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		columns int
		width   int
		want    int
	}{
		{"one column by default", 0, 120, 2},
		{"paired when asked and wide", 2, 120, 1},
		{"narrow falls back to one", 2, 40, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := block()
			b.HeadColumns = tt.columns
			b.Body = nil
			b.Section = ""

			// Title, head rows, and the closing rule: everything but the head
			// is one line, so the head's own line count is what is left.
			lines := b.Render(plainStyles(), tt.width, 2+tt.want)
			head := 0
			for _, line := range lines {
				if strings.HasPrefix(line, "Reviewers") || strings.HasPrefix(line, "Checks") {
					head++
				}
			}

			if head != tt.want {
				t.Errorf("expected %d head lines, got %d:\n%s", tt.want, head, strings.Join(lines, "\n"))
			}
		})
	}
}
