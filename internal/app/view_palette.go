package app

import (
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/forge/github"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/table"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// paletteMaxResults caps the result list. Past this the list stops being
// scannable, and a narrower query is the better answer than more rows.
const paletteMaxResults = 200

// findResults answers the current palette query from cache-resident data only,
// so no keystroke costs a fetch. Repo scope covers the focused repo; fleet
// scope covers every discovered repo.
func (m Model) findResults() []findResult {
	q := parseFindQuery(m.paletteInput.Value())
	fleet := m.paletteFleetScope || q.fleet

	var results []findResult
	if q.wants(findRepo) {
		results = append(results, m.findRepos(q, fleet)...)
	}
	if q.wants(findPR) {
		results = append(results, m.findPRs(q, fleet)...)
	}
	if q.wants(findBranch) {
		results = append(results, m.findBranches(q, fleet)...)
	}
	if q.wants(findStash) {
		results = append(results, m.findStashes(q)...)
	}
	if q.wants(findNote) {
		results = append(results, m.findNotes(q, fleet)...)
	}

	if len(results) > paletteMaxResults {
		results = results[:paletteMaxResults]
	}

	return results
}

// paletteScopePaths is the repo set the query runs over.
func (m Model) paletteScopePaths(fleet bool) []string {
	if fleet || m.viewMode != ViewModeRepoDetail {
		return m.filteredPaths
	}

	return []string{m.selectedRepo}
}

func (m Model) findRepos(q findQuery, fleet bool) []findResult {
	var results []findResult
	for _, path := range m.paletteScopePaths(fleet) {
		summary, ok := m.summaries[path]
		if !ok || !q.matches(summary.Name(), summary.RemoteRepo, summary.Branch) {
			continue
		}

		results = append(results, findResult{
			kind:   findRepo,
			repo:   path,
			label:  summary.Name(),
			detail: summary.Branch + compactSignalSep + display.RepoStatusSummary(summary.RepoSummary),
		})
	}

	return results
}

func (m Model) findPRs(q findQuery, fleet bool) []findResult {
	var results []findResult
	for _, path := range m.paletteScopePaths(fleet) {
		prs := m.cachedPRs(path)
		for i := range prs {
			pr := &prs[i]
			if q.number > 0 && pr.Number != q.number {
				continue
			}
			if q.number == 0 && !q.matches(pr.Title, pr.HeadRef, strconv.Itoa(pr.Number)) {
				continue
			}

			results = append(results, findResult{
				kind:   findPR,
				repo:   path,
				label:  "#" + strconv.Itoa(pr.Number) + " " + pr.Title,
				detail: filepath.Base(path) + compactSignalSep + display.PRStatusDisplay(*pr),
				branch: pr.HeadRef,
				number: pr.Number,
			})
		}
	}

	return results
}

// cachedPRs collects a repo's known pull requests without fetching: the list
// the PR cache holds, the fleet map's copy while ":prs" is open, and the
// summary's own current-branch PR.
func (m Model) cachedPRs(path string) []forge.PullRequest {
	if path == m.selectedRepo && len(m.prs) > 0 {
		return m.prs
	}
	if data, ok := m.prMap[path]; ok && len(data.PRs) > 0 {
		return data.PRs
	}

	summary := m.summaries[path]
	if prs, ok := github.CachedPRs(path, summary.RemoteID, summary.Upstream); ok && len(prs) > 0 {
		return prs
	}
	if summary.PRInfo != nil {
		return []forge.PullRequest{*summary.PRInfo}
	}

	return nil
}

func (m Model) findBranches(q findQuery, fleet bool) []findResult {
	var results []findResult
	for _, path := range m.paletteScopePaths(fleet) {
		for _, branch := range m.cachedBranches(path) {
			if !q.matches(branch.Name) {
				continue
			}

			results = append(results, findResult{
				kind:   findBranch,
				repo:   path,
				label:  branch.Name,
				detail: filepath.Base(path) + compactSignalSep + branchAheadBehindStatus(branch),
				branch: branch.Name,
			})
		}
	}

	return results
}

// cachedBranches returns the branches known for a repo without running git:
// the full list for the repo whose detail is loaded, and the checked-out
// branch alone for the rest.
func (m Model) cachedBranches(path string) []vcs.BranchInfo {
	if path == m.selectedRepo && len(m.branches) > 0 {
		return m.branches
	}
	if data, ok := m.prMap[path]; ok && len(data.Branches) > 0 {
		return data.Branches
	}

	summary := m.summaries[path]
	if summary.Branch == "" {
		return nil
	}

	return []vcs.BranchInfo{{
		Name: summary.Branch, Upstream: summary.Upstream,
		Ahead: summary.Ahead, Behind: summary.Behind, IsCurrent: true,
	}}
}

// findStashes searches the loaded repo's stashes. Stashes are never cached
// fleet-wide, so this stays repo-scoped whatever the query asks for.
func (m Model) findStashes(q findQuery) []findResult {
	var results []findResult
	for _, stash := range m.stashes {
		if !q.matches(stash.Message) {
			continue
		}

		results = append(results, findResult{
			kind:   findStash,
			repo:   m.selectedRepo,
			label:  "stash@{" + strconv.Itoa(stash.Index) + "} " + stash.Message,
			detail: display.StashRelativeDate(stash),
			index:  stash.Index,
		})
	}

	return results
}

func (m Model) findNotes(q findQuery, fleet bool) []findResult {
	var results []findResult
	for _, path := range m.paletteScopePaths(fleet) {
		if path == m.selectedRepo {
			for i, note := range m.notesFiles {
				if !q.matches(note.Name, note.Content) {
					continue
				}

				results = append(results, findResult{
					kind: findNote, repo: path, label: note.Name,
					detail: firstContentLine(note.Content), index: i,
				})
			}

			continue
		}

		for _, note := range m.summaries[path].NotesFiles {
			if !q.matches(note.Name, note.FirstLine) {
				continue
			}

			results = append(results, findResult{
				kind: findNote, repo: path, label: note.Name,
				detail: filepath.Base(path) + compactSignalSep + note.FirstLine,
			})
		}
	}

	return results
}

// openPalette starts a find, scoped to the fleet unless the focused repo view
// opened it.
func (m Model) openPalette() (tea.Model, tea.Cmd) {
	m.paletteReturnMode = m.viewMode
	m.paletteFleetScope = m.viewMode != ViewModeRepoDetail
	m.paletteCursor = 0
	m.paletteMarks = nil
	m.paletteActions = false
	m.paletteResults = false
	m.paletteInput.SetValue("")
	m.paletteInput.Focus()
	m.viewMode = ViewModePalette

	return m, nil
}

func (m Model) closePalette() (tea.Model, tea.Cmd) {
	m.viewMode = m.paletteReturnMode
	m.paletteActions = false
	m.paletteInput.Blur()

	return m, nil
}

// handlePaletteKey drives the find line. Keys type into the query until focus
// moves to the results, where letters are commands: tab marks, the action
// leader opens the verbs for the whole set, and / or i returns to the query.
func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	results := m.findResults()

	switch {
	case msg.String() == keyCtrlC:
		return m, tea.Quit
	case m.paletteActions:
		return m.handlePaletteActionKey(msg, m.paletteTargets(results))
	case m.paletteResults:
		return m.handlePaletteResultsKey(msg, results)
	}

	return m.handlePaletteInputKey(msg, results)
}

