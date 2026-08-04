package app

import (
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// scene is a named question the focused repo view can be pointed at. Scenes
// are curated arrangements of the tabs that already exist, so the active
// scene is derived from the current tab rather than stored: there is no way
// for the footer's claim and the rendered table to disagree.
//
// Only the focused repo view binds scene keys today. Other views can adopt
// the same pattern by defining their own scene list.
type scene struct {
	key  string
	name string
	tab  DetailTab
}

// scenes lists the focused view's scenes in key order.
func scenes() []scene {
	return []scene{
		{key: "1", name: "work", tab: DetailTabBranches},
		{key: "2", name: "review", tab: DetailTabPRs},
		{key: "3", name: "sync", tab: DetailTabWorktrees},
		{key: "4", name: "maintain", tab: DetailTabStashes},
	}
}

// sceneForKey returns the scene a number key selects.
func sceneForKey(key string) (scene, bool) {
	for _, s := range scenes() {
		if s.key == key {
			return s, true
		}
	}

	return scene{}, false
}

// renderSceneBar names every scene and marks the active one, so which scene is
// showing is never ambiguous. The Notes tab belongs to no scene and simply
// leaves all four unmarked.
func renderSceneBar(active DetailTab) string {
	parts := make([]string, 0, len(scenes()))
	for _, s := range scenes() {
		label := s.key + " " + s.name
		if s.tab == active {
			parts = append(parts, styles.FooterKeyStyle.Render("["+label+"]"))
			continue
		}

		parts = append(parts, styles.FooterDescStyle.Render(" "+label+" "))
	}

	return strings.Join(parts, "")
}
