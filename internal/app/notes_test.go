//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// TestNotesPreview_KeepsTheFrameHeightFixed pins the bug the preview used to
// have: its lines were added to a body that had already been sized, so the
// footer slid off the bottom whenever the whole repo set fitted on screen.
func TestNotesPreview_KeepsTheFrameHeightFixed(t *testing.T) {
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

			for _, open := range []bool{false, true} {
				m.notesPreviewOpen = open

				for cursor := range m.filteredPaths {
					m.cursor = cursor

					got := strings.Count(m.renderRepoList(), "\n") + 1
					if got != size.height {
						t.Errorf("preview open=%v, cursor=%d: rendered %d lines, want %d",
							open, cursor, got, size.height)
					}
				}
			}
		})
	}
}

// TestNotesPreview_HoldsTheListWindowSteady covers the second half of the same
// complaint: the window must not resize as the cursor crosses between a repo
// with notes and one without, or the rows jump under the cursor.
func TestNotesPreview_HoldsTheListWindowSteady(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 30)
	m.notesPreviewOpen = true

	m.cursor = 0
	withoutNotes := m.visibleRepoRange(1)

	m.cursor = 1
	withNotes := m.visibleRepoRange(1)

	if got, want := withNotes.end-withNotes.start, withoutNotes.end-withoutNotes.start; got != want {
		t.Errorf("window holds %d rows on a repo with notes and %d on one without", got, want)
	}
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

	panels := m.panelSet(panelSideWidth(m.gridWidth()) - panelBorderWidth)
	notes := panels[panelIndex(panels, panelNotes)]

	if notes.relevance != relevanceUrgent {
		t.Errorf("notes scored %d with a note present, want %d", notes.relevance, relevanceUrgent)
	}

	body := plainText(strings.Join(notes.rows, "\n"))
	for _, want := range []string{"finish the grid", "then the notes"} {
		if !strings.Contains(body, want) {
			t.Errorf("notes panel is missing %q while another panel holds focus:\n%s", want, body)
		}
	}

	if notes.count != len(m.notesFiles) {
		t.Errorf("count is %d, want %d; the body rows must not be counted as files",
			notes.count, len(m.notesFiles))
	}
}