func (m Model) handlePaletteInputKey(msg tea.KeyMsg, results []findResult) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEsc:
		return m.closePalette()

	case keyEnter:
		return m.runPaletteDefault(results)

	case keyTab, keyDown, "ctrl+n":
		return m.focusPaletteResults(), nil

	case "up", "ctrl+p":
		m.paletteCursor = max(m.paletteCursor-1, 0)

		return m, nil
	}

	var cmd tea.Cmd
	m.paletteInput, cmd = m.paletteInput.Update(msg)
	m.paletteCursor = 0

	return m, cmd
}

func (m Model) handlePaletteResultsKey(msg tea.KeyMsg, results []findResult) (tea.Model, tea.Cmd) {
	last := max(len(results)-1, 0)

	switch msg.String() {
	case keyEsc:
		return m.closePalette()

	case "q":
		return m, tea.Quit

	case "?":
		return m.openHelp(), nil

	case "/", "i":
		m.paletteResults = false

		return m, m.paletteInput.Focus()

	case keyTab, markToggleKey:
		return m.togglePaletteMark(results)

	case actionLeader:
		m.paletteActions = len(results) > 0

		return m, nil

	case keyEnter:
		return m.runPaletteDefault(results)

	case "up", "ctrl+p", "k":
		m.paletteCursor = max(m.paletteCursor-1, 0)

	case keyDown, "ctrl+n", "j":
		m.paletteCursor = min(m.paletteCursor+1, last)

	case "home", "g":
		m.paletteCursor = 0

	case "end", "G":
		m.paletteCursor = last
	}

	return m, nil
}

func (m Model) focusPaletteResults() Model {
	m.paletteResults = true
	m.paletteInput.Blur()

	return m
}

// paletteTargets is the set an action applies to: the marked rows, or every
// match when nothing is marked.
func (m Model) paletteTargets(results []findResult) []findResult {
	if len(m.paletteMarks) == 0 {
		return results
	}

	targets := make([]findResult, 0, len(m.paletteMarks))
	for _, r := range results {
		if m.paletteMarks[r.key()] {
			targets = append(targets, r)
		}
	}

	return targets
}

