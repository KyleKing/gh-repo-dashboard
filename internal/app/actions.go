package app

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// pendingAction is a write action awaiting confirmation, along with the view
// to return to once it's confirmed or canceled. An empty scope means the
// action runs against the selected repo.
type pendingAction struct {
	prompt     string
	detail     string
	scope      string
	run        func(Model) (Model, tea.Cmd)
	returnMode ViewMode
}

// switchBranchCmd moves the repo's working copy onto branch.
func switchBranchCmd(repoPath, branch string) tea.Cmd {
	return func() tea.Msg {
		ops := vcs.GetOperations(repoPath)
		ok, msg, err := ops.SwitchBranch(context.Background(), repoPath, branch)
		if err != nil {
			return ActionResultMsg{Path: repoPath, Message: err.Error()}
		}

		return ActionResultMsg{Path: repoPath, Message: msg, Success: ok}
	}
}

// pushBranchCmd pushes branch and its reachable tags, recording tracking for a
// branch that has no upstream yet.
func pushBranchCmd(repoPath, branch string, setUpstream bool) tea.Cmd {
	return func() tea.Msg {
		ops := vcs.GetOperations(repoPath)
		ok, msg, err := ops.PushBranch(context.Background(), repoPath, branch, setUpstream)
		if err != nil {
			return ActionResultMsg{Path: repoPath, Message: err.Error()}
		}

		return ActionResultMsg{Path: repoPath, Message: msg, Success: ok}
	}
}

// checkoutPRCmd checks a pull request out locally, fetching its head ref.
func checkoutPRCmd(repoPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		branch, err := github.CheckoutPR(context.Background(), repoPath, prNumber)
		if err != nil {
			return ActionResultMsg{Path: repoPath, Message: "Failed to check out PR: " + err.Error()}
		}

		return ActionResultMsg{Path: repoPath, Message: "Checked out " + branch, Success: true}
	}
}

// createPRCmd opens a pull request for branch against base.
func createPRCmd(repoPath, branch, base string) tea.Cmd {
	return func() tea.Msg {
		url, err := github.CreatePR(context.Background(), repoPath, branch, base)
		if err != nil {
			return ActionResultMsg{Path: repoPath, Message: "Failed to create PR: " + err.Error()}
		}

		return ActionResultMsg{Path: repoPath, Message: "Created " + url, Success: true}
	}
}

// squashMergePRCmd squash-merges a pull request and deletes its head branch.
func squashMergePRCmd(repoPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		if err := github.SquashMergePR(context.Background(), repoPath, prNumber); err != nil {
			return ActionResultMsg{Path: repoPath, Message: "Failed to merge PR: " + err.Error()}
		}

		return ActionResultMsg{
			Path:    repoPath,
			Message: fmt.Sprintf("Squash-merged PR #%d", prNumber),
			Success: true,
		}
	}
}

// confirmAction parks cmd behind a confirmation prompt instead of running it.
func (m Model) confirmAction(prompt, detail string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	return m.confirmRun(prompt, detail, "", func(m Model) (Model, tea.Cmd) {
		return m, cmd
	})
}

// confirmRun parks a whole model transition behind a confirmation prompt, for
// actions that change view state as well as firing a command.
func (m Model) confirmRun(
	prompt, detail, scope string, run func(Model) (Model, tea.Cmd),
) (Model, tea.Cmd) {
	m.pendingAction = &pendingAction{
		prompt:     prompt,
		detail:     detail,
		scope:      scope,
		run:        run,
		returnMode: m.viewMode,
	}
	m.viewMode = ViewModeConfirm

	return m, nil
}

// handleConfirmKey answers the confirmation prompt: y/enter runs the parked
// action, anything else cancels it.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	action := m.pendingAction
	if action == nil {
		m.viewMode = ViewModeRepoList
		return m, nil
	}

	m.viewMode = action.returnMode
	m.pendingAction = nil

	switch msg.String() {
	case "y", "Y", keyEnter:
		return action.run(m)
	default:
		m.statusMessage = "Canceled"
		return m, clearStatusAfterDelay()
	}
}

// handleActionResult reports a write action's outcome and reloads the repo's
// detail so the branch list reflects it.
func (m Model) handleActionResult(msg ActionResultMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = msg.Message
	if !msg.Success {
		return m, clearStatusAfterDelay()
	}

	return m, tea.Batch(loadRepoSummaryCmd(msg.Path), loadDetailCmd(msg.Path), clearStatusAfterDelay())
}

// actionBranch returns the branch the current view's actions apply to.
func (m Model) actionBranch() (models.BranchInfo, bool) {
	if m.viewMode == ViewModeBranchDetail {
		if m.branchDetail.Branch.Name != "" {
			return m.branchDetail.Branch, true
		}

		return m.selectedBranch, m.selectedBranch.Name != ""
	}

	if branch, ok := m.selectedPanelBranch(); ok {
		return branch, true
	}

	return models.BranchInfo{}, false
}

// actionPR returns the pull request the current view's actions apply to: the
// selected PR row, the branch's own PR, or the repo's PR.
func (m Model) actionPR() (models.PRInfo, bool) {
	switch {
	case m.viewMode == ViewModePRDetail && m.prDetail.Number > 0:
		return m.prDetail.PRInfo, true
	case m.viewMode == ViewModeRepoDetail && m.focusedPanel == panelPRs && m.detailCursor < len(m.prs):
		return m.prs[m.detailCursor], true
	}

	branch, ok := m.actionBranch()
	if !ok {
		return models.PRInfo{}, false
	}
	if pr, found := prsByHeadRef(m.prs)[branch.Name]; found {
		return *pr, true
	}
	if m.viewMode == ViewModeBranchDetail && m.branchDetail.PRInfo != nil {
		return *m.branchDetail.PRInfo, true
	}

	return models.PRInfo{}, false
}

