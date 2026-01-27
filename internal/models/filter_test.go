package models

import "testing"

func TestActiveFilterNewActiveFilter(t *testing.T) {
	tests := []struct {
		mode        FilterMode
		wantEnabled bool
	}{
		{FilterModeAll, true},
		{FilterModeAhead, false},
		{FilterModeBehind, false},
		{FilterModeDirty, false},
		{FilterModeHasPR, false},
		{FilterModeHasStash, false},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			f := NewActiveFilter(tt.mode)
			if f.Enabled != tt.wantEnabled {
				t.Errorf("mode %s: expected enabled=%v, got %v", tt.mode, tt.wantEnabled, f.Enabled)
			}
			if f.Inverted {
				t.Error("new filter should not be inverted")
			}
			if f.Mode != tt.mode {
				t.Errorf("expected mode=%v, got %v", tt.mode, f.Mode)
			}
		})
	}
}

func TestActiveFilterDisplayName(t *testing.T) {
	f := NewActiveFilter(FilterModeAhead)
	if f.DisplayName() != "Ahead" {
		t.Errorf("expected 'Ahead', got %q", f.DisplayName())
	}
}

func TestActiveFilterShortKey(t *testing.T) {
	f := NewActiveFilter(FilterModeAhead)
	if f.ShortKey() != ">" {
		t.Errorf("expected '>', got %q", f.ShortKey())
	}
}

func TestSortDirectionString(t *testing.T) {
	tests := []struct {
		dir      SortDirection
		expected string
	}{
		{SortDirectionOff, ""},
		{SortDirectionAsc, "ASC"},
		{SortDirectionDesc, "DESC"},
	}

	for _, tt := range tests {
		result := tt.dir.String()
		if result != tt.expected {
			t.Errorf("SortDirection %d: expected %q, got %q", tt.dir, tt.expected, result)
		}
	}
}

func TestActiveSortNewActiveSort(t *testing.T) {
	s := NewActiveSort(SortModeName, 0)
	if s.Mode != SortModeName {
		t.Errorf("expected SortModeName, got %v", s.Mode)
	}
	if s.Direction != SortDirectionOff {
		t.Error("new sort should have direction Off")
	}
	if s.Priority != 0 {
		t.Errorf("expected priority 0, got %d", s.Priority)
	}
}

func TestActiveSortIsEnabled(t *testing.T) {
	tests := []struct {
		dir      SortDirection
		expected bool
	}{
		{SortDirectionOff, false},
		{SortDirectionAsc, true},
		{SortDirectionDesc, true},
	}

	for _, tt := range tests {
		s := ActiveSort{Direction: tt.dir}
		if s.IsEnabled() != tt.expected {
			t.Errorf("direction %v: expected IsEnabled()=%v, got %v", tt.dir, tt.expected, s.IsEnabled())
		}
	}
}

func TestActiveSortDisplayName(t *testing.T) {
	tests := []struct {
		sort     ActiveSort
		expected string
	}{
		{
			sort:     ActiveSort{Mode: SortModeName, Direction: SortDirectionOff},
			expected: "Name",
		},
		{
			sort:     ActiveSort{Mode: SortModeName, Direction: SortDirectionAsc},
			expected: "Name (ASC)",
		},
		{
			sort:     ActiveSort{Mode: SortModeModified, Direction: SortDirectionDesc},
			expected: "Modified (DESC)",
		},
	}

	for _, tt := range tests {
		result := tt.sort.DisplayName()
		if result != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, result)
		}
	}
}

func TestActiveSortShortKey(t *testing.T) {
	s := ActiveSort{Mode: SortModeName}
	if s.ShortKey() != "n" {
		t.Errorf("expected 'n', got %q", s.ShortKey())
	}
}
