package app

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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
	case panelStatus:
		return []panelAction{
			{key: "c", name: "cleanup merged", run: Model.startRepoCleanupMerged},
			{key: "f", name: nameFetch, run: Model.startRepoFetch},
			{key: "o", name: "open on remote", run: Model.openRepoURL},
			{key: "p", name: "prune remote", run: Model.startRepoPruneRemote},
			{key: "y", name: nameCopyPath, run: Model.copyRepoPath},
		}
	case panelBranches:
		return []panelAction{
			{key: "d", name: "delete", run: Model.startDeleteBranch},
			{key: "n", name: "new PR", run: Model.startCreatePR},
			{key: "o", name: "open on remote", run: Model.openBranchURL},
			{key: "p", name: "push", run: Model.startPushBranch},
			{key: "s", name: "switch to it", run: Model.startSwitchBranch},
			{key: "y", name: "copy name", run: Model.copyBranchName},
		}
	case panelPRs:
		return []panelAction{
			{key: "c", name: "check out here", run: Model.startCheckoutPR},
			{key: "m", name: "squash-merge", run: Model.startSquashMergePR},
			{key: "o", name: "open in browser", run: Model.openPRURL},
			{key: "u", name: "copy URL", run: Model.copyPRURL},
			{key: "y", name: "copy number", run: Model.copyPRNumber},
		}
	case panelPeers:
		return []panelAction{
			{key: "y", name: nameCopyPath, run: Model.copyPeerPath},
		}
	case panelNotes:
		return []panelAction{
			{key: "e", name: "edit in $EDITOR", run: Model.editSelectedNote},
			{key: "y", name: nameCopyPath, run: Model.copyNotePath},
		}
	case panelStashes:
		return []panelAction{
			{key: "a", name: "apply here", run: Model.startApplyStash},
			{key: "d", name: "drop", run: Model.startDropStash},
		}
	}

	return nil
}

// startDeleteBranch deletes the branch under the cursor after confirmation.
// Deletion is refused for the branch checked out here or held by a peer, since
// git will not delete either. A branch the squash-merge detection has already
// vouched for is forced, because git alone cannot see that it was merged.
func (m Model) startDeleteBranch() (tea.Model, tea.Cmd) {
	branch, ok := m.actionBranch()
	if !ok {
		return m, nil
	}
	if branch.IsCurrent {
		return m, statusCmd(branch.Name + " is checked out here")
	}
	if peer, held := models.CheckoutForBranch(m.RepoCheckouts(), branch.Name); held {
		return m, statusCmd(branch.Name + " is checked out in " + peer.Folder())
	}

	force := m.deletableBranches[branch.Name]
	detail := branch.Name
	if force {
		detail += " (squash-merged, forces the delete)"
	}

	return m.confirmAction("Delete branch?", detail,
		deleteBranchCmd(m.selectedRepo, branch.Name, force))
}

// startApplyStash restores the selected stash into the working copy. It applies
// rather than pops, so the stash survives a mistake and needs no confirmation.
func (m Model) startApplyStash() (tea.Model, tea.Cmd) {
	stash, ok := m.selectedPanelStash()
	if !ok {
		return m, nil
	}

	return m, applyStashCmd(m.selectedRepo, stash.Index)
}

// startDropStash discards the selected stash after confirmation. Nothing
// recovers a dropped stash.
func (m Model) startDropStash() (tea.Model, tea.Cmd) {
	stash, ok := m.selectedPanelStash()
	if !ok {
		return m, nil
	}

	return m.confirmAction("Drop stash?", stash.Message+" (cannot be undone)",
		dropStashCmd(m.selectedRepo, stash.Index))
}

func deleteBranchCmd(repoPath, branch string, force bool) tea.Cmd {
	return func() tea.Msg {
		ops := vcs.GetOperations(repoPath)
		ok, msg, err := ops.DeleteBranch(context.Background(), repoPath, branch, force)
		if err != nil {
			return ActionResultMsg{Path: repoPath, Message: err.Error()}
		}

		return ActionResultMsg{Path: repoPath, Message: msg, Success: ok}
	}
}

func applyStashCmd(repoPath string, index int) tea.Cmd {
	return func() tea.Msg {
		ops := vcs.GetOperations(repoPath)
		ok, msg, err := ops.ApplyStash(context.Background(), repoPath, index)
		if err != nil {
			return ActionResultMsg{Path: repoPath, Message: err.Error()}
		}

		return ActionResultMsg{Path: repoPath, Message: msg, Success: ok}
	}
}

func dropStashCmd(repoPath string, index int) tea.Cmd {
	return func() tea.Msg {
		ops := vcs.GetOperations(repoPath)
		ok, msg, err := ops.DropStash(context.Background(), repoPath, index)
		if err != nil {
			return ActionResultMsg{Path: repoPath, Message: err.Error()}
		}

		return ActionResultMsg{Path: repoPath, Message: msg, Success: ok}
	}
}

