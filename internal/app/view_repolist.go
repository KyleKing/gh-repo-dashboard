package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/region"
	"github.com/kyleking/aragonite/tui/table"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// searchChromeHeight is what the docked search spends above the table: the
// input, its scope legend, and a blank line.
const searchChromeHeight = 3

// searchScopeLegend spells out every scope prefix "/" search accepts, short
// key and long name together, so the grammar is visible the moment the
// search box opens rather than living only behind "?" help.
func searchScopeLegend() string {
	return styles.Skin.Subtitle.Render(
		"[r]epo:  [b]ranch:  [p]r:  [t]emplate:  [n]otes:  [c]ommit:",
	)
}

func (m Model) repoListScreen() screen {
	cached := m.withNeedsCache()

	return screen{
		title: appTitle,
		facts: cached.repoListFacts(),
		body:  cached.renderRepoList(),
		hints: cached.listHints(),
	}
}

// renderRepoList is the list's body: the docked search, then the table and its
// expanded region.
func (m Model) renderRepoList() string {
	var b strings.Builder

	if m.searching {
		b.WriteString(m.searchInput.View())
		b.WriteString("\n")
		b.WriteString(searchScopeLegend())
		b.WriteString("\n\n")
	}

	b.WriteString(m.renderListBody())

	if previewShown(m.width) {
		return m.withPreview(b.String())
	}

	return b.String()
}

// listBodyHeight is how many lines the table and the expanded region share:
// the body less the docked search and the chips line.
func (m Model) listBodyHeight() int {
	height := m.bodyHeight() - m.chipsHeight()

	if m.searching {
		height -= searchChromeHeight
	}

	return max(height, 1)
}

func (m Model) repoListFacts() []frame.Fact {
	listable := len(m.listableRepos())

	repoCount := fmt.Sprintf("%d repos", len(m.filteredPaths))
	if len(m.filteredPaths) != listable {
		repoCount = fmt.Sprintf("%d of %d repos", len(m.filteredPaths), listable)
	}

	counts := m.attentionCounts()

	facts := []frame.Fact{{Text: repoCount}}
	facts = appendCount(facts, m.DirtyCount(), "dirty", "dirty", frame.Warning)
	facts = appendCount(facts, counts.failing, "failing CI", "failing CI", frame.Error)
	facts = appendCount(facts, counts.turn, "your turn", "your turn", frame.Accent)
	facts = appendCount(facts, counts.drift, "behind template", "behind template", frame.Muted)
	facts = appendCount(facts, counts.due, "task due", "tasks due", frame.Muted)

	if progress := m.loadingBadge(); progress != "" {
		facts = append(facts, frame.Fact{Text: progress, Tone: frame.Accent})
	}

	return append(facts, m.narrowingFacts()...)
}

func appendCount(facts []frame.Fact, count int, one, many string, tone frame.Tone) []frame.Fact {
	if count == 0 {
		return facts
	}

	label := many
	if count == 1 {
		label = one
	}

	return append(facts, frame.Fact{Text: strconv.Itoa(count) + " " + label, Tone: tone})
}

// narrowingFacts names the marks, and the sort where the compact layout has no
// column titles to carry it. The filters, predicate, and search are chips above
// the table.
func (m Model) narrowingFacts() []frame.Fact {
	var facts []frame.Fact

	if summary := m.markSummary(); summary != "" {
		facts = append(facts, frame.Fact{Text: summary, Tone: frame.Accent})
	}

	if m.isCompact() {
		facts = append(facts, m.sortFacts()...)
	}

	return facts
}

// sortFacts names the enabled sorts in priority order, for the compact layout
// that has no column titles to carry the mark.
func (m Model) sortFacts() []frame.Fact {
	enabled := m.enabledSorts()
	facts := make([]frame.Fact, 0, len(enabled))

	for _, s := range enabled {
		if s.Mode != models.SortModeAttention {
			facts = append(facts, frame.Fact{Text: s.Mode.String() + " " + sortArrow(s.Direction)})
		}
	}

	return facts
}

// enabledSorts lists the enabled sorts, highest priority first.
func (m Model) enabledSorts() []models.ActiveSort {
	var enabled []models.ActiveSort

	for _, s := range m.activeSorts {
		if s.IsEnabled() {
			enabled = append(enabled, s)
		}
	}

	slices.SortFunc(enabled, func(a, b models.ActiveSort) int { return a.Priority - b.Priority })

	return enabled
}

