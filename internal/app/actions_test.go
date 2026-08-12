//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// detailModel builds a repo-detail model sitting on the branches tab with two
// branches: current "main" and "feature" with an open PR.
func detailModel() Model {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.focusedPanel = panelBranches
	m.selectedRepo = testRepo1Path
	m.summaries[testRepo1Path] = models.RepoSummary{Path: testRepo1Path, Branch: mainBranchName}
	m.branches = []models.BranchInfo{
		{Name: mainBranchName, Upstream: "origin/main", IsCurrent: true},
		{Name: featureBranchName},
	}
	m.prs = []models.PRInfo{{Number: 42, Title: "Add login", HeadRef: featureBranchName}}

	return m
}

func TestPushBranchAsksForConfirmation(t *testing.T) {
	t.Parallel()

	m := detailModel()
	m.detailCursor = 1

	updated, cmd := m.startPushBranch()
	m = mustModel(t, updated)

	if m.viewMode != ViewModeConfirm {
		t.Fatalf("expected the confirm view, got %v", m.viewMode)
	}
	if cmd != nil {
		t.Error("expected no command until the action is confirmed")
	}
	if m.pendingAction == nil || !strings.Contains(m.pendingAction.detail, "sets upstream") {
		t.Errorf("expected a set-upstream note for a branch with no upstream: %+v", m.pendingAction)
	}
}

func TestConfirmRunsPendingAction(t *testing.T) {
	t.Parallel()

	ran := false
	m := detailModel()
	m.viewMode = ViewModeConfirm
	m.pendingAction = &pendingAction{
		prompt:     "Push branch?",
		returnMode: ViewModeRepoDetail,
		run: func(m Model) (Model, tea.Cmd) {
			return m, func() tea.Msg {
				ran = true
				return nil
			}
		},
	}

	updated, cmd := m.handleConfirmKey(keyPress('y'))
	m = mustModel(t, updated)

	if cmd == nil {
		t.Fatal("expected the parked command to run")
	}
	cmd()
	if !ran {
		t.Error("expected the parked command to have been invoked")
	}
	if m.viewMode != ViewModeRepoDetail || m.pendingAction != nil {
		t.Errorf("expected a return to the detail view with no pending action, got %v", m.viewMode)
	}
}

func TestCancelDiscardsPendingAction(t *testing.T) {
	t.Parallel()

	ran := false
	m := detailModel()
	m.viewMode = ViewModeConfirm
	m.pendingAction = &pendingAction{
		returnMode: ViewModeRepoDetail,
		run: func(m Model) (Model, tea.Cmd) {
			return m, func() tea.Msg {
				ran = true
				return nil
			}
		},
	}

	updated, cmd := m.handleConfirmKey(keyPress('n'))
	m = mustModel(t, updated)

	if cmd != nil {
		cmd()
	}
	if ran {
		t.Error("expected a canceled action never to run")
	}
	if m.statusMessage != "Canceled" {
		t.Errorf("expected a canceled status, got %q", m.statusMessage)
	}
}

func TestSwitchBranchRefusesCurrentBranch(t *testing.T) {
	t.Parallel()

	m := detailModel()
	m.detailCursor = 0

	_, cmd := m.startSwitchBranch()
	if cmd == nil {
		t.Fatal("expected a status message command")
	}

	msg, ok := cmd().(StatusMsg)
	if !ok || !strings.Contains(msg.Message, "already checked out") {
		t.Errorf("expected an already-checked-out message, got %+v", msg)
	}
}

func TestSwitchBranchRefusesBranchHeldByPeer(t *testing.T) {
	t.Parallel()

	m := detailModel()
	m.detailCursor = 1
	m.summaries[testRepo1Path] = models.RepoSummary{
		Path: testRepo1Path, Branch: mainBranchName, RemoteRepo: "acme/app",
	}
	m.summaries["/repos/app-feature"] = models.RepoSummary{
		Path: "/repos/app-feature", Branch: featureBranchName, RemoteRepo: "acme/app",
	}

	_, cmd := m.startSwitchBranch()
	if cmd == nil {
		t.Fatal("expected a status message command")
	}

	msg, ok := cmd().(StatusMsg)
	if !ok || !strings.Contains(msg.Message, "app-feature") {
		t.Errorf("expected the holding checkout to be named, got %+v", msg)
	}
}

func TestCreatePRRefusesBranchWithOpenPR(t *testing.T) {
	t.Parallel()

	m := detailModel()
	m.detailCursor = 1

	updated, cmd := m.startCreatePR()
	if mustModel(t, updated).viewMode == ViewModeConfirm {
		t.Error("expected no confirmation for a branch that already has a PR")
	}

	msg, ok := cmd().(StatusMsg)
	if !ok || !strings.Contains(msg.Message, "already has an open PR") {
		t.Errorf("expected an existing-PR message, got %+v", msg)
	}
}

func TestSquashMergeUsesTheBranchesPR(t *testing.T) {
	t.Parallel()

	m := detailModel()
	m.detailCursor = 1

	updated, _ := m.startSquashMergePR()
	m = mustModel(t, updated)

	if m.viewMode != ViewModeConfirm {
		t.Fatalf("expected the confirm view, got %v", m.viewMode)
	}
	if !strings.Contains(m.pendingAction.detail, "#42") {
		t.Errorf("expected PR #42 in the prompt, got %q", m.pendingAction.detail)
	}
}

func TestSquashMergeWithoutPR(t *testing.T) {
	t.Parallel()

	m := detailModel()
	m.detailCursor = 0

	_, cmd := m.startSquashMergePR()
	msg, ok := cmd().(StatusMsg)
	if !ok || !strings.Contains(msg.Message, "No pull request") {
		t.Errorf("expected a no-PR message, got %+v", msg)
	}
}

