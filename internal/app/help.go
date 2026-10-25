package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/overlay"
)

// helpFromView is the view a legend documents. The docks stand for the list.
func helpFromView(mode ViewMode) ViewMode {
	switch mode {
	case ViewModeRepoDetail, ViewModeBranchDetail, ViewModePRMap, ViewModeTaskRun:
		return mode
	default:
		return ViewModeRepoList
	}
}

func (m Model) openHelp() Model {
	m.helpFrom = helpFromView(m.viewMode)
	m.helpPath, m.helpAt = nil, 0
	m.viewMode = ViewModeHelp

	return m
}

func (m Model) closeHelp() Model {
	m.viewMode = m.helpFrom
	m.helpPath, m.helpAt = nil, 0

	return m
}

func (m Model) helpTree() []keyhint.Hint {
	switch m.helpFrom {
	case ViewModeRepoDetail:
		return detailHelpTree()
	case ViewModeBranchDetail:
		return branchHelpTree()
	case ViewModePRMap:
		return prMapHelpTree()
	case ViewModeTaskRun:
		return runHelpTree()
	default:
		return listHelpTree()
	}
}

func (m Model) helpPage() []keyhint.Hint {
	page, ok := keyhint.At(m.helpTree(), m.helpPath)
	if !ok {
		return m.helpTree()
	}

	return page
}

func (m Model) helpRows() []string {
	return keyhint.Column(legendStyles(), m.helpPage(), min(helpBoxWidth, m.width-boxMargin))
}

func (m Model) helpTitle() string {
	if len(m.helpPath) == 0 {
		return "keys"
	}

	return strings.Join(m.helpPath, " then ") + " then"
}

func (m Model) helpScreen() screen {
	under := m
	under.viewMode = m.helpFrom
	base := under.baseScreen()

	rows := m.helpRows()
	body := padLines(strings.Split(base.body, "\n"), m.bodyHeight(), 0)
	inner := max(1, min(len(rows), m.bodyHeight()-boxBorder))
	box := overlay.Box(boxStyles(), m.helpTitle(), rows, inner, m.clampedHelpAt(len(rows), inner))

	x := max(m.width-lipgloss.Width(box[0])-1, 0)
	base.body = strings.Join(overlay.Blit(body, box, x, len(body)-len(box)), "\n")
	base.hints = m.helpHints()
	base.facts = append([]frame.Fact{{Text: "help", Tone: frame.Accent}}, base.facts...)

	return base
}

func (m Model) clampedHelpAt(rows, inner int) int {
	return max(0, min(m.helpAt, rows-inner))
}

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func (m Model) helpHints() []keyhint.Hint {
	back := descClose
	if len(m.helpPath) > 0 {
		back = "up a page"
	}

	return []keyhint.Hint{
		{Key: keyNavPair, What: "scroll", Priority: 3},
		{Key: keyEsc, What: back, Priority: 5},
		{Key: "?", What: descClose, Priority: 4},
		{Key: "q", What: nameQuit, Priority: 2},
	}
}

// handleHelpKey swallows every key it does not use, so none acts on the view behind.
func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pressed := msg.String()

	if _, ok := keyhint.At(m.helpTree(), append(slices.Clone(m.helpPath), pressed)); ok {
		m.helpPath = append(slices.Clone(m.helpPath), pressed)
		m.helpAt = 0

		return m, nil
	}

	switch pressed {
	case keyEsc, "backspace":
		if len(m.helpPath) > 0 {
			m.helpPath = m.helpPath[:len(m.helpPath)-1]
			m.helpAt = 0

			return m, nil
		}

		return m.closeHelp(), nil
	case "?":
		return m.closeHelp(), nil
	case "q", keyCtrlC:
		return m, tea.Quit
	case "j", "down":
		m.helpAt++
	case "k", "up":
		m.helpAt--
	case "pgup", "ctrl+u":
		m.helpAt -= m.bodyHeight() / halfPageDivisor
	case "pgdown", "ctrl+d":
		m.helpAt += m.bodyHeight() / halfPageDivisor
	case "home", "g":
		m.helpAt = 0
	case "end", "G":
		m.helpAt = pastEnd
	}

	m.helpAt = max(0, m.helpAt)

	return m, nil
}

const (
	legendHelp    = "this legend"
	legendRefresh = "refresh, clearing caches"
)

func objectHints() []keyhint.Hint {
	objects := textObjects()
	hints := make([]keyhint.Hint, 0, len(objects)+1)

	for _, obj := range objects {
		hints = append(hints, keyhint.Hint{Key: obj.Key, What: obj.Name})
	}

	return append(hints, keyhint.Hint{What: "the verb's own key again takes the whole filtered set"})
}

func operatorHints() []keyhint.Hint {
	objects := objectHints()

	hints := make([]keyhint.Hint, 0, len(operators()))
	for _, op := range operators() {
		hints = append(hints, keyhint.Hint{
			Key: strings.ToLower(op.Key), What: op.Task, Kids: objects,
		})
	}

	return hints
}

