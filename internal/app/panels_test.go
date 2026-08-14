//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// focusedModel returns a model sitting in the focused repo view on a repo with
// something in every panel.
func focusedModel(width, height int) Model {
	now := time.Now()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = width, height
	m.loading = false
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/dev/alpha"
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {
			Path: "/dev/alpha", VCSType: models.VCSTypeGit, Branch: mainBranchName,
			Upstream: "origin/main", Ahead: 1, StashCount: 2, LastModified: now,
			NotesFiles: []models.NoteFile{{Name: ".doing", FirstLine: "wip"}},
		},
	}
	m.repoPaths = []string{"/dev/alpha"}
	m.branches = []models.BranchInfo{{Name: mainBranchName, Upstream: "origin/main", Ahead: 1, IsCurrent: true}}
	m.stashes = []models.StashDetail{{Index: 0, Message: "On main: spike", Date: now}}
	m.worktrees = []models.WorktreeInfo{
		{Path: "/dev/alpha", Branch: mainBranchName},
		{Path: "/dev/alpha-thing", Branch: "feature/thing"},
	}
	m.prs = []models.PRInfo{{Number: 9, Title: "Add a thing", State: "OPEN", HeadRef: "feature/thing"}}
	m.notesFiles = []models.NoteFileContent{{Name: ".doing", Content: "# 2026-08-05\n\nfinish the grid"}}
	m.focusedPanel = panelBranches
	m.updateFilteredPaths()

	return m
}

func TestEveryPanelShowsContentWithoutAKeypress(t *testing.T) {
	t.Parallel()

	grid := plainText(focusedModel(200, 50).renderPanelGrid())
	want := []string{
		"↑1 vs origin/main",
		"[b]ranches (1)", mainBranchName,
		"P[e]ers (1)", "alpha-thing",
		"S[t]ashes (1)", "On main: spike",
		"[n]otes (1)", ".doing",
	}

	for _, w := range want {
		if !strings.Contains(grid, w) {
			t.Errorf("grid is missing %q; every data set must be visible on arrival:\n%s", w, grid)
		}
	}
}

func TestLetterKeysJumpToPanels(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 45)
	panels := m.panelSet(60)

	for _, p := range panels {
		next, _ := m.handleDetailKey(keyPress(rune(p.key[0])))
		jumped := mustModel(t, next)
		if jumped.focusedPanel != p.id {
			t.Errorf("key %q focused panel %v, want %v", p.key, jumped.focusedPanel, p.id)
		}
		if jumped.detailCursor != 0 {
			t.Errorf("key %q left the cursor at %d, want a reset", p.key, jumped.detailCursor)
		}
	}
}

func TestJJRepoHasNoStashesPanel(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 45)
	summary := m.summaries["/dev/alpha"]
	summary.VCSType = models.VCSTypeJJ
	m.summaries["/dev/alpha"] = summary
	m.stashes = nil

	for _, p := range m.panelSet(60) {
		if p.id == panelStashes {
			t.Error("jj has no stashes, so the panel should be absent rather than explaining itself")
		}
	}
}

// Relevance decides only the room left over once every panel that wants one has
// had an equal share, so a high-scoring panel cannot take the column outright.
func TestRelevanceDecidesTheRoomBeyondAFairShare(t *testing.T) {
	t.Parallel()

	busy := panelContent{id: panelPeers, relevance: relevanceUrgent, rows: make([]string, 12)}
	quiet := panelContent{id: panelStashes, relevance: relevanceIdle, rows: make([]string, 2)}

	heights := distributePanelHeights([]panelContent{busy, quiet}, -1, 24)
	if heights[0] <= heights[1] {
		t.Errorf("heights %v: the panel with more to say must get the leftover", heights)
	}

	minimum := panelChromeHeight + 1
	tight := distributePanelHeights([]panelContent{busy, quiet}, 0, minimum*2)
	for i, h := range tight {
		if h < minimum {
			t.Errorf("panel %d compressed to %d lines; none may drop below its border plus a line", i, h)
		}
	}
}

// Two panels that both want more than the column holds split it evenly rather
// than letting the higher relevance score starve the other down to one row.
func TestContendingPanelsSplitTheColumnEvenly(t *testing.T) {
	t.Parallel()

	quiet := panelContent{id: panelStashes, relevance: relevanceIdle, rows: make([]string, 10)}
	busy := panelContent{id: panelPeers, relevance: relevanceUrgent, rows: make([]string, 10)}

	heights := distributePanelHeights([]panelContent{quiet, busy}, 1, 18)
	if heights[0] != heights[1] {
		t.Errorf("heights %v: equal demands on a short column must be met equally", heights)
	}
}

