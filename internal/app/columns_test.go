//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// detailTableModel returns a model at termWidth with one populated row in each
// detail tab, so every table renders a header plus at least two rows.
func detailTableModel(termWidth int) Model {
	now := time.Now()

	m := New(nil, 1)
	m.width = termWidth
	m.height = 40
	m.branches = []models.BranchInfo{
		{Name: "main", Upstream: "origin/main", IsCurrent: true, LastCommit: now},
		{Name: "feature/a-rather-long-branch-name", Upstream: "origin/feature/a-rather-long-branch-name"},
	}
	m.stashes = []models.StashDetail{
		{Index: 0, Message: "On main: spike the parser", Date: now},
		{Index: 1, Message: "On main: revert the spike", Date: now},
	}
	m.worktrees = []models.WorktreeInfo{
		{Path: "/repos/app", Branch: "main"},
		{Path: "/repos/app-feature", Branch: "feature/a-rather-long-branch-name", IsLocked: true},
	}
	m.prs = []forge.PullRequest{
		{Number: 1, Title: "Add the login flow", State: "OPEN", HeadRef: "feature/login"},
		{
			Number: 4321, State: "OPEN", HeadRef: "feature/b",
			Title: "A title long enough to need truncating in most columns",
		},
	}

	return m
}

// tableRows splits a rendered table into its header and body rows.
func tableRows(rendered string) []string {
	return strings.Split(strings.TrimRight(rendered, "\n"), "\n")
}

func TestDetailTablesFitEveryBreakpoint(t *testing.T) {
	t.Parallel()

	tables := map[string]func(Model) string{
		"branches":  panelRenderer(panelBranches),
		"stashes":   panelRenderer(panelStashes),
		"worktrees": panelRenderer(panelPeers),
	}

	for _, termWidth := range []int{80, 120, 220} {
		for name, render := range tables {
			t.Run(fmt.Sprintf("%s at %d", name, termWidth), func(t *testing.T) {
				t.Parallel()

				m := detailTableModel(termWidth)
				rows := tableRows(render(m))
				limit := contentWidth(termWidth)

				header := lipgloss.Width(rows[0])
				if header > limit {
					t.Errorf("width %d: %s header is %d cells, content width is %d", termWidth, name, header, limit)
				}

				for i, row := range rows[1:] {
					if got := lipgloss.Width(row); got != header {
						t.Errorf("width %d: %s row %d is %d cells, header is %d", termWidth, name, i, got, header)
					}
				}
			})
		}
	}
}

func TestDetailTablesHideColumnsBeforeClipping(t *testing.T) {
	t.Parallel()

	narrow := fitDetailCols(branchColSpecs, 80)
	if narrow.Hidden == 0 {
		t.Fatal("expected the branch tab to hide columns at 80 terminal columns")
	}

	if narrow.Width(colBranchName) == 0 || narrow.Width(colBranchState) == 0 {
		t.Error("branch name and status must never hide")
	}

	if narrow.Width(colUpstream) != 0 {
		t.Error("upstream mirrors the branch name and should be the first to go")
	}

	wide := fitDetailCols(branchColSpecs, 220)
	if wide.Hidden != 0 {
		t.Errorf("hid %d branch columns at 220 terminal columns, want 0", wide.Hidden)
	}
}

func TestRenderCellsFallsBackToTheBaseStyle(t *testing.T) {
	t.Parallel()

	layout := table.Fit([]table.Column{
		{Key: "a", Title: "A", Min: 4},
		{Key: "b", Title: "B", Min: 4},
	}, 10)

	base := lipgloss.NewStyle()
	cells := renderCells(layout, map[string]string{"a": "x", "b": "y"}, nil, &base)

	if len(cells) != 2 {
		t.Fatalf("got %d cells, want 2", len(cells))
	}

	for i, cell := range cells {
		if got := lipgloss.Width(cell); got != 4 {
			t.Errorf("cell %d is %d cells wide, want 4", i, got)
		}
	}
}
