//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// peerModel builds a model where alpha and bravo are parallel checkouts of the
// same remote (so both report a peer count) and solo is not.
func peerModel() Model {
	m := New([]string{"/dev"}, 1)
	m.width = 120
	m.height = 30
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/alpha", Branch: mainBranchName, RemoteRepo: "dev/shared"},
		},
		"/dev/bravo": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/bravo", Branch: mainBranchName, RemoteRepo: "dev/shared"},
		},
		"/dev/solo": {RepoSummary: vcs.RepoSummary{Path: "/dev/solo", Branch: mainBranchName, RemoteRepo: "dev/solo"}},
	}
	m.repoPaths = []string{"/dev/alpha", "/dev/bravo", "/dev/solo"}
	m.updateFilteredPaths()

	// alpha has an open PR that bravo holds locally, so bravo counts as a
	// relevant peer.
	m.prMap = map[string]PRMapLoadedMsg{
		"/dev/alpha": {
			Path: "/dev/alpha",
			PRs:  []forge.PullRequest{{Number: 1, HeadRef: "feature-x"}},
		},
	}
	m.peerBranches = map[string][]vcs.BranchInfo{
		"/dev/bravo": {{Name: "feature-x", Upstream: "origin/feature-x"}},
	}

	return m
}

func TestRepoRow_NeedsDoesNotShiftLaterColumns(t *testing.T) {
	t.Parallel()

	m := peerModel()
	layout := layoutRepoCols(contentWidth(m.width))

	busy := m.summaries["/dev/alpha"]
	busy.Ahead = 2
	m.summaries["/dev/alpha"] = busy

	withNeeds := m.renderTableRow(busy, m.needsOf(busy), false, layout)
	quiet := m.summaries["/dev/solo"]
	withoutNeeds := m.renderTableRow(quiet, m.needsOf(quiet), false, layout)

	if got, want := lipgloss.Width(withNeeds), lipgloss.Width(withoutNeeds); got != want {
		t.Errorf("row with needs is %d cells, quiet row is %d; rows must be equal width", got, want)
	}

	if !strings.Contains(plainText(withNeeds), "↑2") {
		t.Fatalf("expected the unpushed count in the row, got %q", withNeeds)
	}
}

func TestRepoRow_MatchesHeaderWidth(t *testing.T) {
	t.Parallel()

	for _, termWidth := range []int{80, 100, 160, 240} {
		m := peerModel()
		m.width = termWidth
		layout := layoutRepoCols(contentWidth(termWidth))

		header := lipgloss.Width(renderRepoHeader(layout))
		alpha := m.summaries["/dev/alpha"]
		row := lipgloss.Width(m.renderTableRow(alpha, m.needsOf(alpha), false, layout))

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

		if total := cursorWidth + layout.Total(); total != width {
			t.Errorf("terminal %d: layout is %d cells, content width is %d", termWidth, total, width)
		}
	}
}

func TestLayoutRepoCols_HidesModifiedThenPRsAndNeverNeeds(t *testing.T) {
	t.Parallel()

	wide := layoutRepoCols(contentWidth(200))
	if wide.Hidden != 0 {
		t.Errorf("hid %d columns at 200 columns, want 0", wide.Hidden)
	}

	narrow := layoutRepoCols(cursorWidth + 76)
	if narrow.Width(colModified) != 0 || narrow.Width(colPR) == 0 {
		t.Error("MODIFIED should be the first column to hide, before PRs")
	}
	if narrow.Marker() != "…+1" {
		t.Errorf("marker = %q, want %q", narrow.Marker(), "…+1")
	}

	cramped := layoutRepoCols(cursorWidth + 20)
	if cramped.Width(colName) == 0 || cramped.Width(colBranch) == 0 || cramped.Width(colNeeds) == 0 {
		t.Error("name, branch, and needs must never be hidden")
	}
}

func TestLayoutRepoCols_CapsNameAndBranchOnWideTerminals(t *testing.T) {
	t.Parallel()

	layout := layoutRepoCols(listWidth(220))
	if layout.Width(colName) != 28 || layout.Width(colBranch) != 24 {
		t.Errorf("name and branch are %d and %d cells at 220 columns, want 28 and 24",
			layout.Width(colName), layout.Width(colBranch))
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
		{name: "ascii truncates", input: "feature/login", maxLen: 8, want: "feature…"},
		{name: "multibyte fits", input: "café-ünïcode", maxLen: 12, want: "café-ünïcode"},
		{name: "multibyte truncates on width", input: "café-ünïcode", maxLen: 8, want: "café-ün…"},
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

// TestList_HidesAWorktreeWhoseParentIsAlsoDiscovered keeps a linked checkout
// out of the fleet count: it is a place inside its parent repo, and the
// parent's Peers panel already names it.
func TestList_HidesAWorktreeWhoseParentIsAlsoDiscovered(t *testing.T) {
	t.Parallel()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = 160, 40
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {RepoSummary: vcs.RepoSummary{Path: "/dev/alpha", Branch: mainBranchName}},
		"/dev/alpha-wt": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/alpha-wt", Branch: "feature", ParentPath: "/dev/alpha"},
		},
		"/dev/orphan": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/orphan", Branch: "feature", ParentPath: "/elsewhere/parent"},
		},
	}
	m.repoPaths = []string{"/dev/alpha", "/dev/alpha-wt", "/dev/orphan"}
	m.updateFilteredPaths()

	if got := m.filteredPaths; len(got) != 2 {
		t.Fatalf("listed %v, want the worktree dropped and the orphan kept", got)
	}

	for _, path := range m.filteredPaths {
		if path == "/dev/alpha-wt" {
			t.Error("a worktree whose parent is on screen must not be listed as its own repo")
		}
	}
}
