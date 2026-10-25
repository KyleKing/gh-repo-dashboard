package app

import "github.com/kyleking/aragonite/tui/frame"

// Tab is one top-level screen.
type Tab int

// Tabs in bar order.
const (
	TabRepos Tab = iota
	TabTasks
)

// The keys that jump straight to a tab, named by the bracketed letter in its title.
const (
	tabKeyRepos = "R"
	tabKeyTasks = "T"
)

type tabSpec struct {
	tab   Tab
	title string
}

var tabBar = []tabSpec{
	{tab: TabRepos, title: "[R]epos"},
	{tab: TabTasks, title: "[T]asks"},
}

// currentTab is the tab the view mode sits under. Every drill-down of the repo
// list is still the repos tab, so the bar never contradicts the breadcrumb.
func (m Model) currentTab() Tab {
	switch m.underlyingMode() {
	case ViewModeTasks, ViewModeTaskPreview, ViewModeTaskRun, ViewModeTaskLog:
		return TabTasks
	default:
		return TabRepos
	}
}

// underlyingMode is the view an overlay (help, find, a confirmation) was
// opened over, and the view mode itself otherwise.
func (m Model) underlyingMode() ViewMode {
	mode := m.viewMode
	if mode == ViewModeHelp {
		mode = m.helpFrom
	}

	switch mode { //nolint:exhaustive // every other mode is its own base
	case ViewModePalette:
		return m.paletteReturnMode
	case ViewModeConfirm:
		if m.pendingAction != nil {
			return m.pendingAction.returnMode
		}
	}

	return mode
}

func (m Model) frameTabs() []frame.Tab {
	current := m.currentTab()

	tabs := make([]frame.Tab, 0, len(tabBar))
	for _, spec := range tabBar {
		tabs = append(tabs, frame.Tab{Label: spec.title, Active: spec.tab == current})
	}

	return tabs
}
