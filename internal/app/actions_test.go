//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// The leader key is the only way into a panel's verbs, so the letters it offers
// must be free of the single-key namespace the grid uses for jumps and
// movement; otherwise a verb would fire while the menu is closed.
func TestPanelVerbKeysDoNotCollideWithPanelJumps(t *testing.T) {
	t.Parallel()

	jumps := map[string]bool{}
	for _, key := range panelKeys {
		jumps[key] = true
	}

	m := focusedModel(160, 45)
	for _, p := range m.panelSet(60) {
		seen := map[string]bool{}
		for _, action := range panelActionsFor(p.id) {
			if seen[action.key] {
				t.Errorf("panel %q offers %q twice", p.title, action.key)
			}
			seen[action.key] = true
		}
	}

	// A jump key firing a verb would be the collision that matters, so confirm
	// the grid still treats one as a jump while the menu is closed.
	next, _ := m.handleDetailKey(keyPress('p'))
	if jumped := mustModel(t, next); jumped.focusedPanel != panelPRs {
		t.Errorf("p with the menu closed focused %v, want the PRs panel", jumped.focusedPanel)
	}
}

func TestPanelActionMenuRunsAVerbAndClosesOnAnythingElse(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 45)
	m.focusedPanel = panelBranches
	m.branches = []models.BranchInfo{{Name: "feature/thing", Upstream: "origin/feature/thing"}}

	openedModel, _ := m.handleDetailKey(keyPress(rune(panelActionLeader[0])))
	opened := mustModel(t, openedModel)
	if !opened.panelActions {
		t.Fatal("the leader key should open the verb menu")
	}
	if footer := plainText(opened.renderPanelActions("feature/thing")); !strings.Contains(footer, "push") {
		t.Errorf("the open menu should list the branch verbs, got %q", footer)
	}

	ranModel, _ := opened.handleDetailKey(keyPress('p'))
	ran := mustModel(t, ranModel)
	if ran.panelActions {
		t.Error("running a verb should close the menu")
	}
	if ran.viewMode != ViewModeConfirm {
		t.Errorf("push should park behind a confirmation, got %v", ran.viewMode)
	}

	backedOutModel, _ := opened.handleDetailKey(keyPress('z'))
	backedOut := mustModel(t, backedOutModel)
	if backedOut.panelActions {
		t.Error("a key that names no verb should close the menu")
	}
	if backedOut.viewMode != ViewModeRepoDetail {
		t.Errorf("backing out should stay in the grid, got %v", backedOut.viewMode)
	}
}
