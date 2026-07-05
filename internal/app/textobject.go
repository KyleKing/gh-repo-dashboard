package app

import (
	tea "charm.land/bubbletea/v2"
)

// TextObject names a scope of repos an operator can act on, vim-style:
// the two-key sequences ar, br, dr, nr, pr, sr.
type TextObject struct {
	Key     string
	Name    string
	Matches func(m Model, path string) bool
}

func textObjects() []TextObject {
	return []TextObject{
		{Key: "ar", Name: "all", Matches: func(_ Model, _ string) bool {
			return true
		}},
		{Key: "br", Name: "behind", Matches: func(m Model, path string) bool {
			return m.summaries[path].Behind > 0
		}},
		{Key: "dr", Name: "dirty", Matches: func(m Model, path string) bool {
			return m.summaries[path].IsDirty()
		}},
		{Key: "nr", Name: "with notes", Matches: func(m Model, path string) bool {
			return m.summaries[path].NotesFile != ""
		}},
		{Key: "pr", Name: "with PRs", Matches: func(m Model, path string) bool {
			return m.summaries[path].PRInfo != nil
		}},
		{Key: "sr", Name: "selected", Matches: func(m Model, path string) bool {
			return m.selectedPaths[path]
		}},
	}
}

func lookupTextObject(key string) (TextObject, bool) {
	for _, obj := range textObjects() {
		if obj.Key == key {
			return obj, true
		}
	}

	return TextObject{}, false
}

// resolveTextObject returns the visible repo paths the object covers.
func (m Model) resolveTextObject(obj TextObject) []string {
	var paths []string
	for _, path := range m.filteredPaths {
		if obj.Matches(m, path) {
			paths = append(paths, path)
		}
	}

	return paths
}

type operator struct {
	Key      string
	TaskName string
	Cmd      func([]string) tea.Cmd
}

func operators() []operator {
	return []operator{
		{Key: "C", TaskName: "Cleanup Merged", Cmd: batchCleanupMergedCmd},
		{Key: "F", TaskName: "Fetch All", Cmd: batchFetchAllCmd},
		{Key: "P", TaskName: "Prune Remote", Cmd: batchPruneRemoteCmd},
	}
}

func lookupOperator(key string) (operator, bool) {
	for _, op := range operators() {
		if op.Key == key {
			return op, true
		}
	}

	return operator{}, false
}
