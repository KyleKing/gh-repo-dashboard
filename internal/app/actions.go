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
// to return to once it's confirmed or canceled.
type pendingAction struct {
	prompt     string
	detail     string
	cmd        tea.Cmd
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
	m.pendingAction = &pendingAction{
		prompt:     prompt,
		detail:     detail,
		cmd:        cmd,
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
		return m, action.cmd
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

	if m.detailTab == DetailTabBranches && m.detailCursor < len(m.branches) {
		return m.branches[m.detailCursor], true
	}

	return models.BranchInfo{}, false
}

// actionPR returns the pull request the current view's actions apply to: the
// selected PR row, the branch's own PR, or the repo's PR.
func (m Model) actionPR() (models.PRInfo, bool) {
	switch {
	case m.viewMode == ViewModePRDetail && m.prDetail.Number > 0:
		return m.prDetail.PRInfo, true
	case m.viewMode == ViewModeRepoDetail && m.detailTab == DetailTabPRs && m.detailCursor < len(m.prs):
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