func (m Model) togglePaletteMark(results []findResult) (tea.Model, tea.Cmd) {
	if m.paletteCursor >= len(results) {
		return m, nil
	}

	if m.paletteMarks == nil {
		m.paletteMarks = make(map[string]bool)
	}

	key := results[m.paletteCursor].key()
	if m.paletteMarks[key] {
		delete(m.paletteMarks, key)
	} else {
		m.paletteMarks[key] = true
	}

	m.paletteCursor = min(m.paletteCursor+1, max(len(results)-1, 0))

	return m, nil
}

// runPaletteDefault runs the highlighted row's type-appropriate default: open
// the repo, or open it focused on the panel holding the object.
func (m Model) runPaletteDefault(results []findResult) (tea.Model, tea.Cmd) {
	if m.paletteCursor >= len(results) {
		return m, nil
	}

	result := results[m.paletteCursor]
	m.viewMode = m.paletteReturnMode
	m.paletteInput.Blur()

	opened, cmd := m.openRepo(result.repo)
	next := mustAppModel(opened)
	if panel, ok := panelForKind(result.kind); ok {
		next.focusedPanel = panel
	}

	return next, cmd
}

// panelForKind names the panel an object lives in, so acting on a find result
// lands the cursor where the object is. A repo result has none, and opens where
// its own NEEDS say.
func panelForKind(kind findKind) (panelID, bool) {
	switch kind {
	case findBranch:
		return panelBranches, true
	case findPR:
		return panelPRs, true
	case findStash:
		return panelStashes, true
	case findNote:
		return panelNotes, true
	case findRepo, findAny:
		return panelStatus, false
	}

	return panelStatus, false
}

// mustAppModel unwraps the concrete Model a local transition just returned.
func mustAppModel(model tea.Model) Model {
	if m, ok := model.(Model); ok {
		return m
	}

	return Model{}
}

// paletteAction is one verb the action menu offers for a result set.
type paletteAction struct {
	key  string
	name string
	run  func(Model, []findResult) (Model, tea.Cmd)
}

// paletteActionsFor returns the verbs a result set supports. Every set can
// commit its repos to the selected-repos text object, which is the seam that
// makes batch operators compose with a find.
func paletteActionsFor(kind findKind) []paletteAction {
	actions := []paletteAction{
		{key: "s", name: "select repos", run: selectResultRepos},
		{key: "f", name: "fetch repos", run: taskOverResults(tasks.NameFetch)},
		{key: "c", name: "cleanup merged", run: taskOverResults(tasks.NameCleanup)},
	}

	if kind == findPR {
		actions = append(actions, paletteAction{key: "o", name: "open in browser", run: openResultPRs})
	}

	return actions
}

func selectResultRepos(m Model, results []findResult) (Model, tea.Cmd) {
	paths := resultRepos(results)

	m.selectedPaths = make(map[string]bool, len(paths))
	for _, path := range paths {
		m.selectedPaths[path] = true
	}

	return m, statusCmd("Selected " + strconv.Itoa(len(paths)) + " repos")
}

// taskOverResults runs a built-in task on the repos a result set touches,
// taking the confirmation the task asks for.
func taskOverResults(taskName string) func(Model, []findResult) (Model, tea.Cmd) {
	return func(m Model, results []findResult) (Model, tea.Cmd) {
		paths := resultRepos(results)
		if len(paths) == 0 {
			return m, statusCmd("No repos in the result set")
		}

		return m.runNamed(taskName, taskName, paths)
	}
}

// openResultPRs opens each pull request in the set in a browser, using the URL
// the cached list already carries.
func openResultPRs(m Model, results []findResult) (Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, len(results))
	for _, r := range results {
		if r.kind != findPR {
			continue
		}

		prs := m.cachedPRs(r.repo)
		for i := range prs {
			if prs[i].Number == r.number && prs[i].URL != "" {
				cmds = append(cmds, openURLCmd(prs[i].URL))
			}
		}
	}

	if len(cmds) == 0 {
		return m, statusCmd("No pull request URLs in the result set")
	}

	return m, tea.Batch(cmds...)
}

// handlePaletteActionKey answers the action menu: a verb key runs it against the
// target set, anything else backs out to the query.
func (m Model) handlePaletteActionKey(msg tea.KeyMsg, targets []findResult) (tea.Model, tea.Cmd) {
	m.paletteActions = false

	if msg.String() == keyEsc {
		return m, nil
	}

	for _, action := range m.paletteActionSet(targets) {
		if action.key != msg.String() {
			continue
		}

		m.viewMode = m.paletteReturnMode
		m.paletteInput.Blur()

		return action.run(m, targets)
	}

	return m, nil
}

