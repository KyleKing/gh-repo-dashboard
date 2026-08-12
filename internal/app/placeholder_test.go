//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func placeholderModel(loading bool) Model {
	m := New([]string{"/dev"}, 1)
	m.width = 120
	m.height = 30
	m.selectedRepo = "/dev/alpha"
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {Path: "/dev/alpha", Branch: mainBranchName, VCSType: models.VCSTypeGit},
	}
	m.detailLoading = loading

	return m
}

// An empty panel is worth a box only while its data is still in flight. Once
// the load settles, an empty list is dropped and the Status panel names what is
// missing, so the grid never spends four lines saying "none".
func TestEmptyPanelsSayLoadingThenDisappear(t *testing.T) {
	t.Parallel()

	ids := []panelID{panelBranches, panelStashes, panelPRs, panelPeers, panelNotes}
	for _, id := range ids {
		loading := plainText(renderPanel(placeholderModel(true), id))
		if !strings.Contains(loading, "loading") {
			t.Errorf("panel %v while loading reads %q, want it to say so", id, loading)
		}
	}

	settled := placeholderModel(false)
	shown := map[panelID]bool{}
	for _, p := range settled.panelSet(contentWidth(settled.width)) {
		shown[p.id] = true
	}

	for _, id := range []panelID{panelStashes, panelPRs, panelPeers, panelNotes} {
		if shown[id] {
			t.Errorf("panel %v is still drawn once settled with nothing to list", id)
		}
	}
	for _, id := range []panelID{panelStatus, panelBranches} {
		if !shown[id] {
			t.Errorf("panel %v must stay on screen even when empty", id)
		}
	}

	status := plainText(renderPanel(settled, panelStatus))
	if !strings.Contains(status, "no PRs, peers, stashes, or notes") {
		t.Errorf("Status must name the dropped panels, got %q", status)
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