func sortArrow(d models.SortDirection) string {
	if d == models.SortDirectionAsc {
		return "▴"
	}

	return "▾"
}

// sortColumnKey is the column a sort mode orders by.
func sortColumnKey(mode models.SortMode) string {
	switch mode {
	case models.SortModeName:
		return colName
	case models.SortModeModified:
		return colModified
	case models.SortModeStatus:
		return colNeeds
	case models.SortModeBranch:
		return colBranch
	case models.SortModeAttention:
		return ""
	}

	return ""
}

// markSort returns layout with the arrow of the highest-priority sort on its
// column's title, or layout itself when that column is hidden or no sort is on.
func (m Model) markSort(layout table.Layout) table.Layout {
	enabled := m.enabledSorts()
	if len(enabled) == 0 {
		return layout
	}

	key := sortColumnKey(enabled[0].Mode)
	marked := layout
	marked.Columns = slices.Clone(layout.Columns)

	for i, c := range marked.Columns {
		if c.Key == key {
			marked.Columns[i].Title += " " + sortArrow(enabled[0].Direction)
		}
	}

	return marked
}

// loadingBadge reports what is still arriving, or is empty once the fleet has
// settled. Discovery counts against a denominator it knows, and the per-repo
// fetches it starts count down instead: CI is only asked for the rows on
// screen, so their total is not known in advance and a combined fraction would
// have to move its own denominator as the user scrolls.
func (m Model) loadingBadge() string {
	if m.loading {
		return fmt.Sprintf("Loading %d/%d", m.loadedCount, m.loadingCount)
	}

	if inFlight := m.fetchesInFlight(); inFlight > 0 {
		return fmt.Sprintf("Fetching %d", inFlight)
	}

	return ""
}

func (m Model) renderListBody() string {
	below := m.expandedLines()
	list := fitBlock(m.renderTable(), m.listBodyHeight()-len(below))

	if len(below) > 0 {
		list += "\n" + strings.Join(below, "\n")
	}

	if chips := m.chipsLine(); chips != "" {
		return chips + "\n" + list
	}

	return list
}

