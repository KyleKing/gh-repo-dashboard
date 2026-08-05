//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// detailModel builds a repo-detail model sitting on the branches tab with two
// branches: current "main" and "feature" with an open PR.
func detailModel() Model {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.detailTab = DetailTabBranches
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
