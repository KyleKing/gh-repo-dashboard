package app

import "path/filepath"

// Snapshot is a serializable projection of the model's observable state,
// used by the fixture harness and suitable for scripting output.
type Snapshot struct {
	View          string   `json:"view"`
	Cursor        int      `json:"cursor"`
	Filtered      []string `json:"filtered"`
	Selected      []string `json:"selected"`
	Predicate     string   `json:"predicate,omitempty"`
	Search        string   `json:"search,omitempty"`
	CommandInput  string   `json:"commandInput,omitempty"`
	BatchTask     string   `json:"batchTask,omitempty"`
	BatchTotal    int      `json:"batchTotal,omitempty"`
	StatusMessage string   `json:"statusMessage,omitempty"`
}

func (v ViewMode) String() string {
	switch v {
	case ViewModeRepoList:
		return "list"
	case ViewModeRepoDetail:
		return "detail"
	case ViewModeBranchDetail:
		return "branch"
	case ViewModePRDetail:
		return "pr"
	case ViewModeHelp:
		return "help"
	case ViewModeFilter:
		return "filter"
	case ViewModeSort:
		return "sort"
	case ViewModeBatchProgress:
		return "batch"
	default:
		return "unknown"
	}
}

// Snapshot captures the observable state; repo paths are reduced to their
// base names for readability.
func (m Model) Snapshot() Snapshot {
	filtered := make([]string, 0, len(m.filteredPaths))
	for _, path := range m.filteredPaths {
		filtered = append(filtered, filepath.Base(path))
	}

	var selected []string
	for _, path := range m.filteredPaths {
		if m.selectedPaths[path] {
			selected = append(selected, filepath.Base(path))
		}
	}

	return Snapshot{
		View:          m.viewMode.String(),
		Cursor:        m.cursor,
		Filtered:      filtered,
		Selected:      selected,
		Predicate:     m.predicateText,
		Search:        m.searchText,
		CommandInput:  m.commandInput.Value(),
		BatchTask:     m.batchTask,
		BatchTotal:    m.batchTotal,
		StatusMessage: m.statusMessage,
	}
}
