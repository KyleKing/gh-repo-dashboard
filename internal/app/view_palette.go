package app

import (
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/forge/github"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/ui"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
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
			detail: summary.Branch + compactSignalSep + ui.RepoStatusSummary(summary.RepoSummary),
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
				detail: filepath.Base(path) + compactSignalSep + ui.PRStatusDisplay(*pr),
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
			detail: ui.StashRelativeDate(stash),
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

// handlePaletteKey drives the find line: typing requeries, tab marks, enter
// acts on the highlighted row, and the action leader opens the verbs for the
// whole set.
func (m Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	results := m.findResults()

	if m.paletteActions {
		return m.handlePaletteActionKey(msg, m.paletteTargets(results))
	}

	switch msg.String() {
	case keyEsc:
		return m.closePalette()

	case keyTab:
		return m.togglePaletteMark(results)

	case actionLeader:
		if len(results) > 0 {
			m.paletteActions = true
		}

		return m, nil

	case keyEnter:
		return m.runPaletteDefault(results)

	case "up", "ctrl+p":
		m.paletteCursor = max(m.paletteCursor-1, 0)
		return m, nil

	case "down", "ctrl+n":
		m.paletteCursor = min(m.paletteCursor+1, max(len(results)-1, 0))
		return m, nil
	}

	var cmd tea.Cmd
	m.paletteInput, cmd = m.paletteInput.Update(msg)
	m.paletteCursor = 0

	return m, cmd
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
	next.focusedPanel = panelForKind(result.kind)

	return next, cmd
}

// panelForKind names the panel an object lives in, so acting on a find result
// lands the cursor where the object is.
func panelForKind(kind findKind) panelID {
	switch kind {
	// A pull request has no panel of its own, and its head branch is where the
	// work it names actually lives.
	case findBranch, findPR:
		return panelBranches
	case findStash:
		return panelStashes
	case findNote:
		return panelNotes
	case findRepo, findAny:
		return panelStatus
	}

	return panelStatus
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
		{key: "f", name: "fetch repos", run: batchOverResults(taskFetchAll, false, batchFetchAllCmd)},
		{key: "c", name: "cleanup merged", run: batchOverResults(taskCleanupMerged, true, batchCleanupMergedCmd)},
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

// batchOverResults runs a batch task on the repos a result set touches, taking
// the same confirmation the operator keys take when it deletes anything.
func batchOverResults(
	taskName string, destructive bool, taskCmd func([]string) tea.Cmd,
) func(Model, []findResult) (Model, tea.Cmd) {
	return func(m Model, results []findResult) (Model, tea.Cmd) {
		paths := resultRepos(results)
		if len(paths) == 0 {
			return m, statusCmd("No repos in the result set")
		}

		return m.confirmBatchTask(taskName, destructive, paths, taskCmd)
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

	for _, action := range paletteActionsFor(homogeneousKind(targets)) {
		if action.key != msg.String() {
			continue
		}

		m.viewMode = m.paletteReturnMode
		m.paletteInput.Blur()

		return action.run(m, targets)
	}

	return m, nil
}

// renderPalette draws the find line, its matches, and a preview of the
// highlighted one.
func (m Model) renderPalette() string {
	width := contentWidth(m.width)
	results := m.findResults()

	var b strings.Builder
	b.WriteString(styles.TitleStyle.Render(nameFind))
	b.WriteString(" ")
	b.WriteString(m.paletteInput.View())
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render(m.paletteScopeLine(len(results))))
	b.WriteString("\n\n")

	b.WriteString(m.renderPaletteResults(results, width))
	b.WriteString("\n\n")

	if m.paletteActions {
		b.WriteString(renderPaletteActions(m.paletteTargets(results)))
	} else {
		b.WriteString(m.renderPalettePreview(results, width))
	}

	b.WriteString("\n")
	b.WriteString(styles.FooterStyle.Render(paletteFooter(m.paletteActions)))

	return b.String()
}

func (m Model) paletteScopeLine(matches int) string {
	scope := scopeThisRepo
	if m.paletteFleetScope || parseFindQuery(m.paletteInput.Value()).fleet {
		scope = "all repos"
	}

	line := "scope: " + scope + compactSignalSep + strconv.Itoa(matches) + " " + plural(matches, "match", "matches")
	if marks := len(m.paletteMarks); marks > 0 {
		line += compactSignalSep + strconv.Itoa(marks) + " marked"
	}

	return line
}

// paletteVisibleRows is how many matches the list shows before it scrolls.
const paletteVisibleRows = 10

func (m Model) renderPaletteResults(results []findResult, width int) string {
	if len(results) == 0 {
		return styles.SubtitleStyle.Render("no matches")
	}

	window := visibleRange(m.paletteCursor, len(results), paletteVisibleRows)

	lines := make([]string, 0, window.end-window.start)
	for i := window.start; i < window.end; i++ {
		r := results[i]
		style := rowStyleFor(i == m.paletteCursor)

		mark := " "
		if m.paletteMarks[r.key()] {
			mark = "✓"
		}

		row := rowCursorFor(i == m.paletteCursor) + mark + " " +
			table.Pad(r.kindName(), paletteKindColWidth, table.AlignLeft) + " " +
			table.Pad(r.label, max(width/paletteLabelShare, paletteKindColWidth), table.AlignLeft) + " " +
			styles.SubtitleStyle.Render(r.detail)

		lines = append(lines, style.Render(table.Truncate(row, width)))
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

func renderPaletteActions(targets []findResult) string {
	label := "act on " + strconv.Itoa(len(targets)) + " " + plural(len(targets), "result", "results")
	actions := paletteActionsFor(homogeneousKind(targets))

	lines := make([]string, 0, len(actions)+1)
	lines = append(lines, styles.HeaderStyle.Render(label))
	for _, action := range actions {
		lines = append(lines, styles.FooterKeyStyle.Render(action.key)+styles.FooterDescStyle.Render(" "+action.name))
	}

	return strings.Join(lines, "\n")
}

func paletteFooter(actions bool) string {
	if actions {
		return styles.FooterKeyStyle.Render("key") + styles.FooterDescStyle.Render(" run  ") +
			styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" back")
	}

	hints := [][2]string{
		{keyEnter, "open"},
		{keyTab, "mark"},
		{actionLeader, "act on set"},
		{"*", "widen to fleet"},
		{keyEsc, "close"},
	}

	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, styles.FooterKeyStyle.Render(h[0])+styles.FooterDescStyle.Render(" "+h[1]))
	}

	return strings.Join(parts, "  ")
}