// fitBlock pads or truncates a block of lines to exactly height lines.
func fitBlock(block string, height int) string {
	lines := strings.Split(block, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// rowSummary is the summary a list row renders from. A row exists from
// discovery, before its own summary has been read, and the zero summary carries
// no path: standing the path back in keeps the row named and lets every cell
// tell a value still being fetched from one that is genuinely absent.
func (m Model) rowSummary(path string) models.RepoSummary {
	summary, loaded := m.summaries[path]
	if !loaded {
		summary.Path = path
	}

	return summary
}

func (m Model) renderTable() string {
	if len(m.filteredPaths) == 0 {
		if m.loading {
			return m.loadingPlaceholder("Discovering repositories")
		}
		if len(m.repoPaths) > 0 {
			return m.emptyPlaceholder("No repositories match the active filters",
				"Press f to change filters, / to clear the search.")
		}

		return m.emptyPlaceholder("No repositories found",
			"Nothing was discovered under the configured scan paths.")
	}

	compact := m.isCompact()
	width := repoTableWidth(m.width)

	var (
		rows      []string
		rowHeight = 1
	)

	layout := layoutRepoCols(width)
	if compact {
		layout = table.Fit(compactColSpecs, width-gutterWidth)
		rowHeight = compactRowHeight
	} else {
		rows = append(rows, styles.Skin.Subtitle.Bold(true).Render(renderRepoHeader(m.markSort(layout))))
	}

	window := m.visibleRepoRange(rowHeight)
	groups := m.attentionGroups()

	for i := window.start; i < window.end; i++ {
		path := m.filteredPaths[i]
		summary := m.rowSummary(path)
		n := m.needsOf(summary)

		if groups.grouped && (i == window.start || groups.promoted[i] != groups.promoted[i-1]) {
			rows = append(rows, groups.rule(groups.promoted[i], width))
		}

		if compact {
			rows = append(rows, m.renderCompactRow(summary, n, i == m.cursor, layout))
		} else {
			rows = append(rows, m.renderTableRow(summary, n, i == m.cursor, layout))
		}
	}

	return strings.Join(rows, "\n")
}

const (
	expandPercent = 60
	expandMinRows = 3
	listMinRows   = 6
)

func (m Model) expandHeight(body int) int {
	if !m.expandOpen {
		return 0
	}

	headerRows := 1
	if m.isCompact() {
		headerRows = 0
	}

	room := body - headerRows - listMinRows
	if room < expandMinRows {
		return 0
	}

	return min(body*expandPercent/percentDenominator, room)
}

func (m Model) expandedLines() []string {
	limit := m.expandHeight(m.listBodyHeight())
	if limit == 0 {
		return nil
	}

	return m.expandLines(repoTableWidth(m.width), limit)
}

// tableHeight is how many of the body's lines the table keeps once the expanded
// region has taken its share.
func (m Model) tableHeight() int {
	return m.listBodyHeight() - len(m.expandedLines())
}

// percentDenominator scales the percentage budgets above; elisionEnds is how
// many ends of a list survive a middle elision.
const (
	percentDenominator = 100
	elisionEnds        = 2
)

const groupRuleCount = 2

// visibleRepoRange returns the half-open slice of filteredPaths that fits the
// window, keeping the cursor near the middle. The rowHeight argument is how
// many terminal lines one record covers, so the compact layout scrolls by
// records rather than by lines.
func (m Model) visibleRepoRange(rowHeight int) repoWindow {
	availableLines := m.tableHeight()
	if !m.isCompact() {
		availableLines--
	}

	total := len(m.filteredPaths)

	if !m.groupedByAttention() {
		return visibleRange(m.cursor, total, availableLines/rowHeight)
	}

	window := visibleRange(m.cursor, total, (availableLines-1)/rowHeight)
	if m.straddlesGroups(window) {
		window = visibleRange(m.cursor, total, (availableLines-groupRuleCount)/rowHeight)
	}

	return window
}

// visibleRange returns the half-open slice of a list of the given length that
// fits visible rows, keeping the cursor near the middle.
func visibleRange(cursor, length, visible int) repoWindow {
	visible = max(visible, 1)

	start := max(cursor-visible/visibleWindowCenter, 0)

	end := start + visible
	if end > length {
		end = length
		start = max(end-visible, 0)
	}

	return repoWindow{start: start, end: end}
}

type repoWindow struct {
	start int
	end   int
}

func (m Model) expandLines(width, height int) []string {
	if m.cursor >= len(m.filteredPaths) {
		return nil
	}

	path := m.filteredPaths[m.cursor]
	summary := m.rowSummary(path)
	notes := m.expandNotes(path, summary, width)

	block := region.Region{
		Title: qualifiedRepoName(path) + compactSignalSep +
			section(m.summaryPending(path), overviewIdentity(summary)),
		Head: m.expandFacts(path, summary),
	}

	if len(summary.NotesFiles) > 0 {
		block.Section = "notes" + compactSignalSep + notes.caption
		block.Body = notes.lines
	}

	return block.Lines(regionStyles(), width, height)
}

func (m Model) expandFacts(path string, summary models.RepoSummary) []region.Fact {
	data, loaded := m.prMap[path]
	peersPending := m.loading || !loaded || m.fetchPending(path, fetchPeerBranches)

	candidates := []region.Fact{
		{Label: "Changed", Value: m.expandChanged(path, summary)},
		{Label: "CI", Value: m.expandCI(summary)},
		{Label: "Template", Value: expandTemplate(summary.TemplateInfo)},
		{Label: tabNamePeers, Value: section(peersPending, overviewRelevantPeers(m.relevantPeers(path)))},
		{Label: tabNameBranches, Value: section(!loaded, expandBranches(data.Branches))},
		{
			Label: tabNamePRs,
			Value: section(len(summary.OpenPRs) == 0 && m.fetchPending(path, fetchOpenPRs), expandPRs(summary.OpenPRs)),
		},
	}

	facts := make([]region.Fact, 0, len(candidates))

	for _, fact := range candidates {
		if fact.Value != "" && fact.Value != emDash {
			facts = append(facts, fact)
		}
	}

	return facts
}

const changedPathsShown = 3

func (m Model) expandChanged(path string, summary models.RepoSummary) string {
	if summary.UncommittedCount() == 0 {
		return ""
	}

	counts := workToken(summary.Staged+summary.Unstaged+summary.Conflicted, summary.Untracked)

	paths, loaded := m.changedPaths[path]
	if !loaded {
		return counts + compactSignalSep + readingLabel
	}

	shown := paths[:min(len(paths), changedPathsShown)]
	text := counts + compactSignalSep + strings.Join(shown, ", ")

	if more := len(paths) - len(shown); more > 0 {
		text += " +" + strconv.Itoa(more)
	}

	return text
}

func (m Model) expandCI(summary models.RepoSummary) string {
	if summary.CI == nil {
		return section(m.fetchPending(summary.Path, fetchCI), "")
	}

	runs := slices.Clone(summary.CI.Workflows)
	slices.SortStableFunc(runs, func(a, b forge.CIWorkflowRun) int { return ciOrder(a) - ciOrder(b) })

	parts := make([]string, 0, len(runs))
	for i := range runs {
		parts = append(parts, ciWorkflowText(runs[i]))
	}

	return strings.Join(parts, "  ")
}

func ciOrder(run forge.CIWorkflowRun) int {
	switch {
	case run.Conclusion == models.CIFailureConclusion:
		return 0
	case run.Status != forge.StatusCompleted:
		return 1
	default:
		return 2 //nolint:mnd // third tier: everything that finished without failing
	}
}

func ciWorkflowText(run forge.CIWorkflowRun) string {
	switch {
	case run.Conclusion == models.CIFailureConclusion && len(run.FailingJobs) > 0:
		return "✗ " + run.Workflow + " (" + strings.Join(run.FailingJobs, ", ") + ")"
	case run.Conclusion == models.CIFailureConclusion:
		return "✗ " + run.Workflow
	case run.Status != forge.StatusCompleted:
		return "… " + run.Workflow
	case run.Conclusion == "success":
		return "✓ " + run.Workflow
	default:
		return "⊘ " + run.Workflow
	}
}

func expandTemplate(info *models.CopierTemplateInfo) string {
	switch {
	case info == nil:
		return ""
	case !info.IsTag:
		return info.Commit + " (pinned)"
	case info.Behind && info.LatestTag != "":
		return info.Commit + " → " + info.LatestTag
	default:
		return info.Commit + " (current)"
	}
}

// expandBranches counts the repo's local branches and names the ones holding
// commits their remote does not have, which are the branches with work on them.
// The default branch is left out: it is where the work lands, not work itself.
func expandBranches(branches []vcs.BranchInfo) string {
	if len(branches) == 0 {
		return emDash
	}

	defaultBranch := findDefaultBranch(branches)

	names := make([]string, 0, len(branches))
	for _, branch := range branches {
		if branch.Ahead == 0 || branch.Name == defaultBranch {
			continue
		}

		names = append(names, branch.Name+" ↑"+strconv.Itoa(branch.Ahead))
	}

	count := strconv.Itoa(len(branches)) + " local"
	if len(names) == 0 {
		return count
	}

	return count + compactSignalSep + strings.Join(names, ", ")
}

// expandPRs counts the repo's open pull requests and names them newest first.
func expandPRs(prs []forge.PullRequest) string {
	if len(prs) == 0 {
		return emDash
	}

	titles := make([]string, 0, len(prs))
	for i := range prs {
		titles = append(titles, "#"+strconv.Itoa(prs[i].Number)+" "+prs[i].Title)
	}

	return strconv.Itoa(len(prs)) + " open" + compactSignalSep + strings.Join(titles, ", ")
}

// notesBlock is the region's notes section: the note text itself, and the
// caption the closing divider carries, which is what names a note when nothing
// else on screen does.
type notesBlock struct {
	lines   []string
	caption string
}

// expandNotes renders the selected repo's notes at the foot of the region.
func (m Model) expandNotes(path string, summary models.RepoSummary, width int) notesBlock {
	if len(summary.NotesFiles) == 0 {
		return notesBlock{}
	}

	contents, loaded := m.notesPreview[path]
	if !loaded {
		return notesBlock{caption: readingLabel}
	}

	// One note needs no heading of its own: the divider already names the
	// file. Several do, or their text runs together.
	if len(contents) == 1 {
		return notesBlock{lines: notesBodyLines(contents[0].Content, width), caption: contents[0].Name}
	}

	lines := make([]string, 0, len(contents))
	for i := range contents {
		lines = append(lines, sectionRule(contents[i].Name, width))
		lines = append(lines, notesBodyLines(contents[i].Content, width)...)
	}

	return notesBlock{lines: lines, caption: strconv.Itoa(len(contents)) + " files"}
}

// qualifiedRepoName names a repo by its parent directory as well as its own,
// which is what tells two checkouts of the same project apart.
func qualifiedRepoName(path string) string {
	return filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path))
}

