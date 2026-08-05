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

func TestDetailTabs_LoadingStateIsDistinctFromEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		render      func(Model) string
		wantLoading string
		wantEmpty   string
	}{
		{
			name:        "branches",
			render:      Model.renderBranchList,
			wantLoading: "Loading branches...",
			wantEmpty:   "No branches found",
		},
		{
			name:        "stashes",
			render:      Model.renderStashList,
			wantLoading: "Loading stashes...",
			wantEmpty:   "No stashes",
		},
		{
			name:        "pull requests",
			render:      Model.renderPRList,
			wantLoading: "Loading pull requests...",
			wantEmpty:   "No open pull requests",
		},
		{
			name:        "worktrees",
			render:      Model.renderWorktreeList,
			wantLoading: "Loading checkouts...",
			wantEmpty:   "This is the only checkout of the repo",
		},
		{
			name:        "notes",
			render:      Model.renderNotesTab,
			wantLoading: "Loading notes...",
			wantEmpty:   "No notes file found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loading := tt.render(placeholderModel(true))
			if !strings.Contains(loading, tt.wantLoading) {
				t.Errorf("while loading, got %q, want it to contain %q", loading, tt.wantLoading)
			}
			if strings.Contains(loading, tt.wantEmpty) {
				t.Errorf("while loading, %q must not claim %q", loading, tt.wantEmpty)
			}

			settled := tt.render(placeholderModel(false))
			if !strings.Contains(settled, tt.wantEmpty) {
				t.Errorf("once settled, got %q, want it to contain %q", settled, tt.wantEmpty)
			}
		})
	}
}

func TestNotesTab_ContentWrapsToContentWidth(t *testing.T) {
	t.Parallel()

	m := placeholderModel(false)
	m.notesFiles = []models.NoteFileContent{
		{Name: "doing.txt", Content: strings.Repeat("word ", 40)},
	}

	got := m.renderNotesTab()

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