// repoURL is the selected repo's page on its forge, empty when no remote was
// detected. The remote is recorded as "owner/repo", so the host is assumed to
// be GitHub, which is the only forge the PR integration speaks to anyway.
func (m Model) repoURL() string {
	if repo := m.summaries[m.selectedRepo].RemoteRepo; repo != "" {
		return "https://github.com/" + repo
	}

	return ""
}

func (m Model) openRepoURL() (tea.Model, tea.Cmd) {
	url := m.repoURL()
	if url == "" {
		return m, statusCmd("No remote to open")
	}

	return m, openURLCmd(url)
}

func (m Model) copyRepoPath() (tea.Model, tea.Cmd) {
	return m, copyToClipboardCmd(m.selectedRepo)
}

func (m Model) startRepoFetch() (tea.Model, tea.Cmd) {
	return m.confirmBatchTask(taskFetchAll, false, []string{m.selectedRepo}, batchFetchAllCmd)
}

func (m Model) startRepoPruneRemote() (tea.Model, tea.Cmd) {
	return m.confirmBatchTask("Prune Remote", true, []string{m.selectedRepo}, batchPruneRemoteCmd)
}

func (m Model) startRepoCleanupMerged() (tea.Model, tea.Cmd) {
	return m.confirmBatchTask(taskCleanupMerged, true, []string{m.selectedRepo}, batchCleanupMergedCmd)
}

func (m Model) openBranchURL() (tea.Model, tea.Cmd) {
	branch, ok := m.actionBranch()
	url := m.repoURL()
	if !ok || url == "" {
		return m, statusCmd("No remote branch to open")
	}

	return m, openURLCmd(url + "/tree/" + branch.Name)
}

func (m Model) copyBranchName() (tea.Model, tea.Cmd) {
	branch, ok := m.actionBranch()
	if !ok {
		return m, nil
	}

	return m, copyToClipboardCmd(branch.Name)
}

func (m Model) openPRURL() (tea.Model, tea.Cmd) {
	pr, ok := m.actionPR()
	if !ok || pr.URL == "" {
		return m, statusCmd("No pull request URL to open")
	}

	return m, openURLCmd(pr.URL)
}

func (m Model) copyPRURL() (tea.Model, tea.Cmd) {
	pr, ok := m.actionPR()
	if !ok || pr.URL == "" {
		return m, statusCmd("No pull request URL to copy")
	}

	return m, copyToClipboardCmd(pr.URL)
}

func (m Model) copyPRNumber() (tea.Model, tea.Cmd) {
	pr, ok := m.actionPR()
	if !ok {
		return m, nil
	}

	return m, copyToClipboardCmd("#" + strconv.Itoa(pr.Number))
}

func (m Model) copyPeerPath() (tea.Model, tea.Cmd) {
	checkout, ok := m.selectedPanelCheckout()
	if !ok {
		return m, nil
	}

	return m, copyToClipboardCmd(checkout.Path)
}

func (m Model) copyNotePath() (tea.Model, tea.Cmd) {
	note, ok := m.selectedPanelNote()
	if !ok {
		return m, nil
	}

	return m, copyToClipboardCmd(filepath.Join(m.selectedRepo, note.Name))
}

// editSelectedNote hands the terminal to $EDITOR for the note under the cursor
// and reloads the repo's detail once it exits, so an edit made there is on
// screen without a refresh.
func (m Model) editSelectedNote() (tea.Model, tea.Cmd) {
	note, ok := m.selectedPanelNote()
	if !ok {
		return m, nil
	}

	// The editor is split on spaces rather than run through a shell, so a
	// $EDITOR carrying flags ("code -w") works without the string ever reaching
	// an interpreter.
	editor := strings.Fields(cmp.Or(os.Getenv("VISUAL"), os.Getenv("EDITOR")))
	if len(editor) == 0 {
		return m, statusCmd("Set $EDITOR or $VISUAL to edit notes here")
	}

	path := filepath.Join(m.selectedRepo, note.Name)
	repo, name := m.selectedRepo, note.Name

	args := append(slices.Clone(editor[1:]), path)
	// #nosec G204,G702 -- no shell is involved: the binary comes from the
	// operator's own environment and the path is a note file this scan read.
	editorCmd := exec.CommandContext(context.Background(), editor[0], args...)

	return m, tea.ExecProcess(editorCmd, func(err error) tea.Msg {
		if err != nil {
			return StatusMsg{Message: "Editor exited with " + err.Error()}
		}

		return ActionResultMsg{Path: repo, Message: "Reloaded after editing " + name, Success: true}
	})
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