// sectionRule draws a labeled rule across width.
func sectionRule(label string, width int) string {
	rule := strings.Repeat("─", max(width-lipgloss.Width(label)-sectionRuleLead-sectionRuleSpaces, 0))

	return styles.Skin.Subtitle.Render(strings.Repeat("─", sectionRuleLead)+" ") +
		styles.Skin.Heading.Render(label) +
		styles.Skin.Subtitle.Render(" "+rule)
}

const (
	sectionRuleLead   = 2
	sectionRuleSpaces = 2
)

// notesBodyLines styles a note's text one line per source line, cutting the
// middle of any line too wide for the region.
func notesBodyLines(content string, width int) []string {
	body := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(body) == 1 && strings.TrimSpace(body[0]) == "" {
		return []string{styles.Skin.Subtitle.Render("  (empty)")}
	}

	lines := make([]string, len(body))
	for i, line := range body {
		style := styles.Skin.Muted
		if strings.HasPrefix(strings.TrimSpace(line), "!") {
			style = styles.Skin.Warning
		}

		lines[i] = style.Render(table.TruncateMiddle(strings.TrimRight(line, " \t"), width))
	}

	return lines
}

// elideMiddle drops the middle of lines until it fits height, marking the cut,
// so the newest entries at the end of a note survive alongside the oldest.
func elideMiddle(lines []string, height int) []string {
	if len(lines) <= height || height < 1 {
		return lines
	}

	kept := height - 1
	head := kept / elisionEnds
	tail := kept - head

	out := make([]string, 0, height)
	out = append(out, lines[:head]...)
	out = append(out, styles.Skin.Subtitle.Render(
		"  ⋯ "+strconv.Itoa(len(lines)-head-tail)+" more lines ⋯",
	))

	return append(out, lines[len(lines)-tail:]...)
}

