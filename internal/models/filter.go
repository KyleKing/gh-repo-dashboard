package models

type ActiveFilter struct {
	Mode     FilterMode
	Enabled  bool
	Inverted bool
}

func (f ActiveFilter) DisplayName() string {
	return f.Mode.String()
}

func (f ActiveFilter) ShortKey() string {
	return f.Mode.ShortKey()
}

func NewActiveFilter(mode FilterMode) ActiveFilter {
	return ActiveFilter{
		Mode:     mode,
		Enabled:  mode == FilterModeAll,
		Inverted: false,
	}
}

type SortDirection int

const (
	SortDirectionOff SortDirection = iota
	SortDirectionAsc
	SortDirectionDesc
)

func (d SortDirection) String() string {
	switch d {
	case SortDirectionAsc:
		return "ASC"
	case SortDirectionDesc:
		return "DESC"
	default:
		return ""
	}
}

type ActiveSort struct {
	Mode      SortMode
	Direction SortDirection
	Priority  int
}

func (s ActiveSort) DisplayName() string {
	name := s.Mode.String()
	if s.Direction != SortDirectionOff {
		name += " (" + s.Direction.String() + ")"
	}
	return name
}

func (s ActiveSort) ShortKey() string {
	return s.Mode.ShortKey()
}

func (s ActiveSort) IsEnabled() bool {
	return s.Direction != SortDirectionOff
}

func NewActiveSort(mode SortMode, priority int) ActiveSort {
	return ActiveSort{
		Mode:      mode,
		Direction: SortDirectionOff,
		Priority:  priority,
	}
}
