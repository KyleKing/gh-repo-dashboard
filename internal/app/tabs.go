package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// Tab is one top-level screen. The two answer different questions (what is the
// state of my repos, and what is waiting on me across them), so each keeps its
// own list, filters, and keys rather than sharing one.
type Tab int

// Tabs in bar order.
const (
	TabRepos Tab = iota
	TabPRs
)

type tabSpec struct {
	tab   Tab
	key   string
	title string
}

// tabBar is the bar as rendered, left to right. The key is capitalized because
// the lowercase letters belong to the panels and the filters underneath.
var tabBar = []tabSpec{
	{tab: TabRepos, key: "R", title: "Repos"},
	{tab: TabPRs, key: "P", title: tabNamePRs},
}

// tabForKey returns the tab a bar key opens.
func tabForKey(key string) (Tab, bool) {
	for _, spec := range tabBar {
		if spec.key == key {
			return spec.tab, true
		}
	}

	return TabRepos, false
}

// currentTab is the tab the view mode sits under. Every drill-down of the repo
// list is still the repos tab, so the bar never contradicts the breadcrumb.
func (m Model) currentTab() Tab {
	if m.viewMode == ViewModePRList {
		return TabPRs
	}

	return TabRepos
}

// renderTabBar draws the bar with the current tab lit and its key bracketed in
// its own name, the way each panel's border carries its jump key.
func (m Model) renderTabBar() string {
	current := m.currentTab()

	labels := make([]string, 0, len(tabBar))
	for _, spec := range tabBar {
		label := " " + markHotkey(spec.title, spec.key) + " "
		if spec.tab == current {
			labels = append(labels, styles.SelectedTabStyle.Render(label))

			continue
		}

		labels = append(labels, styles.TabStyle.Render(label))
	}

	return strings.Join(labels, styles.TabStyle.Render("│"))
}

// openTab moves to tab, reading what it shows if it has nothing yet. Pressing
// the current tab's own key is a no-op rather than a reload, so a key held
// down never re-runs a search.
func (m Model) openTab(tab Tab) (tea.Model, tea.Cmd) {
	if tab == m.currentTab() {
		return m, nil
	}

	if tab == TabPRs {
		next, cmd := m.openPRList()

		return next, cmd
	}

	m.viewMode = ViewModeRepoList

	return m, nil
}