// formatPRCell formats a repo's PR-column text: "#N" with a review-status
// indicator and a CI/workflow failure indicator, or emDash if there's no PR.
func formatPRCell(s models.RepoSummary) string {
	if s.PRInfo == nil {
		return emDash
	}

	prNum := fmt.Sprintf("#%d", s.PRInfo.Number)

	switch display.PRReviewStatus(*s.PRInfo) {
	case forge.ReviewApproved:
		prNum += " ✓"
	case forge.ReviewChangesRequested:
		prNum += " ✗"
	}

	if s.PRInfo.Checks.Total > 0 && display.ChecksSummary(s.PRInfo.Checks) == forge.StatusFailing {
		prNum += " ⚠"
	}

	return prNum
}

// warnSuffix marks a template ref whose currency cannot be judged.
const warnSuffix = " ⚠"

// formatCopierCell formats a repo's copier-template column: emDash if the
// repo isn't copier-generated, the installed tag (with "→ latest" appended
// when behind), or the installed ref plus a warning icon when it isn't a
// semver tag at all (commit- or branch-pinned, so currency can't be judged).
func formatCopierCell(s models.RepoSummary, width int) string {
	info := s.TemplateInfo
	if info == nil {
		return emDash
	}

	if !info.IsTag {
		return truncate(info.Commit, width-lipgloss.Width(warnSuffix)) + warnSuffix
	}

	if info.Behind && info.LatestTag != "" {
		return info.Commit + " → " + info.LatestTag
	}

	return info.Commit
}

func renderRepoHeader(layout table.Layout) string {
	return strings.Repeat(" ", gutterWidth) + table.Header(layout)
}

// ciCell renders the default branch's CI rollup as a single word: a check
// when every workflow passed, the count of failures when not, the pending glyph
// while the fetch is in flight, and emDash once it is known there is nothing
// to report. Used where a line of text has room for one signal, not a table
// column; the expanded region names each workflow.
func (m Model) ciCell(s models.RepoSummary) string {
	if s.CI == nil {
		return absentCell(m.fetchPending(s.Path, fetchCI) || m.summaryPending(s.Path))
	}

	failing, total := s.CIFailing()

	switch {
	case total == 0:
		return emDash
	case failing > 0:
		return "✗ " + strconv.Itoa(failing) + "/" + strconv.Itoa(total)
	case s.CIRunning() > 0:
		return "…" + strconv.Itoa(s.CIRunning())
	default:
		return "✓"
	}
}

