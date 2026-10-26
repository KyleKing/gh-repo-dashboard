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

// TestDetailRegion_KeepsTheFrameHeightFixed pins the bug the region used to
// have: its lines were added to a body that had already been sized, so the
// footer slid off the bottom whenever the whole repo set fitted on screen. Only
// one repo of the fleet carries notes, branches, and pull requests, so a frame
// that sized itself from the selection would fail here.
func TestDetailRegion_KeepsTheFrameHeightFixed(t *testing.T) {
	t.Parallel()

	sizes := []struct {
		name          string
		width, height int
	}{
		{name: "compact", width: 80, height: 24},
		{name: "below", width: 120, height: 40},
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
				m.detailOpen = open

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

// TestDetailRegion_SitsBelowTheTable pins the shape: the table leads, the region
// follows it, and the region's own sections keep a fixed order.
func TestDetailRegion_SitsBelowTheTable(t *testing.T) {
	t.Parallel()

	m := compactModel(120, 40)
	m.detailOpen = true
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

	body := plainText(strings.Join(m.renderDetail(100, 38), "\n"))

	// Each row is anchored to the start of its line, which is what tells a
	// section's label from the same word inside another row.
	order := []string{
		"dev/bravo", "── NEEDS", "\n" + tabNameBranches, "── PULL REQUESTS", "── NOTES", "wip",
	}
	at := -1
	for _, want := range order {
		i := strings.Index(body, want)
		if i < 0 {
			t.Fatalf("the detail is missing %q:\n%s", want, body)
		}
		if i < at {
			t.Errorf("%q renders above the section before it:\n%s", want, body)
		}
		at = i
	}

	for _, want := range []string{"2 local", "feature ↑2", "#42"} {
		if !strings.Contains(body, want) {
			t.Errorf("the detail is missing %q:\n%s", want, body)
		}
	}
}

// TestDetailRegion_ReadsWhileItsDataIsOutstanding covers the other half of the
// pending treatment: a section with nothing yet says it is reading rather than
// claiming the repo has no branches or pull requests.
func TestDetailRegion_ReadsWhileItsDataIsOutstanding(t *testing.T) {
	t.Parallel()

	m := compactModel(120, 40)
	m.detailOpen = true
	m.cursor = 1
	m.startFetch("/dev/bravo", fetchOpenPRs)

	body := plainText(strings.Join(m.renderDetail(100, 38), "\n"))

	for _, section := range []string{tabNameBranches, "── PULL REQUESTS", "── NOTES"} {
		if !strings.Contains(lineStarting(t, body, section)+nextLine(body, section), readingLabel) {
			t.Errorf("%s does not say it is reading with its fetch still out:\n%s", section, body)
		}
	}

	m.cursor = 0

	if quiet := plainText(strings.Join(m.renderDetail(100, 38), "\n")); !strings.Contains(quiet, "none") {
		t.Errorf("a repo with no pull requests must say none, not reading:\n%s", quiet)
	}
}

func nextLine(body, prefix string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) && i+1 < len(lines) {
			return lines[i+1]
		}
	}

	return ""
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

func TestNotesPreview_HeadsEachNoteOfSeveral(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files []models.NoteFileContent
		want  []string
	}{
		{
			name:  "one note needs no heading of its own",
			files: []models.NoteFileContent{{Name: ".doing", Content: "wip"}},
			want:  []string{"NOTES · 1", "wip"},
		},
		{
			name: "several notes are headed and counted",
			files: []models.NoteFileContent{
				{Name: ".doing", Content: "wip"},
				{Name: "TODO.md", Content: "later"},
			},
			want: []string{"NOTES · 2", "── .doing", "── TODO.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := compactModel(120, 40)
			m.detailOpen = true
			m.cursor = 1 // bravo, the repo carrying notes
			m.notesPreview = map[string][]models.NoteFileContent{"/dev/bravo": tt.files}
			got := plainText(strings.Join(m.renderDetail(100, 38), "\n"))

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("detail is missing %q:\n%s", want, got)
				}
			}
		})
	}
}

// TestEscape_DismissesOneListLayerAtATime pins the order the repo list gives
// back: the most recently opened thing closes first, and an esc with nothing
// open must not walk out of the fleet.
func TestEscape_DismissesOneListLayerAtATime(t *testing.T) {
	t.Parallel()

	m := compactModel(120, 40)
	m.detailOpen = true
	m.searchText = "alpha"
	m.SetFilter(models.FilterModeDirty)
	m.updateFilteredPaths()

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}

	m = stepModel(t, &m, esc)

	if m.detailOpen {
		t.Fatal("the first escape must close the detail region")
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
