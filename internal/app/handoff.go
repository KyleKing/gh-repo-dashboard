package app

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/handoff"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

const shellKey = "S"

// handoffTarget is the one row a hand-off runs for. An empty field is a value
// the row does not have, so a template needing it is not offered.
type handoffTarget struct {
	path, repo, pr, branch string
}

func (t handoffTarget) values() map[string]string {
	return map[string]string{"path": t.path, "repo": t.repo, "pr": t.pr, "branch": t.branch}
}

func (t handoffTarget) supports(task config.Task) bool {
	values := t.values()
	if values["path"] == "" {
		return false
	}

	for _, name := range handoff.Needs(task.Run) {
		if values[name] == "" {
			return false
		}
	}

	return true
}

func summaryTarget(s models.RepoSummary) handoffTarget {
	t := handoffTarget{path: s.Path, repo: cmp.Or(s.RemoteRepo, s.Name())}

	if !s.IsDetached() {
		t.branch = s.Branch
	}

	if s.PRInfo != nil {
		t.pr = strconv.Itoa(s.PRInfo.Number)
	}

	return t
}

// listTarget is the repo under the cursor in the list.
func (m Model) listTarget() (handoffTarget, bool) {
	if m.cursor >= len(m.filteredPaths) {
		return handoffTarget{}, false
	}

	s, ok := m.summaries[m.filteredPaths[m.cursor]]

	return summaryTarget(s), ok
}

// focusTarget is the item the focused view's cursor is on, falling back to the
// repo for sections that name no branch or PR.
func (m Model) focusTarget() (handoffTarget, bool) {
	s, ok := m.summaries[m.selectedRepo]
	if !ok {
		return handoffTarget{}, false
	}

	t := summaryTarget(s)

	switch m.focusedPanel {
	case panelPRs:
		if pr, ok := m.selectedPanelPR(); ok {
			t.pr, t.branch = strconv.Itoa(pr.Number), pr.HeadRef
		}
	case panelBranches:
		if branch, ok := m.selectedPanelBranch(); ok {
			t.branch, t.pr = branch.Name, ""
			if pr, found := prsByHeadRef(m.prs)[branch.Name]; found {
				t.pr = strconv.Itoa(pr.Number)
			}
		}
	case panelCI:
		t.pr = ""
		if s.CI != nil {
			t.branch = s.CI.Branch
		}
	case panelPeers:
		if checkout, ok := m.selectedPanelCheckout(); ok {
			t.path, t.branch, t.pr = checkout.Path, checkout.Branch, ""
		}
	case panelStatus, panelStashes, panelNotes:
	}

	return t, true
}

// resultTarget is a find result when it is the only one in the set.
func (m Model) resultTarget(results []findResult) (handoffTarget, bool) {
	if len(results) != 1 {
		return handoffTarget{}, false
	}

	r := results[0]

	s, ok := m.summaries[r.repo]
	if !ok {
		return handoffTarget{}, false
	}

	t := summaryTarget(s)

	switch r.kind {
	case findPR:
		t.pr, t.branch = strconv.Itoa(r.number), r.branch
	case findBranch:
		t.pr, t.branch = "", r.branch
	case findRepo, findStash, findNote, findAny:
	}

	return t, true
}

// handoffActions offers each interactive task whose placeholders the target
// has a value for. A key is the first letter or digit of the task's name that
// no earlier verb in the menu has taken.
func (m Model) handoffActions(taken []string, target handoffTarget) []panelAction {
	var actions []panelAction

	for _, task := range m.handoffList() {
		if !target.supports(task) {
			continue
		}

		key := handoffKey(task.Name, taken)
		if key == "" {
			continue
		}

		taken = append(taken, key)
		actions = append(actions, panelAction{
			key:  key,
			name: cmp.Or(task.Desc, task.Name) + " (hand-off)",
			run:  func(m Model) (tea.Model, tea.Cmd) { return m.startHandoff(task, target) },
		})
	}

	return actions
}

func handoffKey(name string, taken []string) string {
	for _, r := range strings.ToLower(name) + "123456789" {
		if (r >= 'a' && r <= 'z' || r >= '0' && r <= '9') && !slices.Contains(taken, string(r)) {
			return string(r)
		}
	}

	return ""
}

// withHandoffs appends the hand-offs the target supports to a menu's verbs.
func (m Model) withHandoffs(actions []panelAction, target handoffTarget, ok bool) []panelAction {
	if !ok {
		return actions
	}

	taken := make([]string, 0, len(actions))
	for _, a := range actions {
		taken = append(taken, a.key)
	}

	return append(slices.Clone(actions), m.handoffActions(taken, target)...)
}

func (m Model) startHandoff(task config.Task, target handoffTarget) (tea.Model, tea.Cmd) {
	env := []string{"REPO_NAME=" + target.repo, "REPO_PATH=" + target.path}

	cmd, err := handoff.Run(task.Name, task.Run, target.values(), target.path, env)
	if err != nil {
		return m, statusCmd(err.Error())
	}

	if task.Confirm {
		return m.confirmAction("Hand the terminal to "+task.Name+"?", target.path, cmd)
	}

	return m, cmd
}

// handleShellKey hands the terminal to the built-in shell for the row under the
// cursor, in the list or the focused view.
func (m Model) handleShellKey() (tea.Model, tea.Cmd) {
	target, ok := m.listTarget()
	if m.viewMode == ViewModeRepoDetail {
		target, ok = m.focusTarget()
	}

	if !ok {
		return m, nil
	}

	for _, task := range m.handoffList() {
		if task.Name == tasks.NameShell {
			return m.startHandoff(task, target)
		}
	}

	return m, nil
}

// handleHandoffDone re-reads the repo the tool may have changed and reports how it exited.
func (m Model) handleHandoffDone(msg handoff.DoneMsg) (tea.Model, tea.Cmd) {
	message := msg.Name + " exited"
	if msg.Err != nil {
		message = msg.Name + ": " + msg.Err.Error()
	}

	return m.handleActionResult(ActionResultMsg{Path: msg.Dir, Message: message, Success: true})
}

// paletteActionSet is the verbs for a result set, plus the hand-offs when the
// set is the one result a hand-off can run for.
func (m Model) paletteActionSet(targets []findResult) []paletteAction {
	actions := paletteActionsFor(homogeneousKind(targets))

	target, ok := m.resultTarget(targets)
	if !ok {
		return actions
	}

	taken := make([]string, 0, len(actions))
	for _, a := range actions {
		taken = append(taken, a.key)
	}

	for _, a := range m.handoffActions(taken, target) {
		run := a.run
		actions = append(
			actions,
			paletteAction{key: a.key, name: a.name, run: func(m Model, _ []findResult) (Model, tea.Cmd) {
				next, cmd := run(m)

				return mustAppModel(next), cmd
			}},
		)
	}

	return actions
}