// conflictMark flags a repo whose branch is checked out somewhere else too.
const conflictMark = "⚠"

// ownCheckoutOf adapts a summary into the checkout form the conflict check
// compares its peers against.
func ownCheckoutOf(summary *models.RepoSummary) *models.PeerCheckout {
	own := models.OwnCheckout(summary)

	return &own
}

// refreshShared groups summaries by remote once, since checking each repo against every other makes a render quadratic.
func (m *Model) refreshShared() {
	shared := make(map[string]bool)

	for _, group := range m.summariesByRemote() {
		if len(group) < minSharedRemote {
			continue
		}

		for i := range group {
			peers := make([]models.PeerCheckout, 0, len(group)-1)

			for j := range group {
				if j != i {
					peers = append(peers, models.OwnCheckout(&group[j]))
				}
			}

			if models.ConflictingBranches(ownCheckoutOf(&group[i]), peers)[group[i].Branch] {
				shared[group[i].Path] = true
			}
		}
	}

	m.shared = shared
}

const minSharedRemote = 2

func (m Model) renderTableRow(s models.RepoSummary, n needs, selected bool, layout table.Layout) string {
	base := rowStyleFor(selected)

	cells := make([]string, 0, len(layout.Columns))
	for _, c := range layout.Columns {
		cells = append(cells, m.repoCell(c.Key, s, n, layout.Width(c.Key), base, selected))
	}

	return rowGutter(m.isMarked(s.Path), selected) + paintRow(layout, cells, base)
}

// repoCell renders one column of a repo row, padded to exactly width cells so
// the columns after it stay aligned. A repo that needs nothing draws its name
// Muted, so the quiet rows recede.
func (m Model) repoCell(
	col string, s models.RepoSummary, n needs, width int, base lipgloss.Style, selected bool,
) string {
	switch col {
	case colName:
		return nameStyle(n, base, selected).Render(padCell(s.Name(), width))
	case colBranch:
		return withSelection(m.branchStyle(s), selected).Render(padCell(m.branchCell(s, width), width))
	case colNeeds:
		if m.summaryPending(s.Path) {
			return base.Render(padCell(pendingGlyph, width))
		}

		return m.renderNeeds(n, width, base, selected)
	case colPR:
		return m.prsCell(n, width, base, selected)
	case colModified:
		if m.summaryPending(s.Path) {
			return base.Render(padCell(pendingGlyph, width))
		}

		age := padCell(display.RepoRelativeModified(s.RepoSummary), width)

		return withSelection(styles.Skin.Muted, selected).Render(age)
	default:
		return base.Render(padCell("", width))
	}
}

func nameStyle(n needs, base lipgloss.Style, selected bool) lipgloss.Style {
	if n.promotes() {
		return base
	}

	return withSelection(styles.Skin.Muted, selected)
}

// branchStyle draws a branch name in the body face, and quietly when it is the
// default branch, which is where a repo is expected to be.
func (m Model) branchStyle(s models.RepoSummary) lipgloss.Style {
	if m.ciBranch[s.Path] == s.Branch || vcs.IsDefaultBranchName(s.Branch) {
		return styles.Skin.Muted
	}

	return styles.Skin.Body
}

// branchCell renders the checked-out branch name plus, once the repo's local
// branch count has loaded, a "+N" for the other branches it holds, so the
// list's BRANCH column carries the same count a separate BRs column used to.
// The name truncates first, reserving room for the suffix, so a long branch
// name never crowds out the count the way trimming the combined string from
// the tail would.
func (m Model) branchCell(s models.RepoSummary, width int) string {
	suffix := ""
	if count, ok := m.branchCount[s.Path]; ok && count > 1 {
		suffix = " +" + strconv.Itoa(count-1)
	}

	name := table.Truncate(s.Branch, max(width-lipgloss.Width(suffix), 0))

	return name + suffix
}

// regionStyles are the faces the expandable regions draw with, kept in one
// place so the Repos and PRs tabs cannot drift apart.
func regionStyles() region.Styles {
	return region.Styles{Rule: styles.Skin.Subtitle, Label: styles.Skin.Heading}
}