func (m Model) paletteScreen() screen {
	results := m.findResults()

	return screen{
		title: nameFind,
		facts: m.paletteFacts(len(results)),
		body:  m.renderPalette(results),
		hints: paletteHints(m.paletteActions, m.paletteResults),
	}
}

// renderPalette draws the find line, its matches, and a preview of the
// highlighted one.
func (m Model) renderPalette(results []findResult) string {
	width := contentWidth(m.viewWidth())

	var b strings.Builder
	b.WriteString(m.paletteInput.View())
	b.WriteString("\n\n")

	b.WriteString(m.renderPaletteResults(results, width))
	b.WriteString("\n\n")

	if m.paletteActions {
		b.WriteString(m.renderPaletteActions(m.paletteTargets(results)))
	} else {
		b.WriteString(m.renderPalettePreview(results, width))
	}

	return b.String()
}

func (m Model) paletteFacts(matches int) []frame.Fact {
	scope := "this repo"
	if m.paletteFleetScope || parseFindQuery(m.paletteInput.Value()).fleet {
		scope = "all repos"
	}

	facts := []frame.Fact{
		{Text: "scope: " + scope},
		{Text: strconv.Itoa(matches) + " " + plural(matches, "match", "matches")},
	}

	if marks := len(m.paletteMarks); marks > 0 {
		facts = append(facts, frame.Fact{Text: strconv.Itoa(marks) + " marked", Tone: frame.Accent})
	}

	return facts
}

// paletteVisibleRows is how many matches the list shows before it scrolls.
const paletteVisibleRows = 10

func (m Model) renderPaletteResults(results []findResult, width int) string {
	if len(results) == 0 {
		return styles.Skin.Subtitle.Render("no matches")
	}

	window := visibleRange(m.paletteCursor, len(results), paletteVisibleRows)

	lines := make([]string, 0, window.end-window.start)
	for i := window.start; i < window.end; i++ {
		r := results[i]
		selected := i == m.paletteCursor

		mark := " "
		if m.paletteMarks[r.key()] {
			mark = "✓"
		}

		text := rowStyleFor(selected).Render(mark + " " +
			table.Pad(r.kindName(), paletteKindColWidth, table.AlignLeft) + " " +
			table.Pad(r.label, max(width/paletteLabelShare, paletteKindColWidth), table.AlignLeft) + " ")
		room := max(width-gutterWidth-lipgloss.Width(text), 0)
		detail := withSelection(styles.Skin.Subtitle, selected).Render(table.Truncate(r.detail, room))

		lines = append(lines, rowCursorFor(selected)+text+detail)
	}

	return strings.Join(lines, "\n")
}

// paletteKindColWidth fits the longest kind name ("branch"), and the label
// takes this fraction of the frame before the detail column starts.
const (
	paletteKindColWidth = 6
	paletteLabelShare   = 2
)

func (m Model) renderPalettePreview(results []findResult, width int) string {
	if m.paletteCursor >= len(results) {
		return ""
	}

	r := results[m.paletteCursor]
	lines := []string{
		detailField("kind", r.kindName()),
		detailField("repo", filepath.Base(r.repo)),
	}
	if r.branch != "" {
		lines = append(lines, detailField(nameBranch, r.branch))
	}
	if r.detail != "" {
		lines = append(lines, detailField("state", truncate(r.detail, width)))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderPaletteActions(targets []findResult) string {
	label := "act on " + strconv.Itoa(len(targets)) + " " + plural(len(targets), "result", "results")
	actions := m.paletteActionSet(targets)

	lines := make([]string, 0, len(actions)+1)
	lines = append(lines, styles.Skin.Subtitle.Bold(true).Render(label))
	for _, action := range actions {
		lines = append(lines, styles.Skin.Key.Render(action.key)+styles.Skin.Subtitle.Render(" "+action.name))
	}

	return strings.Join(lines, "\n")
}

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func paletteHints(actions, results bool) []keyhint.Hint {
	switch {
	case actions:
		return []keyhint.Hint{
			{Key: "key", What: nameRun, Priority: 2},
			{Key: keyEsc, What: descBack, Priority: 1},
		}
	case results:
		return []keyhint.Hint{
			{Key: keyEnter, What: descOpen, Priority: 5},
			{Key: keyTab, What: "mark", Priority: 3},
			{Key: actionLeader, What: "act on set", Priority: 2},
			{Key: "/", What: "edit query", Priority: 4},
			{Key: keyNavPair, What: descNav, Priority: 1},
			{Key: keyEsc, What: descClose, Priority: 6},
		}
	}

	return []keyhint.Hint{
		{Key: keyEnter, What: descOpen, Priority: 5},
		{Key: keyTab + "/↓", What: "results", Priority: 3},
		{Key: "*", What: "widen to fleet", Priority: 1},
		{Key: keyEsc, What: descClose, Priority: 4},
	}
}
