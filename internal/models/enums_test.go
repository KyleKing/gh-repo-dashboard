package models

import "testing"

func TestVCSTypeString(t *testing.T) {
	tests := []struct {
		vcs      VCSType
		expected string
	}{
		{VCSTypeGit, "git"},
		{VCSTypeJJ, "jj"},
	}

	for _, tt := range tests {
		if tt.vcs.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.vcs.String())
		}
	}
}

func TestFilterModeString(t *testing.T) {
	tests := []struct {
		mode     FilterMode
		expected string
	}{
		{FilterModeAll, "All"},
		{FilterModeAhead, "Ahead"},
		{FilterModeBehind, "Behind"},
		{FilterModeDirty, "Dirty"},
		{FilterModeHasPR, "Has PR"},
		{FilterModeHasStash, "Has Stash"},
	}

	for _, tt := range tests {
		if tt.mode.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.mode.String())
		}
	}
}

func TestFilterModeShortKey(t *testing.T) {
	tests := []struct {
		mode     FilterMode
		expected string
	}{
		{FilterModeAll, "a"},
		{FilterModeAhead, ">"},
		{FilterModeBehind, "<"},
		{FilterModeDirty, "d"},
		{FilterModeHasPR, "p"},
		{FilterModeHasStash, "s"},
	}

	for _, tt := range tests {
		if tt.mode.ShortKey() != tt.expected {
			t.Errorf("FilterMode %v: expected %s, got %s", tt.mode, tt.expected, tt.mode.ShortKey())
		}
	}
}

func TestAllFilterModes(t *testing.T) {
	modes := AllFilterModes()
	if len(modes) != 6 {
		t.Errorf("expected 6 filter modes, got %d", len(modes))
	}
}

func TestSortModeString(t *testing.T) {
	tests := []struct {
		mode     SortMode
		expected string
	}{
		{SortModeName, "Name"},
		{SortModeModified, "Modified"},
		{SortModeStatus, "Status"},
		{SortModeBranch, "Branch"},
	}

	for _, tt := range tests {
		if tt.mode.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.mode.String())
		}
	}
}

func TestSortModeNext(t *testing.T) {
	tests := []struct {
		mode     SortMode
		expected SortMode
	}{
		{SortModeName, SortModeModified},
		{SortModeModified, SortModeStatus},
		{SortModeStatus, SortModeBranch},
		{SortModeBranch, SortModeName},
	}

	for _, tt := range tests {
		if tt.mode.Next() != tt.expected {
			t.Errorf("SortMode %v.Next(): expected %v, got %v", tt.mode, tt.expected, tt.mode.Next())
		}
	}
}

func TestRepoStatusString(t *testing.T) {
	tests := []struct {
		status   RepoStatus
		expected string
	}{
		{RepoStatusClean, "clean"},
		{RepoStatusDirty, "dirty"},
		{RepoStatusAhead, "ahead"},
		{RepoStatusBehind, "behind"},
		{RepoStatusDiverged, "diverged"},
	}

	for _, tt := range tests {
		if tt.status.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.status.String())
		}
	}
}

func TestItemKindString(t *testing.T) {
	tests := []struct {
		kind     ItemKind
		expected string
	}{
		{ItemKindBranch, "branch"},
		{ItemKindStash, "stash"},
		{ItemKindWorktree, "worktree"},
	}

	for _, tt := range tests {
		if tt.kind.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.kind.String())
		}
	}
}
