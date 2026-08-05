package app

import (
	"path/filepath"
	"strings"
)

// Snapshot is a serializable projection of the model's observable state,
// used by the fixture harness and suitable for scripting output.
type Snapshot struct {
	View          string   `json:"view"`
	Cursor        int      `json:"cursor"`
	Filtered      []string `json:"filtered"`
	Selected      []string `json:"selected"`
	Predicate     string   `json:"predicate,omitempty"`
	Search        string   `json:"search,omitempty"`
	CommandInput  string   `json:"command_input,omitempty"`
	BatchTask     string   `json:"batch_task,omitempty"`
	BatchTotal    int      `json:"batch_total,omitempty"`
	StatusMessage string   `json:"status_message,omitempty"`
	Panel         string   `json:"panel,omitempty"`
	Find          string   `json:"find,omitempty"`
	FindMatches   []string `json:"find_matches,omitempty"`
	Overview      []string `json:"overview,omitempty"`
}

func (v ViewMode) String() string {
	switch v {
	case ViewModeRepoList:
		return "list"
	case ViewModeRepoDetail:
		return "detail"
	case ViewModeBranchDetail:
		return nameBranch
	case ViewModePRDetail:
		return "pr"
	case ViewModeHelp:
		return nameHelp
	case ViewModeFilter:
		return nameFilter
	case ViewModeSort:
		return nameSort
	case ViewModePRMap:
		return "prmap"
	case ViewModePalette:
		return nameFind
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
		Panel:         m.focusedPanelName(),
		Find:          m.paletteInput.Value(),
		FindMatches:   m.findMatchLabels(),
		Overview:      m.overviewSummary(),
	}
}

// findMatchLabels lists what the open palette currently matches, so a fixture
// can assert the find without simulating its rendering.
func (m Model) findMatchLabels() []string {
	if m.viewMode != ViewModePalette {
		return nil
	}

	results := m.findResults()

	labels := make([]string, 0, len(results))
	for i := range results {
		labels = append(labels, results[i].kindName()+":"+results[i].label)
	}

	return labels
}

// focusedPanelName names the panel holding the cursor in the focused view, or
// is empty outside that view.
func (m Model) focusedPanelName() string {
	if m.viewMode != ViewModeRepoDetail {
		return ""
	}

	panels := m.panelSet(contentWidth(m.width))
	for _, p := range panels {
		if p.id == m.focusedPanel {
			return strings.ToLower(p.title)
		}
	}

	return ""
}

// overviewSummary renders the focused view's overview rows as "label=value"
// pairs, so a fixture can assert what the pane answers on arrival.
func (m Model) overviewSummary() []string {
	if m.viewMode != ViewModeRepoDetail {
		return nil
	}

	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return nil
	}

	rows := m.overviewRows(summary, m.isCompact())
	pairs := make([]string, 0, len(rows))
	for _, row := range rows {
		pairs = append(pairs, row.label+"="+row.value)
	}

	return pairs
}
