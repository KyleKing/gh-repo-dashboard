//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
		"[S]tatus", "↑1 vs origin/main",
		"[B]ranches (1)", mainBranchName,
		"[P]Rs (1)", "Add a thing",
		"P[e]ers (1)", "alpha-thing",
		"S[t]ashes (1)", "On main: spike",
		"[N]otes (1)", ".doing",
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

func TestRelevanceGivesABusyPanelMoreRoom(t *testing.T) {
	t.Parallel()

	busy := panelContent{id: panelPRs, relevance: relevanceUrgent, rows: make([]string, 10)}
	quiet := panelContent{id: panelStashes, relevance: relevanceIdle, rows: make([]string, 10)}

	heights := distributePanelHeights([]panelContent{busy, quiet}, -1, 20)
	if heights[0] <= heights[1] {
		t.Errorf("heights %v: the panel with actionable state must get the room", heights)
	}

	minimum := panelChromeHeight + 1
	tight := distributePanelHeights([]panelContent{busy, quiet}, 0, minimum*2)
	for i, h := range tight {
		if h < minimum {
			t.Errorf("panel %d compressed to %d lines; none may drop below its border plus a line", i, h)
		}
	}
}

func TestFocusedPanelIsServedBeforeRelevance(t *testing.T) {
	t.Parallel()

	quiet := panelContent{id: panelStashes, relevance: relevanceIdle, rows: make([]string, 8)}
	busy := panelContent{id: panelPRs, relevance: relevanceUrgent, rows: make([]string, 8)}

	heights := distributePanelHeights([]panelContent{quiet, busy}, 0, 14)
	if heights[0] < panelChromeHeight+len(quiet.rows) {
		t.Errorf("heights %v: the focused panel must fit its own content first", heights)
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

	m.focusedPanel = panelPRs
	m.prDetail = models.PRDetail{PRInfo: m.prs[0], Body: "wires the panel grid"}
	if got := plainText(m.renderPanelDetail(60, 20)); !strings.Contains(got, "wires the panel grid") {
		t.Errorf("PR detail pane is missing the description:\n%s", got)
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

func mustUpdate(t *testing.T, m *Model, msg tea.Msg) tea.Model {
	t.Helper()

	updated, _ := m.Update(msg)

	return updated
}

// TestGrid_FillsTheTerminalExactly pins the two geometry bugs the grid had:
// panel boxes never counted their own title line, so the column ran past the
// bottom, and the lines nothing claimed were left unspent, so it ended ragged
// beside a full-height detail pane.
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

			side := panelSideWidth(width)
			column := m.renderPanelColumn(m.panelSet(side-panelBorderWidth), 1, side, m.height-gridChromeHeight)

			if got, want := strings.Count(column, "\n")+1, m.height-gridChromeHeight; got != want {
				t.Errorf("panel column is %d lines beside a %d-line detail pane", got, want)
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
