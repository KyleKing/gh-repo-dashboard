//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// TestExpandRegion_KeepsTheFrameHeightFixed pins the bug the region used to
// have: its lines were added to a body that had already been sized, so the
// footer slid off the bottom whenever the whole repo set fitted on screen. Only
// one repo of the fleet carries notes, branches, and pull requests, so a frame
// that sized itself from the selection would fail here.
func TestExpandRegion_KeepsTheFrameHeightFixed(t *testing.T) {
	t.Parallel()

	sizes := []struct {
		name          string
		width, height int
	}{
		{name: "compact", width: 80, height: 24},
		{name: "standard", width: 140, height: 20},
		{name: "wide", width: 180, height: 45},
	}

	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			t.Parallel()

			m := compactModel(size.width, size.height)
			m.notesPreview = map[string][]models.NoteFileContent{
				"/dev/bravo": {{Name: ".doing", Content: strings.Repeat("a wide line of notes\n", 40)}},
			}
			m.prMap = map[string]PRMapLoadedMsg{
				"/dev/bravo": {
					Path:     "/dev/bravo",
					PRs:      []forge.PullRequest{{Number: 42, Title: "Add login flow"}},
					Branches: []vcs.BranchInfo{{Name: featureBranchName, Ahead: 2}},
				},
			}

			for _, open := range []bool{false, true} {
				m.expandOpen = open

				for cursor := range m.filteredPaths {
					m.cursor = cursor

					got := strings.Count(m.renderScreen(), "\n") + 1
					if got != size.height {
						t.Errorf("region open=%v, cursor=%d: rendered %d lines, want %d",
							open, cursor, got, size.height)
					}
				}
			}
		})
	}
}

// TestExpandRegion_SizesToItsContent covers the region's height: only as many
// lines as it holds, never past the cap, so a repo with little to say leaves the
// table the rest.
func TestExpandRegion_SizesToItsContent(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 40)
	m.expandOpen = true

	m.cursor = 0
	quiet := len(m.expandedLines())

	m.cursor = 1
	m.notesPreview = map[string][]models.NoteFileContent{
		"/dev/bravo": {{Name: ".doing", Content: "one\ntwo\nthree"}},
	}
	noted := len(m.expandedLines())

	if quiet >= noted {
		t.Errorf("a repo with notes holds %d lines, one without holds %d; the region must follow its content",
			noted, quiet)
	}

	limit := m.expandHeight(m.listBodyHeight())
	if noted > limit {
		t.Errorf("region is %d lines, over its %d-line cap", noted, limit)
	}

	if tall := len(m.expandLines(120, 4)); tall > 4 {
		t.Errorf("region drew %d lines under a 4-line cap", tall)
	}
}

// TestExpandRegion_SitsBelowTheTable pins the redesign's shape: the table leads,
// the region follows it, and the region's own sections keep a fixed order.
func TestExpandRegion_SitsBelowTheTable(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 30)
	m.expandOpen = true
	m.cursor = 1
	m.notesPreview = map[string][]models.NoteFileContent{
		"/dev/bravo": {{Name: ".doing", Content: "wip"}},
	}
	bravo := m.summaries["/dev/bravo"]
	bravo.OpenPRs = []forge.PullRequest{{Number: 42, Title: "Add login flow"}}
	m.summaries["/dev/bravo"] = bravo
	m.prMap = map[string]PRMapLoadedMsg{
		"/dev/bravo": {
			Path:     "/dev/bravo",
			PRs:      []forge.PullRequest{{Number: 42, Title: "Add login flow"}},
			Branches: []vcs.BranchInfo{{Name: mainBranchName}, {Name: featureBranchName, Ahead: 2}},
		},
	}

	body := plainText(m.renderListBody())

	// Each region row is anchored to the start of its line, which is what tells
	// the PRs row from the PRs column of the table's header.
	order := []string{
		"charlie", "dev/bravo · git", "\n" + tabNameBranches, "\n" + tabNamePRs,
		"notes · .doing", "wip",
	}
	at := -1
	for _, want := range order {
		i := strings.Index(body, want)
		if i < 0 {
			t.Fatalf("the body is missing %q:\n%s", want, body)
		}
		if i < at {
			t.Errorf("%q renders above the section before it:\n%s", want, body)
		}
		at = i
	}

	for _, want := range []string{"2 local · feature ↑2", "1 open · #42 Add login flow"} {
		if !strings.Contains(body, want) {
			t.Errorf("the region is missing %q:\n%s", want, body)
		}
	}
}

// TestExpandRegion_ReadsWhileItsDataIsOutstanding covers the other half of the
// pending treatment: a section with nothing yet says it is reading rather than
// claiming the repo has no branches or pull requests.
func TestExpandRegion_ReadsWhileItsDataIsOutstanding(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 30)
	m.expandOpen = true
	m.cursor = 1
	m.startFetch("/dev/bravo", fetchOpenPRs)

	body := plainText(m.renderListBody())

	for _, section := range []string{tabNameBranches, tabNamePRs} {
		line := lineStarting(t, body, section)
		if !strings.Contains(line, readingLabel) {
			t.Errorf("%s reads %q with its fetch still out, want %q", section, line, readingLabel)
		}
	}

	if !strings.Contains(body, "notes"+compactSignalSep+readingLabel) {
		t.Errorf("the notes divider does not say the read is still out:\n%s", body)
	}
}

