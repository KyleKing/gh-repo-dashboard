package app

import "github.com/kyleking/aragonite/tui/frame"

// Tab is one top-level screen.
type Tab int

// Tabs in bar order.
const (
	TabRepos Tab = iota
)

type tabSpec struct {
	tab   Tab
	title string
}

var tabBar = []tabSpec{
	{tab: TabRepos, title: "Repos"},
}

// currentTab is the tab the view mode sits under. Every drill-down of the repo
// list is still the repos tab, so the bar never contradicts the breadcrumb.
func (m Model) currentTab() Tab {
	return TabRepos
}

func (m Model) frameTabs() []frame.Tab {
	current := m.currentTab()

	tabs := make([]frame.Tab, 0, len(tabBar))
	for _, spec := range tabBar {
		tabs = append(tabs, frame.Tab{Label: spec.title, Active: spec.tab == current})
	}

	return tabs
}
