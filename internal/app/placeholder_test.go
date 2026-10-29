//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/preview"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func placeholderModel(loading bool) Model {
	m := New([]string{"/dev"}, 1)
	m.width = 120
	m.height = 30
	m.selectedRepo = "/dev/alpha"
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {RepoSummary: vcs.RepoSummary{Path: "/dev/alpha", Branch: mainBranchName, VCSType: vcs.TypeGit}},
	}
	m.detailLoading = loading

	return m
}

// An empty section collapses to its rule, and says so with an ellipsis for a
// count while its data is still in flight.
func TestEmptySectionsCollapseToTheirRule(t *testing.T) {
	t.Parallel()

	loading := placeholderModel(true)
	rendered := plainText(strings.Join(loading.renderSections(
		loading.panelSet(loading.width), -1, loading.width, loading.bodyHeight(),
	), "\n"))

	if !strings.Contains(rendered, "BRANCHES · …") {
		t.Errorf("a loading section should show an ellipsis count:\n%s", rendered)
	}

	settled := placeholderModel(false)
	panels := settled.panelSet(settled.width)

	if len(panels) != len(panelOrder)-1 {
		t.Fatalf("got %d sections, want every section but the repo's own", len(panels))
	}

	for i, h := range preview.Heights(sectionsOf(panels), -1, settled.bodyHeight()) {
		if len(panels[i].rows) == 0 && h != 1 {
			t.Errorf("empty section %v is %d lines, want just its rule", panels[i].id, h)
		}
	}
}

func TestNotesTab_ContentWrapsToContentWidth(t *testing.T) {
	t.Parallel()

	m := placeholderModel(false)
	m.notesFiles = []models.NoteFileContent{
		{Name: "doing.txt", Content: strings.Repeat("word ", 40)},
	}

	got := renderPanel(m, panelNotes)

	width := contentWidth(m.width)
	for _, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Errorf("line %q is %d cells wide, want at most %d", line, w, width)
		}
	}
}

func TestRepoList_DiscoveringIsDistinctFromNoRepos(t *testing.T) {
	t.Parallel()

	m := New([]string{"/dev"}, 1)
	m.width = 120
	m.height = 30

	m.loading = true
	if got := m.renderTable(); !strings.Contains(got, "Discovering repositories...") {
		t.Errorf("while discovering, got %q", got)
	}

	m.loading = false
	if got := m.renderTable(); !strings.Contains(got, "No repositories found") {
		t.Errorf("once settled with no repos, got %q", got)
	}
}

func TestRepoList_FilteredToNothingSaysSo(t *testing.T) {
	t.Parallel()

	m := New([]string{"/dev"}, 1)
	m.width = 120
	m.height = 30
	m.loading = false
	m.repoPaths = []string{"/dev/alpha"}
	m.filteredPaths = nil

	got := m.renderTable()
	if !strings.Contains(got, "No repositories match the active filters") {
		t.Errorf("with repos present but filtered out, got %q", got)
	}
}
