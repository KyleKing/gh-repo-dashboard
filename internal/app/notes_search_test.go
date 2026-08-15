//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// TestSearchNotesScopeMatchesLoadedContentOnly confirms "n:" searches note
// bodies rather than repo name/branch, and that a repo whose notes haven't
// loaded yet simply doesn't match instead of erroring.
func TestSearchNotesScopeMatchesLoadedContentOnly(t *testing.T) {
	t.Parallel()

	m := New([]string{"/test"}, 1)
	m.loading = false
	m.repoPaths = []string{"/test/alpha", "/test/beta", "/test/gamma"}
	m.summaries = map[string]models.RepoSummary{
		"/test/alpha": {Path: "/test/alpha", Branch: mainBranchName, NotesFiles: []models.NoteFile{{Name: "doing.md"}}},
		"/test/beta":  {Path: "/test/beta", Branch: mainBranchName, NotesFiles: []models.NoteFile{{Name: "doing.md"}}},
		"/test/gamma": {Path: "/test/gamma", Branch: mainBranchName},
	}
	m.notesPreview = map[string][]models.NoteFileContent{
		"/test/alpha": {{Name: "doing.md", Content: "!! FIRST: fix the schema migration"}},
		// beta has NotesFiles but hasn't loaded its content preview yet.
	}

	m.searchText = "n:schema"
	m.updateFilteredPaths()

	if len(m.filteredPaths) != 1 || m.filteredPaths[0] != "/test/alpha" {
		t.Errorf("expected only alpha (loaded, matching content), got %v", m.filteredPaths)
	}
}
