package app

import (
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/keyusage"
	"github.com/kyleking/aragonite/tui/keyhint"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// UsageApp names the usage log and its directory override variable.
const UsageApp = "gh-repo-dashboard"

// StaleAfter is how long a key may go unpressed before `gh-repo-dashboard keys` lists it as neglected.
const StaleAfter = 28 * 24 * time.Hour

const (
	ctxBranchDetail = "branch-detail"
	ctxConfirm      = "confirm"
	ctxFilter       = "filter"
	ctxHelp         = "help"
	ctxPalette      = "palette"
	ctxPRMap        = "pr-map"
	ctxRepoDetail   = "repo-detail"
	ctxRepoList     = "repo-list"
	ctxSort         = "sort"
	ctxTaskLog      = "task-log"
	ctxTaskPreview  = "task-preview"
	ctxTaskRun      = "task-run"
	ctxTasks        = "tasks"
)

// usage is the recorder for this session and what earlier sessions logged.
type usage struct {
	rec  *keyusage.Recorder
	seen *keyusage.Usage
	keys keyusage.Keymap
	now  time.Time
}

// WithUsage records command keys to rec and brightens the hints that seen
// shows unpressed. A nil rec records nothing and a nil seen brightens nothing.
func (m Model) WithUsage(rec *keyusage.Recorder, seen *keyusage.Usage) Model {
	m.usage = &usage{rec: rec, seen: seen, keys: Keymap(), now: time.Now()}

	return m
}

// keyContext names where the key about to be dispatched will land. A chord's
// later keys share its leader's context.
func (m Model) keyContext() string { return viewContext(m.viewMode) }

func viewContext(mode ViewMode) string {
	switch mode {
	case ViewModeRepoDetail:
		return ctxRepoDetail
	case ViewModeBranchDetail:
		return ctxBranchDetail
	case ViewModeHelp:
		return ctxHelp
	case ViewModeFilter:
		return ctxFilter
	case ViewModeSort:
		return ctxSort
	case ViewModeTaskRun:
		return ctxTaskRun
	case ViewModeConfirm:
		return ctxConfirm
	case ViewModePRMap:
		return ctxPRMap
	case ViewModePalette:
		return ctxPalette
	case ViewModeTasks:
		return ctxTasks
	case ViewModeTaskPreview:
		return ctxTaskPreview
	case ViewModeTaskLog:
		return ctxTaskLog
	case ViewModeRepoList:
		return ctxRepoList
	}

	return ctxRepoList
}

// recordKey logs msg unless a text prompt will take it as typed text.
func (m Model) recordKey(msg tea.KeyMsg) {
	if m.usage == nil || m.typing() {
		return
	}

	m.usage.rec.Key(m.keyContext(), msg.String())
}

func (m Model) typing() bool {
	return m.searching || m.commandMode ||
		(m.viewMode == ViewModePalette && !m.paletteResults && !m.paletteActions)
}

// brighten marks the hints whose key the log shows unpressed, and the pages under them.
func (m Model) brighten(ctx string, hints []keyhint.Hint, under ...string) []keyhint.Hint {
	if m.usage == nil || m.usage.seen == nil {
		return hints
	}

	out := slices.Clone(hints)
	for i := range out {
		h := &out[i]
		if h.Head || h.Key == "" {
			continue
		}

		at := append(slices.Clone(under), h.Key)
		if path, ok := m.usage.path(ctx, at); ok {
			h.Fresh = m.usage.seen.Level(ctx, path, m.usage.now)
		}

		h.Kids = m.brighten(ctx, h.Kids, at...)
	}

	return out
}

// path is the keymap path of the keys pressed in ctx, absent when any is not a single binding.
func (u *usage) path(ctx string, keys []string) (string, bool) {
	bs, path := u.keys[ctx], make([]string, 0, len(keys))

	for _, k := range keys {
		i := slices.IndexFunc(bs, func(b keyusage.Binding) bool { return slices.Contains(b.Keys, k) })
		if i < 0 {
			return "", false
		}

		path, bs = append(path, bs[i].Keys[0]), bs[i].Kids
	}

	return strings.Join(path, " "), true
}

// Keymap is every context keyContext names, built from the tables the views
// dispatch on and draw their legends from.
func Keymap() keyusage.Keymap {
	k := DefaultKeyMap()
	nav := bindings(k.Up, k.Down, k.Top, k.Bottom)
	leaders := append(bindings(k.Quit, k.Help), listGrammar()...)

	return keyusage.Keymap{
		ctxRepoList: slices.Concat(nav, leaders, bindings(k.Enter, k.Find, k.Back, k.Tab, k.Command, k.Refresh,
			k.Filter, k.Sort, k.Search, k.Detail, k.Peers, k.Repeat), []keyusage.Binding{
			{Keys: []string{shellKey}, What: "shell in the repo"},
			{Keys: []string{tabKeyTasks}, What: "tasks tab"},
			{Keys: []string{"g"}, What: "go", Kids: []keyusage.Binding{{Keys: []string{"g"}, What: "top"}}},
		}),
		ctxRepoDetail: slices.Concat(nav, bindings(k.Quit, k.Help, k.Left, k.Right, k.Tab, k.Enter, k.Back,
			k.Find, k.Refresh, k.OpenDetail), []keyusage.Binding{
			{Keys: []string{shellKey}, What: "shell in the repo"},
			{Keys: []string{actionLeader}, What: "verbs for the panel", Kids: panelVerbs()},
		}, panelJumps()),
		ctxBranchDetail: bindings(k.Quit, k.Help, k.Back, k.Refresh, k.CopyBranch, k.SwitchBranch, k.PushBranch,
			k.CreatePR, k.OpenURL, k.MergePR),
		ctxHelp:        slices.Concat(nav, bindings(k.Quit, k.Help, k.Back), helpPages()),
		ctxFilter:      append(bindings(k.Quit, k.Back), filterKeys()...),
		ctxSort:        append(bindings(k.Quit, k.Back), sortKeys()...),
		ctxPRMap:       slices.Concat(nav, bindings(k.Quit, k.Help, k.Back, k.Enter, k.OpenURL)),
		ctxPalette:     paletteBindings(k),
		ctxTasks:       slices.Concat(nav, bindings(k.Quit, k.Help, k.Enter, k.Tab, k.Back), tabRepos()),
		ctxTaskPreview: slices.Concat(nav, bindings(k.Quit, k.Help, k.Back, k.Enter)),
		ctxTaskRun:     slices.Concat(nav, bindings(k.Quit, k.Help, k.Back, k.Enter)),
		ctxTaskLog:     slices.Concat(nav, bindings(k.Quit, k.Help, k.Back)),
		ctxConfirm: {
			{Keys: []string{"y", keyEnter}, What: "confirm"},
			{Keys: []string{"n", keyEsc}, What: "decline"},
		},
	}
}

func bindings(bs ...key.Binding) []keyusage.Binding {
	out := make([]keyusage.Binding, len(bs))
	for i, b := range bs {
		out[i] = keyusage.Binding{Keys: b.Keys(), What: b.Help().Desc}
	}

	return out
}

func tabRepos() []keyusage.Binding {
	return []keyusage.Binding{{Keys: []string{tabKeyRepos}, What: "repos tab"}}
}

func verbs(actions []panelAction) []keyusage.Binding {
	out := make([]keyusage.Binding, 0, len(actions))
	for _, a := range actions {
		out = append(out, keyusage.Binding{Keys: []string{a.key}, What: a.name})
	}

	return out
}

func panelVerbs() []keyusage.Binding {
	var out []keyusage.Binding

	for _, id := range panelOrder {
		for _, v := range verbs(panelActionsFor(id)) {
			if !slices.ContainsFunc(out, func(b keyusage.Binding) bool { return b.Keys[0] == v.Keys[0] }) {
				out = append(out, v)
			}
		}
	}

	return out
}

func panelJumps() []keyusage.Binding {
	out := make([]keyusage.Binding, 0, len(panelOrder))
	for _, id := range panelOrder {
		out = append(out, keyusage.Binding{Keys: []string{panelKeys[id]}, What: "jump to " + panelTitles[id]})
	}

	return out
}

// listGrammar is the mark keys, the verb menu, and the operator-then-object sentence.
func listGrammar() []keyusage.Binding {
	objects := map[string]*keyusage.Binding{}

	var seconds []keyusage.Binding

	for _, obj := range textObjects() {
		first, rest := obj.Key[:1], obj.Key[1:]
		if _, ok := objects[first]; !ok {
			seconds = append(seconds, keyusage.Binding{Keys: []string{first}})
			objects[first] = &seconds[len(seconds)-1]
		}

		objects[first].Kids = append(objects[first].Kids, keyusage.Binding{Keys: []string{rest}, What: obj.Name})
	}

	ops := make([]keyusage.Binding, 0, len(operators()))
	for _, op := range operators() {
		kids := slices.Concat(
			[]keyusage.Binding{{Keys: []string{strings.ToLower(op.Key)}, What: "the whole filtered set"}},
			cloneBindings(seconds))
		ops = append(ops, keyusage.Binding{Keys: []string{op.Key}, What: op.Task, Kids: kids})
	}

	return []keyusage.Binding{
		{Keys: []string{markToggleKey}, What: "mark the row"},
		{Keys: []string{visualKey}, What: "visual range"},
		{Keys: []string{actionLeader}, What: "verbs", Kids: verbs(listActions(false))},
		{Keys: []string{operatorLeader}, What: "operator", Kids: ops},
	}
}

func cloneBindings(bs []keyusage.Binding) []keyusage.Binding {
	out := slices.Clone(bs)
	for i := range out {
		out[i].Kids = cloneBindings(out[i].Kids)
	}

	return out
}

func filterKeys() []keyusage.Binding {
	modes := models.SelectableFilterModes()
	out := make([]keyusage.Binding, 0, len(modes)+3)
	out = append(out,
		keyusage.Binding{Keys: []string{"*"}, What: "clear the filters"},
		keyusage.Binding{Keys: []string{"x"}, What: "drop the predicate"},
		keyusage.Binding{Keys: []string{scopeKey}, What: "switch scope"})

	for _, mode := range modes {
		out = append(out, keyusage.Binding{Keys: []string{mode.ShortKey()}, What: mode.String()})
	}

	return out
}

func sortKeys() []keyusage.Binding {
	modes := models.AllSortModes()
	out := make([]keyusage.Binding, 0, len(modes)+3)
	out = append(out,
		keyusage.Binding{Keys: []string{"["}, What: "move up"},
		keyusage.Binding{Keys: []string{"]"}, What: "move down"},
		keyusage.Binding{Keys: []string{"*"}, What: "reset the sorts"})

	for _, mode := range modes {
		out = append(out, keyusage.Binding{Keys: []string{mode.ShortKey()}, What: mode.String()})
	}

	return out
}

// helpPages are the keys that open a page of a legend.
func helpPages() []keyusage.Binding {
	var out []keyusage.Binding

	for _, h := range listHelpTree() {
		if len(h.Kids) > 0 {
			out = append(out, keyusage.Binding{Keys: []string{h.Key}, What: h.What, Kids: hintBindings(h.Kids)})
		}
	}

	return out
}

func hintBindings(hints []keyhint.Hint) []keyusage.Binding {
	var out []keyusage.Binding

	for _, h := range hints {
		if h.Head || h.Key == "" {
			continue
		}

		out = append(out, keyusage.Binding{Keys: []string{h.Key}, What: h.What, Kids: hintBindings(h.Kids)})
	}

	return out
}

// paletteBindings are the keys that act on the results; the query line takes text.
func paletteBindings(k KeyMap) []keyusage.Binding {
	return slices.Concat(bindings(k.Quit, k.Help, k.Back, k.Up, k.Down), []keyusage.Binding{
		{Keys: []string{keyTab, markToggleKey}, What: "mark the result"},
		{Keys: []string{"/", "i"}, What: "back to the query"},
		{Keys: []string{actionLeader}, What: "verbs", Kids: hintBindings(paletteVerbHints())},
	})
}