// startSwitchBranch checks out the branch under the cursor. Switching to the
// branch already checked out here, or to one held by a parallel checkout,
// would fail in git, so both are refused up front.
func (m Model) startSwitchBranch() (tea.Model, tea.Cmd) {
	branch, ok := m.actionBranch()
	if !ok {
		return m, nil
	}
	if branch.IsCurrent {
		return m, statusCmd(branch.Name + " is already checked out here")
	}
	if peer, held := models.CheckoutForBranch(m.RepoCheckouts(), branch.Name); held {
		return m, statusCmd(branch.Name + " is checked out in " + peer.Folder())
	}

	return m, switchBranchCmd(m.selectedRepo, branch.Name)
}

// startCheckoutPR checks the pull request under the cursor out locally after
// confirmation. A branch a parallel checkout already holds is refused, for the
// same reason switching to it is: git will not check one branch out twice.
func (m Model) startCheckoutPR() (tea.Model, tea.Cmd) {
	pr, ok := m.actionPR()
	if !ok {
		return m, statusCmd("No pull request under the cursor")
	}
	if peer, held := models.CheckoutForBranch(m.RepoCheckouts(), pr.HeadRef); held {
		return m, statusCmd(pr.HeadRef + " is checked out in " + peer.Folder())
	}
	for _, branch := range m.branches {
		if branch.Name == pr.HeadRef && branch.IsCurrent {
			return m, statusCmd(pr.HeadRef + " is already checked out here")
		}
	}

	detail := fmt.Sprintf("#%d %s → %s", pr.Number, pr.Title, pr.HeadRef)

	return m.confirmAction("Check the PR branch out here?", detail,
		checkoutPRCmd(m.selectedRepo, pr.Number))
}

// startPushBranch pushes the branch under the cursor after confirmation.
func (m Model) startPushBranch() (tea.Model, tea.Cmd) {
	branch, ok := m.actionBranch()
	if !ok {
		return m, nil
	}

	setUpstream := branch.Upstream == ""
	target := "origin/" + branch.Name
	if !setUpstream {
		target = branch.Upstream
	}

	detail := fmt.Sprintf("%s → %s, with reachable tags", branch.Name, target)
	if setUpstream {
		detail += " (sets upstream)"
	}

	return m.confirmAction("Push branch?", detail, pushBranchCmd(m.selectedRepo, branch.Name, setUpstream))
}

// startCreatePR opens a pull request for the branch under the cursor after
// confirmation, basing it on the repo's default branch.
func (m Model) startCreatePR() (tea.Model, tea.Cmd) {
	branch, ok := m.actionBranch()
	if !ok {
		return m, nil
	}
	if _, exists := prsByHeadRef(m.prs)[branch.Name]; exists {
		return m, statusCmd(branch.Name + " already has an open PR")
	}

	base := findDefaultBranch(m.branches)
	detail := branch.Name
	if base != "" {
		detail += " → " + base
	}

	return m.confirmAction("Create pull request?", detail, createPRCmd(m.selectedRepo, branch.Name, base))
}

// startSquashMergePR squash-merges the current view's pull request after
// confirmation, deleting its head branch.
func (m Model) startSquashMergePR() (tea.Model, tea.Cmd) {
	pr, ok := m.actionPR()
	if !ok {
		return m, statusCmd("No pull request for this branch")
	}

	detail := fmt.Sprintf("#%d %s (deletes %s)", pr.Number, pr.Title, pr.HeadRef)

	return m.confirmAction("Squash-merge pull request?", detail, squashMergePRCmd(m.selectedRepo, pr.Number))
}

// panelActionLeader opens the verb menu for whatever the focused panel has
// selected. It is the same key the universal find uses to act on its result
// set, so one idiom covers both.
const panelActionLeader = "!"

// panelAction is one verb the leader key offers. Keys are scoped to a panel,
// so the same letter can mean different things in different panels and each
// stays mnemonic.
type panelAction struct {
	key  string
	name string
	run  func(Model) (tea.Model, tea.Cmd)
}

// panelActionsFor returns the verbs the focused panel's selection supports.
func panelActionsFor(id panelID) []panelAction {
	switch id {
	case panelBranches:
		return []panelAction{
			{key: "n", name: "new PR", run: Model.startCreatePR},
			{key: "p", name: "push", run: Model.startPushBranch},
			{key: "s", name: "switch to it", run: Model.startSwitchBranch},
		}
	case panelPRs:
		return []panelAction{
			{key: "c", name: "check out here", run: Model.startCheckoutPR},
			{key: "m", name: "squash-merge", run: Model.startSquashMergePR},
		}
	case panelStatus, panelPeers, panelStashes, panelNotes:
		return nil
	}

	return nil
}

// handlePanelActionKey answers the open verb menu: a verb key runs it against
// the current selection, anything else backs out.
func (m Model) handlePanelActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.panelActions = false

	for _, action := range panelActionsFor(m.focusedPanel) {
		if action.key == msg.String() {
			return action.run(m)
		}
	}

	return m, nil
}
