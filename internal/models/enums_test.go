package models_test

import (
	"testing"

	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestVCSTypeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		vcs      vcs.Type
		expected string
	}{
		{vcs.TypeGit, "git"},
		{vcs.TypeJJ, "jj"},
	}

	for _, tt := range tests {
		if tt.vcs.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.vcs.String())
		}
	}
}

func TestFilterModeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode     models.FilterMode
		expected string
	}{
		{models.FilterModeAll, "All"},
		{models.FilterModeAhead, "Ahead"},
		{models.FilterModeBehind, "Behind"},
		{models.FilterModeDirty, "Dirty"},
		{models.FilterModeHasPR, "Has PR"},
		{models.FilterModeHasStash, "Has Stash"},
		{models.FilterModeHasNotes, "Has Notes"},
		{models.FilterModeGit, "Git"},
	}

	for _, tt := range tests {
		if tt.mode.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.mode.String())
		}
	}
}

func TestFilterModeShortKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode     models.FilterMode
		expected string
	}{
		{models.FilterModeAll, "a"},
		{models.FilterModeAhead, ">"},
		{models.FilterModeBehind, "<"},
		{models.FilterModeDirty, "d"},
		{models.FilterModeHasPR, "p"},
		{models.FilterModeHasStash, "s"},
		{models.FilterModeHasNotes, "n"},
		{models.FilterModeGit, "g"},
	}

	for _, tt := range tests {
		if tt.mode.ShortKey() != tt.expected {
			t.Errorf("FilterMode %v: expected %s, got %s", tt.mode, tt.expected, tt.mode.ShortKey())
		}
	}
}

func TestAllFilterModes(t *testing.T) {
	t.Parallel()
	modes := models.AllFilterModes()
	if len(modes) != 8 {
		t.Errorf("expected 8 filter modes, got %d", len(modes))
	}
}

func TestSortModeString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode     models.SortMode
		expected string
	}{
		{models.SortModeName, "Name"},
		{models.SortModeModified, "Modified"},
		{models.SortModeStatus, "Status"},
		{models.SortModeBranch, "Branch"},
	}

	for _, tt := range tests {
		if tt.mode.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.mode.String())
		}
	}
}

func TestSortModeNext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode     models.SortMode
		expected models.SortMode
	}{
		{models.SortModeName, models.SortModeModified},
		{models.SortModeModified, models.SortModeStatus},
		{models.SortModeStatus, models.SortModeBranch},
		{models.SortModeBranch, models.SortModeName},
	}

	for _, tt := range tests {
		if tt.mode.Next() != tt.expected {
			t.Errorf("SortMode %v.Next(): expected %v, got %v", tt.mode, tt.expected, tt.mode.Next())
		}
	}
}

func TestRepoStatusString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   vcs.RepoStatus
		expected string
	}{
		{vcs.RepoStatusClean, "clean"},
		{vcs.RepoStatusDirty, "dirty"},
		{vcs.RepoStatusAhead, "ahead"},
		{vcs.RepoStatusBehind, "behind"},
		{vcs.RepoStatusDiverged, "diverged"},
	}

	for _, tt := range tests {
		if tt.status.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.status.String())
		}
	}
}

func TestItemKindString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind     models.ItemKind
		expected string
	}{
		{models.ItemKindBranch, "branch"},
		{models.ItemKindStash, "stash"},
		{models.ItemKindWorktree, "worktree"},
	}

	for _, tt := range tests {
		if tt.kind.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.kind.String())
		}
	}
}