// lineStarting returns the region row labeled want, which the column header
// cannot be mistaken for because a header row is indented by the cursor gutter.
func lineStarting(t *testing.T, body, want string) string {
	t.Helper()

	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, want) {
			return line
		}
	}

	t.Fatalf("no line starts with %q:\n%s", want, body)

	return ""
}

func TestNotesPreview_ElidesTheMiddleOfALongNote(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, 20)
	for i := range 20 {
		lines = append(lines, string(rune('a'+i)))
	}

	got := plainText(strings.Join(elideMiddle(lines, 5), "\n"))

	if !strings.Contains(got, "a") || !strings.Contains(got, "t") {
		t.Errorf("elision dropped an end; both must survive:\n%s", got)
	}
	if !strings.Contains(got, "16 more lines") {
		t.Errorf("elision does not say how much it cut:\n%s", got)
	}
}

// TestNotesPanel_ShowsTheNoteWithoutFocusingIt covers the focused view's half
// of the emphasis: a note is what the last session left behind, so its text
// has to be readable from whichever panel the view opened on.
func TestNotesPanel_ShowsTheNoteWithoutFocusingIt(t *testing.T) {
	t.Parallel()

	m := focusedModel(180, 45)
	m.focusedPanel = panelBranches
	m.notesFiles = []models.NoteFileContent{
		{Name: ".doing", Content: "# heading\n\nfinish the grid\nthen the notes"},
	}

	panels := m.panelSet(m.sectionWidth())
	notes := panels[panelIndex(panels, panelNotes)]

	if notes.relevance != relevanceUrgent {
		t.Errorf("notes scored %d with a note present, want %d", notes.relevance, relevanceUrgent)
	}

	body := plainText(strings.Join(notes.rows, "\n"))
	for _, want := range []string{"finish the grid"} {
		if !strings.Contains(body, want) {
			t.Errorf("notes panel is missing %q while another panel holds focus:\n%s", want, body)
		}
	}

	if notes.count != len(m.notesFiles) {
		t.Errorf("count is %d, want %d; one row per file",
			notes.count, len(m.notesFiles))
	}
}

func TestNotesPreview_CaptionsTheRegionAtItsDivider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files []models.NoteFileContent
		want  []string
	}{
		{
			name:  "one note is named by the divider alone",
			files: []models.NoteFileContent{{Name: ".doing", Content: "wip"}},
			want:  []string{"notes · .doing"},
		},
		{
			name: "several notes are headed and counted",
			files: []models.NoteFileContent{
				{Name: ".doing", Content: "wip"},
				{Name: "TODO.md", Content: "later"},
			},
			want: []string{"── .doing", "── TODO.md", "notes · 2 files"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := compactModel(180, 40)
			m.expandOpen = true
			m.cursor = 1 // bravo, the repo carrying notes
			m.notesPreview = map[string][]models.NoteFileContent{"/dev/bravo": tt.files}
			lines := m.expandLines(120, m.expandHeight(m.listBodyHeight()))
			got := plainText(strings.Join(lines, "\n"))

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("preview is missing %q:\n%s", want, got)
				}
			}

			if closing := plainText(lines[len(lines)-1]); strings.Trim(closing, "─") != "" {
				t.Errorf("the last line must be a bare closing rule, got %q", closing)
			}
		})
	}
}

// TestEscape_DismissesOneListLayerAtATime pins the order the repo list gives
// back: the most recently opened thing closes first, and an esc with nothing
// open must not walk out of the fleet.
func TestEscape_DismissesOneListLayerAtATime(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 40)
	m.expandOpen = true
	m.searchText = "alpha"
	m.SetFilter(models.FilterModeDirty)
	m.updateFilteredPaths()

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}

	m = stepModel(t, &m, esc)

	if m.expandOpen {
		t.Fatal("the first escape must close the expanded region")
	}

	if m.searchText != "alpha" || !m.filtersActive() {
		t.Fatal("the first escape must leave the search and the filters alone")
	}

	m = stepModel(t, &m, esc)

	if m.searchText != "" {
		t.Errorf("the second escape must clear the search, got %q", m.searchText)
	}

	if !m.filtersActive() {
		t.Fatal("the second escape must leave the filters alone")
	}

	m = stepModel(t, &m, esc)

	if m.filtersActive() {
		t.Error("the third escape must clear the filters")
	}

	m = stepModel(t, &m, esc)

	if m.viewMode != ViewModeRepoList {
		t.Errorf("an escape with nothing open must stay in the list, got %v", m.viewMode)
	}
}

func stepModel(t *testing.T, m *Model, msg tea.Msg) Model {
	t.Helper()

	next, _ := m.Update(msg)

	return mustModel(t, next)
}
