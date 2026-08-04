//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// peerModel builds a model where alpha and bravo are parallel checkouts of the
// same remote (so both report a peer count) and solo is not.
func peerModel() Model {
	m := New([]string{"/dev"}, 1)
	m.width = 120
	m.height = 30
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {Path: "/dev/alpha", Branch: mainBranchName, RemoteRepo: "dev/shared"},
		"/dev/bravo": {Path: "/dev/bravo", Branch: mainBranchName, RemoteRepo: "dev/shared"},
		"/dev/solo":  {Path: "/dev/solo", Branch: mainBranchName, RemoteRepo: "dev/solo"},
	}
	m.repoPaths = []string{"/dev/alpha", "/dev/bravo", "/dev/solo"}
	m.updateFilteredPaths()

	return m
}

func TestRepoRow_PeerCountDoesNotShiftLaterColumns(t *testing.T) {
	t.Parallel()

	m := peerModel()
	layout := layoutRepoCols(contentWidth(m.width))

	withPeers := m.renderTableRow(m.summaries["/dev/alpha"], false, layout)
	withoutPeers := m.renderTableRow(m.summaries["/dev/solo"], false, layout)

	if got, want := lipgloss.Width(withPeers), lipgloss.Width(withoutPeers); got != want {
		t.Errorf("peer row width = %d, peerless row width = %d; rows must be equal width", got, want)
	}

	if !strings.Contains(withPeers, "⧉1") {
		t.Fatalf("expected a peer count in the row, got %q", withPeers)
	}
}

func TestRepoRow_MatchesHeaderWidth(t *testing.T) {
	t.Parallel()

	for _, termWidth := range []int{80, 100, 160, 240} {
		m := peerModel()
		m.width = termWidth
		layout := layoutRepoCols(contentWidth(termWidth))

		header := lipgloss.Width(renderRepoHeader(layout))
		row := lipgloss.Width(m.renderTableRow(m.summaries["/dev/alpha"], false, layout))

		if header != row {
			t.Errorf("width %d: header is %d cells, row is %d cells", termWidth, header, row)
		}
	}
}

func TestLayoutRepoCols_FitsWithinContentWidth(t *testing.T) {
	t.Parallel()

	for _, termWidth := range []int{80, 100, 120, 200, 300} {
		width := contentWidth(termWidth)
		layout := layoutRepoCols(width)

		total := cursorWidth
		for i, c := range layout.cols {
			if i > 0 {
				total += colGutter
			}
			total += layout.width(c.col)
		}

		if total != width {
			t.Errorf("terminal %d: layout is %d cells, content width is %d", termWidth, total, width)
		}
	}
}

func TestLayoutRepoCols_DropsLowPriorityColumnsWhenNarrow(t *testing.T) {
	t.Parallel()

	wide := layoutRepoCols(contentWidth(200))
	if wide.width(colTemplate) == 0 {
		t.Error("template column should survive at 200 columns")
	}

	narrow := layoutRepoCols(minRowWidth(repoColSpecs) - 1)
	if narrow.width(colTemplate) != 0 {
		t.Error("template column should be dropped once the row no longer fits")
	}
	if narrow.width(colName) == 0 || narrow.width(colBranch) == 0 || narrow.width(colStatus) == 0 {
		t.Error("name, branch, and status must never be dropped")
	}
}

func TestContentWidth_CapsAndFloors(t *testing.T) {
	t.Parallel()

	if got := contentWidth(400); got != maxContentWidth {
		t.Errorf("contentWidth(400) = %d, want %d", got, maxContentWidth)
	}
	if got := contentWidth(20); got != minContentWidth {
		t.Errorf("contentWidth(20) = %d, want %d", got, minContentWidth)
	}
	if got := contentWidth(100); got != 100-2*frameGutter {
		t.Errorf("contentWidth(100) = %d, want %d", got, 100-2*frameGutter)
	}
}

func TestTruncate_MeasuresDisplayWidthNotBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "ascii fits", input: "main", maxLen: 10, want: "main"},
		{name: "ascii truncates", input: "feature/login", maxLen: 8, want: "featu..."},
		{name: "multibyte fits", input: "café-ünïcode", maxLen: 12, want: "café-ünïcode"},
		{name: "multibyte truncates on width", input: "café-ünïcode", maxLen: 8, want: "café-..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if w := lipgloss.Width(got); w > tt.maxLen {
				t.Errorf("truncate(%q, %d) rendered %d cells wide", tt.input, tt.maxLen, w)
			}
		})
	}
}

func TestFrame_CentersContentAndLeavesBlankLinesEmpty(t *testing.T) {
	t.Parallel()

	framed := frame("abc\n\ndef", 200)
	lines := strings.Split(framed, "\n")

	pad := frameLeftPad(200)
	if pad == 0 {
		t.Fatal("expected a non-zero indent at 200 columns")
	}
	if !strings.HasPrefix(lines[0], strings.Repeat(" ", pad)+"abc") {
		t.Errorf("line 0 = %q, want %d-space indent", lines[0], pad)
	}

	want := pad + contentWidth(200)
	for i, line := range lines {
		if got := lipgloss.Width(line); got != want {
			t.Errorf("line %d is %d cells wide, want %d; short lines leave stale cells on screen", i, got, want)
		}
	}
}