func TestFocusedPanelIsServedBeforeRelevance(t *testing.T) {
	t.Parallel()

	quiet := panelContent{id: panelStashes, relevance: relevanceIdle, rows: make([]string, 10)}
	busy := panelContent{id: panelPeers, relevance: relevanceUrgent, rows: make([]string, 1)}

	heights := distributePanelHeights([]panelContent{quiet, busy}, 0, 20)
	if heights[0] < panelChromeHeight+panelTitleHeight+len(quiet.rows) {
		t.Errorf("heights %v: the focused panel must fit its own content once the shares are out",
			heights)
	}
}

func TestDetailPaneDescribesTheSelectedItem(t *testing.T) {
	t.Parallel()

	m := focusedModel(200, 50)
	m.branchDetail = models.BranchDetail{
		Branch:  m.branches[0],
		Commits: []models.CommitInfo{{ShortHash: "d2c794a", Subject: "expand and center tables"}},
	}

	if got := plainText(m.renderPanelDetail(60, 20)); !strings.Contains(got, "expand and center tables") {
		t.Errorf("branch detail pane is missing its commits:\n%s", got)
	}

	m.focusedPanel = panelNotes
	if got := plainText(m.renderPanelDetail(60, 20)); !strings.Contains(got, "finish the grid") {
		t.Errorf("note detail pane is missing the file body:\n%s", got)
	}
}

func TestNotesPeekSkipsHeadingsAndBlanks(t *testing.T) {
	t.Parallel()

	if got := firstContentLine("# 2026-08-05\n\n  finish the grid\n"); got != "finish the grid" {
		t.Errorf("note peek = %q, want the first line that carries content", got)
	}
	if got := firstContentLine("\n\n"); got != "" {
		t.Errorf("note peek = %q, want nothing for a file with no content", got)
	}
}

// TestDetailPaneFocus_EnterMovesInAndEscMovesBack pins the lazygit focus
// model: enter hands the keyboard to the detail pane, movement scrolls the
// text instead of the rows, and esc gives it back.
func TestDetailPaneFocus_EnterMovesInAndEscMovesBack(t *testing.T) {
	t.Parallel()

	m := focusedModel(200, 50)
	m.focusedPanel = panelNotes
	m.notesFiles = []models.NoteFileContent{
		{Name: ".doing", Content: strings.Repeat("a line of notes\n", 200)},
	}

	focused := mustModel(t, mustUpdate(t, &m, tea.KeyPressMsg{Code: tea.KeyEnter}))
	if !focused.detailFocused {
		t.Fatal("enter did not move focus into the detail pane")
	}

	scrolled := mustModel(t, mustUpdate(t, &focused, tea.KeyPressMsg{Code: 'j', Text: "j"}))
	if scrolled.detailScroll != 1 {
		t.Errorf("j scrolled the pane to %d, want 1", scrolled.detailScroll)
	}
	if scrolled.detailCursor != focused.detailCursor {
		t.Error("j moved the row cursor while the detail pane held focus")
	}

	back := mustModel(t, mustUpdate(t, &scrolled, tea.KeyPressMsg{Code: tea.KeyEscape}))
	if back.detailFocused || back.detailScroll != 0 {
		t.Errorf("esc left focus=%v scroll=%d, want the panel column at the top",
			back.detailFocused, back.detailScroll)
	}
	if back.viewMode != ViewModeRepoDetail {
		t.Errorf("esc left the focused view entirely, got %v", back.viewMode)
	}
}

func TestDetailPaneFocus_ScrollStopsAtTheEnd(t *testing.T) {
	t.Parallel()

	m := focusedModel(200, 50)
	m.detailFocused = true
	m.detailScroll = m.maxDetailScroll()

	further := m.scrollDetailPane(1)
	if further.detailScroll != m.detailScroll {
		t.Errorf("scrolled past the last line to %d, want %d", further.detailScroll, m.detailScroll)
	}
}