func TestActionResultReloadsOnSuccess(t *testing.T) {
	t.Parallel()

	m := detailModel()

	updated, cmd := m.handleActionResult(ActionResultMsg{
		Path: testRepo1Path, Message: "Pushed feature to origin", Success: true,
	})
	if mustModel(t, updated).statusMessage != "Pushed feature to origin" {
		t.Error("expected the action message in the status bar")
	}
	if cmd == nil {
		t.Error("expected a reload command after a successful action")
	}
}

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
	next, _ := m.handleDetailKey(keyPress('t'))
	if jumped := mustModel(t, next); jumped.focusedPanel != panelStashes {
		t.Errorf("t with the menu closed focused %v, want the stashes panel", jumped.focusedPanel)
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
	menu := plainText(opened.renderView())
	if !strings.Contains(menu, "push") || !strings.Contains(menu, "feature/thing") {
		t.Errorf("the open menu should list the branch verbs against the selection, got %q", menu)
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

// The menu used to be a footer line, so a pull request title wider than the
// terminal pushed the verbs off the end of it.
func TestPanelActionMenuKeepsItsVerbsUnderALongTitle(t *testing.T) {
	t.Parallel()

	const width = 100

	m := focusedModel(width, 30)
	m.viewMode = ViewModePRList
	m.prSearchCursor = 0
	m.prSearch = []models.PRInfo{{
		Number: 10,
		Title:  strings.TrimSpace(strings.Repeat("bump the go-dependencies group across 1 directory ", 4)),
	}}
	m.panelActions = true

	view := plainText(m.renderView())
	for _, verb := range prListActions() {
		if !strings.Contains(view, verb.name) {
			t.Errorf("the menu dropped %q:\n%s", verb.name, view)
		}
	}

	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("line %d is %d cells wide, past the %d the terminal has: %q", i, got, width, line)
		}
	}
}

func TestStashFullDiffVerbSwapsTheDetailPane(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 45)
	m.focusedPanel = panelStashes
	m.panelActions = true
	m.stashDiffstat = map[int]string{0: " one.go | 2 +-"}

	toggledModel, cmd := m.handleDetailKey(keyPress('o'))
	toggled := mustModel(t, toggledModel)
	if !toggled.stashFullDiff {
		t.Fatal("the verb should swap the pane to the full diff")
	}
	if cmd == nil {
		t.Error("a diff nothing has read yet should be fetched")
	}

	if pane := plainText(strings.Join(toggled.stashDetailLines(60), "\n")); !strings.Contains(pane, "loading diff…") {
		t.Errorf("a diff still loading must say so, got %q", pane)
	}

	// An external viewer's own colors reach the pane already styled, and
	// restyling them would end that styling at the viewer's first reset.
	const colored = "\x1b[92m+new\x1b[0m"

	loadedModel, _ := toggled.Update(StashDiffLoadedMsg{
		Path: toggled.selectedRepo, Index: 0, Diff: "@@ -1 +1 @@\n-old\n" + colored,
	})
	loaded := mustModel(t, loadedModel)

	rendered := strings.Join(loaded.stashDetailLines(60), "\n")
	if !strings.Contains(rendered, colored) {
		t.Errorf("the pane dropped the viewer's own styling, got %q", rendered)
	}

	pane := plainText(rendered)
	if !strings.Contains(pane, "+new") || strings.Contains(pane, "one.go") {
		t.Errorf("the pane should show the patch instead of the diffstat, got %q", pane)
	}

	loaded.panelActions = true
	backModel, backCmd := loaded.handleDetailKey(keyPress('o'))
	back := mustModel(t, backModel)
	if back.stashFullDiff {
		t.Error("toggling again should return to the diffstat")
	}
	if backCmd != nil {
		t.Error("a cached diffstat should not be re-read")
	}
	if pane := plainText(strings.Join(back.stashDetailLines(60), "\n")); !strings.Contains(pane, "one.go") {
		t.Errorf("the pane should show the diffstat again, got %q", pane)
	}
}

// Everything that destroys work is parked behind a confirmation, and applying a
// stash is not one of those things because the stash survives it.
func TestDestructivePanelVerbsAskFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		panel       panelID
		verb        rune
		wantConfirm bool
	}{
		{name: "deleting a branch asks", panel: panelBranches, verb: 'd', wantConfirm: true},
		{name: "dropping a stash asks", panel: panelStashes, verb: 'd', wantConfirm: true},
		{name: "applying a stash does not", panel: panelStashes, verb: 'a', wantConfirm: false},
		{name: "pruning the remote asks", panel: panelStatus, verb: 'p', wantConfirm: true},
		{name: "fetching does not", panel: panelStatus, verb: 'f', wantConfirm: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := focusedModel(160, 45)
			m.focusedPanel = tt.panel
			m.branches = []models.BranchInfo{{Name: "spike/unheld"}}
			m.panelActions = true

			ranModel, _ := m.handleDetailKey(keyPress(tt.verb))
			ran := mustModel(t, ranModel)

			if got := ran.viewMode == ViewModeConfirm; got != tt.wantConfirm {
				t.Errorf("confirmation shown = %v, want %v", got, tt.wantConfirm)
			}
		})
	}
}

// A branch git cannot delete is refused before the confirmation, so the prompt
// never offers something that is going to fail.
func TestDeleteBranchRefusesACheckedOutBranch(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 45)
	m.focusedPanel = panelBranches
	m.branches = []models.BranchInfo{{Name: mainBranchName, IsCurrent: true}}

	next, cmd := m.startDeleteBranch()
	if mustModel(t, next).viewMode == ViewModeConfirm {
		t.Error("deleting the checked-out branch should be refused, not confirmed")
	}
	if cmd == nil {
		t.Error("the refusal should say why")
	}
}