func actionHints(actions []panelAction) []keyhint.Hint {
	hints := make([]keyhint.Hint, 0, len(actions))
	for _, action := range actions {
		hints = append(hints, keyhint.Hint{Key: action.key, What: action.name})
	}

	return hints
}

func panelVerbHints() []keyhint.Hint {
	hints := make([]keyhint.Hint, 0, len(panelOrder))

	for _, id := range panelOrder {
		hints = append(hints, keyhint.Hint{What: panelTitles[id], Head: true})
		hints = append(hints, actionHints(panelActionsFor(id))...)
	}

	return hints
}

func listHelpTree() []keyhint.Hint {
	return []keyhint.Hint{
		{What: "moving", Head: true},
		{Key: keyNavPair, What: "a row"},
		{Key: "gg/G", What: "the ends"},
		{Key: keyEnter, What: "open the repo"},
		{Key: ";", What: "find a branch, stash, note, or PR: #12, b/s/n/r then a space, * fleet-wide"},
		{Key: "]", What: "parallel checkouts of this repo"},
		{Key: keyEsc, What: "close the region, then clear marks, search, and filters"},
		{What: "marking", Head: true},
		{Key: markToggleKey, What: "mark the row and step past it"},
		{Key: visualKey, What: "a range every motion extends"},
		{What: "narrowing", Head: true},
		{Key: "f", What: "filter dock: enter or its key cycles, * resets"},
		{Key: "s", What: "sort dock: enter or its key cycles, [/] reorders, * resets"},
		{Key: "/", What: "search: r: b: p: t: n: c: scope it, * ? ^ $ glob or anchor"},
		{What: ":filter takes a boolean expression, like dirty and (ahead or behind)"},
		{What: "doing", Head: true},
		{Key: "v", What: "read the repo under the cursor in full"},
		{Key: actionLeader, What: "verbs for what is marked, or the fleet", Kids: actionHints(listActions(false))},
		{Key: operatorLeader, What: "an operator, then a text object", Kids: operatorHints()},
		{Key: "r", What: legendRefresh},
		{What: "commands", Head: true},
		{Key: ":", What: "command prompt, with :help listing them"},
		{Key: "@:", What: "repeat the last command"},
		{What: ":history lists recent commands, :prs maps open PRs to branches and checkouts"},
		{What: "general", Head: true},
		{Key: "?", What: legendHelp},
		{Key: "q", What: nameQuit},
	}
}

func detailHelpTree() []keyhint.Hint {
	return []keyhint.Hint{
		{What: "moving", Head: true},
		{Key: keyNavPair, What: "a row, crossing from one panel into the next"},
		{Key: keyTab + "/l", What: "the next panel"},
		{Key: "h", What: "the previous panel"},
		{Key: panelJumpKeys(), What: "jump to a panel, as bracketed in its title"},
		{Key: keyEnter, What: "into the detail pane, where j/k scroll and esc returns"},
		{Key: "O", What: "the selected branch full screen, or the checkout's repo"},
		{Key: "space", What: "find within this repo: #12, b/s/n/r then a space, * fleet-wide"},
		{Key: keyEsc, What: "back to the list"},
		{What: "doing", Head: true},
		{Key: actionLeader, What: "verbs for the focused panel's selection", Kids: panelVerbHints()},
		{Key: "r", What: legendRefresh},
		{What: "general", Head: true},
		{Key: "?", What: legendHelp},
		{Key: "q", What: nameQuit},
	}
}

func branchHelpTree() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: "b", What: "copy the branch name"},
		{Key: "c", What: "switch to it"},
		{Key: "p", What: "push it with its tags"},
		{Key: "N", What: "open a PR for it"},
		{Key: "o", What: "open its PR in the browser"},
		{Key: "M", What: "squash-merge its PR and delete the branch"},
		{Key: "r", What: legendRefresh},
		{Key: keyEsc, What: "back to the repo"},
		{Key: "?", What: legendHelp},
		{Key: "q", What: nameQuit},
	}
}

func runHelpTree() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: keyNavPair, What: "a repo"},
		{Key: keyCtrlC, What: "cancel the run: running repos stop and queued repos are canceled"},
		{Key: keyEsc, What: "back, once the run has finished"},
		{Key: "?", What: legendHelp},
		{Key: "q", What: "quit, once the run has finished"},
	}
}

func prMapHelpTree() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: keyNavPair, What: "a row"},
		{Key: "gg/G", What: "the ends"},
		{Key: keyEnter, What: "open the repo the row belongs to"},
		{Key: "o", What: "open the pull request in the browser"},
		{Key: keyEsc, What: "back to the list"},
		{Key: "?", What: legendHelp},
		{Key: "q", What: nameQuit},
	}
}
