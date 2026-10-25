package app

import "github.com/kyleking/gh-repo-dashboard/internal/tasks"

// TextObject names a scope of repos an operator can act on, vim-style:
// the two-key sequences ar, br, dr, nr, pr, sr.
type TextObject struct {
	Key     string
	Name    string
	Matches func(m Model, path string) bool
}

func textObjects() []TextObject {
	return []TextObject{
		{Key: "ar", Name: nameAll, Matches: func(_ Model, _ string) bool {
			return true
		}},
		{Key: "br", Name: "behind", Matches: func(m Model, path string) bool {
			return m.summaries[path].Behind > 0
		}},
		{Key: "dr", Name: "dirty", Matches: func(m Model, path string) bool {
			return m.summaries[path].IsDirty()
		}},
		{Key: "nr", Name: "with notes", Matches: func(m Model, path string) bool {
			return m.summaries[path].HasNotes()
		}},
		{Key: "pr", Name: "with PRs", Matches: func(m Model, path string) bool {
			return len(m.summaries[path].OpenPRs) > 0
		}},
		{Key: "sr", Name: nameMarked, Matches: func(m Model, path string) bool {
			return m.isMarked(path)
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

// operator is a batch verb: the key that names a built-in task.
type operator struct {
	Key  string
	Task string
}

func operators() []operator {
	return []operator{
		{Key: "C", Task: tasks.NameCleanup},
		{Key: "F", Task: tasks.NameFetch},
		{Key: "P", Task: tasks.NamePrune},
		{Key: "R", Task: tasks.NameRefreshPRs},
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
