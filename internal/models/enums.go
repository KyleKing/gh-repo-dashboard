package models

type VCSType int

const (
	VCSTypeGit VCSType = iota
	VCSTypeJJ
)

func (v VCSType) String() string {
	switch v {
	case VCSTypeGit:
		return "git"
	case VCSTypeJJ:
		return "jj"
	default:
		return "unknown"
	}
}

type FilterMode int

const (
	FilterModeAll FilterMode = iota
	FilterModeAhead
	FilterModeBehind
	FilterModeDirty
	FilterModeHasPR
	FilterModeHasStash
)

func (f FilterMode) String() string {
	switch f {
	case FilterModeAll:
		return "All"
	case FilterModeAhead:
		return "Ahead"
	case FilterModeBehind:
		return "Behind"
	case FilterModeDirty:
		return "Dirty"
	case FilterModeHasPR:
		return "Has PR"
	case FilterModeHasStash:
		return "Has Stash"
	default:
		return "Unknown"
	}
}

func (f FilterMode) ShortKey() string {
	switch f {
	case FilterModeAll:
		return "a"
	case FilterModeAhead:
		return ">"
	case FilterModeBehind:
		return "<"
	case FilterModeDirty:
		return "d"
	case FilterModeHasPR:
		return "p"
	case FilterModeHasStash:
		return "s"
	default:
		return "?"
	}
}

func AllFilterModes() []FilterMode {
	return []FilterMode{
		FilterModeAll,
		FilterModeAhead,
		FilterModeBehind,
		FilterModeDirty,
		FilterModeHasPR,
		FilterModeHasStash,
	}
}

func SelectableFilterModes() []FilterMode {
	return []FilterMode{
		FilterModeDirty,
		FilterModeAhead,
		FilterModeBehind,
		FilterModeHasPR,
		FilterModeHasStash,
	}
}

type SortMode int

const (
	SortModeName SortMode = iota
	SortModeModified
	SortModeStatus
	SortModeBranch
)

func (s SortMode) String() string {
	switch s {
	case SortModeName:
		return "Name"
	case SortModeModified:
		return "Modified"
	case SortModeStatus:
		return "Status"
	case SortModeBranch:
		return "Branch"
	default:
		return "Unknown"
	}
}

func (s SortMode) ShortKey() string {
	switch s {
	case SortModeName:
		return "n"
	case SortModeModified:
		return "m"
	case SortModeStatus:
		return "s"
	case SortModeBranch:
		return "b"
	default:
		return "?"
	}
}

func (s SortMode) Next() SortMode {
	return SortMode((int(s) + 1) % 4)
}

func AllSortModes() []SortMode {
	return []SortMode{
		SortModeName,
		SortModeModified,
		SortModeStatus,
		SortModeBranch,
	}
}

type RepoStatus int

const (
	RepoStatusClean RepoStatus = iota
	RepoStatusDirty
	RepoStatusAhead
	RepoStatusBehind
	RepoStatusDiverged
)

func (r RepoStatus) String() string {
	switch r {
	case RepoStatusClean:
		return "clean"
	case RepoStatusDirty:
		return "dirty"
	case RepoStatusAhead:
		return "ahead"
	case RepoStatusBehind:
		return "behind"
	case RepoStatusDiverged:
		return "diverged"
	default:
		return "unknown"
	}
}

type ItemKind int

const (
	ItemKindBranch ItemKind = iota
	ItemKindStash
	ItemKindWorktree
)

func (i ItemKind) String() string {
	switch i {
	case ItemKindBranch:
		return "branch"
	case ItemKindStash:
		return "stash"
	case ItemKindWorktree:
		return "worktree"
	default:
		return "unknown"
	}
}
