//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestBreakpointForSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width, height int
		want          breakpoint
	}{
		{name: "80x24 is compact", width: 80, height: 24, want: breakpointCompact},
		{name: "99 columns is still compact", width: 99, height: 40, want: breakpointCompact},
		{name: "100 columns is standard", width: 100, height: 35, want: breakpointStandard},
		{name: "159 columns is standard", width: 159, height: 35, want: breakpointStandard},
		{name: "160 columns is wide", width: 160, height: 50, want: breakpointWide},
		{name: "a short terminal falls back to standard", width: 220, height: 19, want: breakpointStandard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := breakpointFor(tt.width, tt.height); got != tt.want {
				t.Errorf("breakpointFor(%d, %d) = %s, want %s", tt.width, tt.height, got, tt.want)
			}
		})
	}
}

// compactModel returns a three-repo fleet with a signal on each record.
func compactModel(width, height int) Model {
	now := time.Now()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = width, height
	m.loading = false
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {Path: "/dev/alpha", Branch: "main", StashCount: 2, LastModified: now},
		"/dev/bravo": {
			Path: "/dev/bravo", Branch: "feature/login", Staged: 1, Untracked: 1, LastModified: now,
			NotesFiles: []models.NoteFile{{Name: ".doing", FirstLine: "wip"}},
		},
		"/dev/charlie": {Path: "/dev/charlie", Branch: "trunk", LastModified: now},
	}
	m.repoPaths = []string{"/dev/alpha", "/dev/bravo", "/dev/charlie"}
	m.updateFilteredPaths()

	return m
}

func TestCompactLayoutRendersTwoLineRecords(t *testing.T) {
	t.Parallel()

	m := compactModel(80, 24)
	lines := strings.Split(strings.TrimRight(m.renderTable(m.listBodyHeight()), "\n"), "\n")

	if len(lines) != len(m.filteredPaths)*compactRowHeight {
		t.Fatalf("got %d lines for %d repos, want %d",
			len(lines), len(m.filteredPaths), len(m.filteredPaths)*compactRowHeight)
	}

	if strings.Contains(plainText(lines[0]), "NAME") {
		t.Error("the compact layout has no column header; the record itself is the row")
	}

	if !strings.Contains(plainText(lines[1]), "2 stashes") {
		t.Errorf("signal line does not carry the stash count: %q", plainText(lines[1]))
	}

	if !strings.Contains(plainText(lines[3]), "1 note") {
		t.Errorf("signal line does not carry the notes count: %q", plainText(lines[3]))
	}
}

func TestCompactLayoutFitsEightyColumns(t *testing.T) {
	t.Parallel()

	m := compactModel(80, 24)
	limit := contentWidth(80)

	for i, line := range strings.Split(m.renderScreen(), "\n") {
		if got := lipgloss.Width(line); got > limit+frameLeftPad(80, contentWidth(80))*2 {
			t.Errorf("line %d is %d cells wide at 80 columns: %q", i, got, plainText(line))
		}
	}
}

func TestStandardLayoutKeepsTheColumnHeader(t *testing.T) {
	t.Parallel()

	m := compactModel(120, 35)
	first := strings.Split(m.renderTable(m.listBodyHeight()), "\n")[0]

	if !strings.Contains(plainText(first), "NAME") {
		t.Errorf("standard layout dropped its header: %q", plainText(first))
	}
}

func TestCompactGridStacksEveryPanel(t *testing.T) {
	t.Parallel()

	m := compactModel(80, 24)
	m.selectedRepo = "/dev/alpha"
	m.branches = []models.BranchInfo{{Name: mainBranchName, IsCurrent: true}}
	m.prs = []models.PRInfo{{Number: 4, Title: "Compact", State: "OPEN"}}

	grid := plainText(m.renderPanelGrid())
	// The Status title is built rather than spelled out: the bracketed form is
	// pinned by TestEveryPanelShowsContentWithoutAKeypress, and what matters
	// here is only that every panel made it into the stack.
	status := markHotkey("Status", panelKeys[panelStatus])
	for _, want := range []string{status, "[B]ranches (1)", "[P]Rs (1)"} {
		if !strings.Contains(grid, want) {
			t.Errorf("compact grid is missing %q:\n%s", want, grid)
		}
	}
}