// TestJumpDiffFile_MovesBetweenFileBoundaries covers both diff renderers this
// app wires diff.external to: git's own "diff --git a/" header and
// difftastic's bare "path --- language" header.
func TestJumpDiffFile_MovesBetweenFileBoundaries(t *testing.T) {
	t.Parallel()

	filler := strings.Repeat("context line\n", 30)
	diff := "diff --git a/one.go b/one.go\n" + filler +
		"two.go --- Go\n" + filler +
		"diff --git a/three.go b/three.go\n" + filler

	m := focusedModel(200, 20)
	m.focusedPanel = panelStashes
	m.stashes = []models.StashDetail{{Index: 0, Message: "wip"}}
	m.stashFullDiff = true
	m.stashDiff = map[int]string{0: diff}
	m.detailFocused = true
	m.detailScroll = 0

	afterFirst := m.jumpDiffFile(1)
	if afterFirst.detailScroll <= 0 {
		t.Fatalf("] did not scroll to the first file's header, got %d", afterFirst.detailScroll)
	}

	afterSecond := afterFirst.jumpDiffFile(1)
	if afterSecond.detailScroll <= afterFirst.detailScroll {
		t.Fatalf("] did not advance past %d, landed on %d", afterFirst.detailScroll, afterSecond.detailScroll)
	}

	afterThird := afterSecond.jumpDiffFile(1)
	if afterThird.detailScroll <= afterSecond.detailScroll {
		t.Fatalf("] did not advance past %d, landed on %d", afterSecond.detailScroll, afterThird.detailScroll)
	}

	afterFourth := afterThird.jumpDiffFile(1)
	if afterFourth.detailScroll != afterThird.detailScroll {
		t.Errorf("] past the last file moved from %d to %d, want no movement",
			afterThird.detailScroll, afterFourth.detailScroll)
	}

	back := afterThird.jumpDiffFile(-1)
	if back.detailScroll != afterSecond.detailScroll {
		t.Errorf("[ landed on %d, want the previous boundary %d", back.detailScroll, afterSecond.detailScroll)
	}
}

func mustUpdate(t *testing.T, m *Model, msg tea.Msg) tea.Model {
	t.Helper()

	updated, _ := m.Update(msg)

	return updated
}

// TestGrid_FillsTheTerminalExactly pins the geometry bug the grid had: panel
// boxes never counted their own title line, so the column ran past the bottom.
// The column may now end short of the detail pane, because padding a box with
// blank rows costs more than the ragged edge does, but it must never overrun.
func TestGrid_FillsTheTerminalExactly(t *testing.T) {
	t.Parallel()

	sizes := [][2]int{{100, 30}, {160, 40}, {200, 60}, {80, 24}}

	for _, size := range sizes {
		t.Run(strconv.Itoa(size[0])+"x"+strconv.Itoa(size[1]), func(t *testing.T) {
			t.Parallel()

			m := focusedModel(size[0], size[1])

			if got := strings.Count(m.renderPanelGrid(), "\n") + 1; got != size[1] {
				t.Errorf("grid rendered %d lines into a %d-line terminal", got, size[1])
			}

			width := m.gridWidth()
			if gridStacked(m.isCompact(), width) {
				return
			}

			side := panelSideWidth(width, false)
			column := m.renderPanelColumn(m.panelSet(side-panelBorderWidth), 1, side, m.height-gridChromeHeight)

			if got, limit := strings.Count(column, "\n")+1, m.height-gridChromeHeight; got > limit {
				t.Errorf("panel column is %d lines in a %d-line body", got, limit)
			}
		})
	}
}

// A row wider than its panel wraps inside the box and pushes the column past
// the bottom of the terminal, so every row is cut to fit. A branch name is cut
// from the left, because its head is the part every sibling shares.
func TestPanelRowsFitTheirBoxAndKeepTheBranchHead(t *testing.T) {
	t.Parallel()

	const longBranch = "kyleking/eng-1234-all-dependencies-56744d620d"

	sizes := [][2]int{{80, 24}, {100, 30}, {160, 40}, {220, 50}}

	for _, size := range sizes {
		t.Run(strconv.Itoa(size[0])+"x"+strconv.Itoa(size[1]), func(t *testing.T) {
			t.Parallel()

			m := focusedModel(size[0], size[1])
			m.branches = append(m.branches, models.BranchInfo{Name: longBranch})
			m.prs = []models.PRInfo{{Number: 11, Title: strings.Repeat("bump the dependencies ", 6)}}

			width := panelSideWidth(m.gridWidth(), false) - panelBorderWidth

			var branchRows string
			for _, p := range m.panelSet(width) {
				for _, row := range p.rows {
					if got := lipgloss.Width(row); got > width {
						t.Errorf("a %s row is %d cells wide in a %d-cell panel: %q", p.title, got, width, row)
					}
				}
				if p.id == panelBranches {
					branchRows = plainText(strings.Join(p.rows, "\n"))
				}
			}

			if !strings.Contains(branchRows, "kyleking") {
				t.Errorf("the branch name lost its head, which is the part that identifies it:\n%s", branchRows)
			}
		})
	}
}

func TestGridWidth_UsesMoreOfAWideTerminal(t *testing.T) {
	t.Parallel()

	if got := focusedModel(200, 50).gridWidth(); got <= maxContentWidth {
		t.Errorf("grid width at 200 cells is %d; it must outgrow the single-column cap of %d",
			got, maxContentWidth)
	}

	if got := focusedModel(400, 50).gridWidth(); got != gridMaxWidth {
		t.Errorf("grid width at 400 cells is %d, want the cap of %d", got, gridMaxWidth)
	}
}
