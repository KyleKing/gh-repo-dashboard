package app

import (
	"context"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/tui/preview"
	"github.com/kyleking/aragonite/tui/split"
	"github.com/kyleking/aragonite/tui/table"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// The detail region sits beside the table from detailSideMinWidth columns,
// where the table stops at tableMaxWidth, and below it from standardMinWidth
// once opened. Narrower terminals have none, and enter opens the focused view.
const (
	detailSideMinWidth = 140
	tableMaxWidth      = 100
	detailMinHeight    = 15
	detailBelowPercent = 60
	detailCommitLimit  = 5
	changedPathsShown  = 3
)

var detailSpec = split.Spec{
	SideMin:      detailSideMinWidth,
	MainMax:      tableMaxWidth,
	MinWidth:     standardMinWidth,
	MinHeight:    detailMinHeight,
	BelowPercent: detailBelowPercent,
	Pad:          1,
}

//nolint:mnd // the numbers are this table's geometry, not constants reused elsewhere
var detailCommitSpecs = []table.Column{
	{Key: colCommitHash, Title: colCommitHash, Min: 7},
	{Key: colCommitDate, Title: colCommitDate, Min: 8},
	{Key: colCommitSubject, Title: colCommitSubject, Min: 16, Weight: 1},
}

// repoTableWidth is the width the repo table renders into.
func repoTableWidth(termWidth int) int {
	if termWidth >= detailSpec.SideMin {
		return tableMaxWidth
	}

	return listWidth(termWidth)
}

func (m Model) detailPlacement() split.Placement {
	return detailSpec.Place(m.width, m.bodyHeight(), m.detailOpen)
}

// detailToggleable reports whether v does anything at this width: below the
// side-by-side width the region is opened by hand, and nowhere else.
func (m Model) detailToggleable() bool {
	return m.width >= detailSpec.MinWidth && m.width < detailSpec.SideMin
}

// mainHeight is how many lines the chips, search, and table share: the whole
// body unless the detail region sits below them.
func (m Model) mainHeight() int {
	height := m.bodyHeight()
	if m.detailPlacement() != split.Below {
		return height
	}

	return height - detailSpec.DetailHeight(split.Below, height)
}

func (m Model) detailPath() (string, bool) {
	if m.detailPlacement() == split.None || m.cursor >= len(m.filteredPaths) {
		return "", false
	}

	return m.filteredPaths[m.cursor], true
}

// CommitLogLoadedMsg carries the newest commits of a repo the detail region
// shows. A failed read arrives empty, so it is not asked for again.
type CommitLogLoadedMsg struct {
	Path    string
	Commits []vcs.CommitInfo
}

// loadCommitLogCmd reads the local commit log, which costs no network call.
func loadCommitLogCmd(path string) tea.Cmd {
	return func() tea.Msg {
		//nolint:errcheck // a repo with no readable log shows no commits
		commits, _ := vcs.GetOperations(path).GetCommitLog(context.Background(), path, detailCommitLimit)

		return CommitLogLoadedMsg{Path: path, Commits: commits}
	}
}

// requestDetail asks for what the detail region still lacks for the repo under
// the cursor, once per repo: its branches, notes, uncommitted paths, and commit log.
func (m *Model) requestDetail() tea.Cmd {
	path, ok := m.detailPath()
	if !ok {
		return nil
	}

	summary, loaded := m.summaries[path]
	if !loaded {
		return nil
	}

	var cmds []tea.Cmd

	if _, cached := m.prMap[path]; !cached && !m.fetchPending(path, fetchPRMap) {
		m.startFetch(path, fetchPRMap)
		cmds = append(cmds, loadPRMapCmd(path, summary.RemoteID, summary.Upstream))
	}

	if _, cached := m.commitLog[path]; !cached && !m.fetchPending(path, fetchCommitLog) {
		m.startFetch(path, fetchCommitLog)
		cmds = append(cmds, loadCommitLogCmd(path))
	}

	if _, cached := m.notesPreview[path]; !cached && len(summary.NotesFiles) > 0 && !m.fetchPending(path, fetchNotes) {
		m.startFetch(path, fetchNotes)
		cmds = append(cmds, loadNotesContentCmd(path, summary.NotesFiles))
	}

	if _, cached := m.changedPaths[path]; !cached && summary.UncommittedCount() > 0 &&
		!m.fetchPending(path, fetchChangedPaths) {
		m.startFetch(path, fetchChangedPaths)
		cmds = append(cmds, loadChangedPathsCmd(path, summary.VCSType))
	}

	return tea.Batch(cmds...)
}

func splitStyles() split.Styles {
	return split.Styles{Rule: styles.Skin.Hairline}
}

// renderDetail draws everything the list knows about the repo under the cursor:
// identity, what needs attention, the state behind it, then pull requests, CI,
// commits, and notes sharing what height is left.
func (m Model) renderDetail(width, height int) []string {
	path, ok := m.detailPath()
	if !ok {
		return nil
	}

	summary := m.rowSummary(path)
	identity := section(m.summaryPending(path), summary.VCSType.String()+compactSignalSep+summary.Branch)

	label := truncate(m.repoLabel(path), width)
	lines := []string{
		styles.Skin.Heading.Render(label) +
			styles.Skin.Subtitle.Render(truncate(compactSignalSep+identity, max(width-lipgloss.Width(label), 0))),
		preview.LabelRule(previewStyles, "NEEDS", width),
	}

	lines = append(lines, m.detailNeeds(summary, width)...)
	lines = append(lines, m.detailFacts(path, summary, width)...)

	view := m
	view.selectedRepo = path
	view.detailLoading = true
	view.focusedPanel = panelStatus

	panels := []panelContent{
		view.detailEmptyNote(path, view.prsPanel(summary, width), fetchOpenPRs),
		view.detailEmptyNote(path, view.ciPanel(summary, width), fetchCI),
		view.detailCommitsPanel(path, width),
	}

	if notes, ok := m.detailNotesPanel(path, summary, width); ok {
		panels = append(panels, notes)
	}

	sections := make([]preview.Section, len(panels))
	for i, p := range panels {
		sections[i] = p.previewSection()
	}

	return append(lines, preview.Column(previewStyles, sections, -1, 0,
		height-len(lines), width, 1)...)
}

func (m Model) detailNeeds(summary models.RepoSummary, width int) []string {
	if m.summaryPending(summary.Path) {
		return []string{styles.Skin.Muted.Render(readingLabel)}
	}

	tokens := m.needsOf(summary).tokens()
	if len(tokens) == 0 {
		return []string{styles.Skin.Muted.Render("nothing needs you")}
	}

	lines := make([]string, 0, len(tokens))
	for _, token := range tokens {
		lines = append(lines, token.tone.style().Render(truncate(token.words, width)))
	}

	return lines
}

type detailFact struct {
	label string
	value string
}

const detailFactLabelWidth = 8

// detailFacts are the state lines the sections below do not carry: what changed
// in the working tree, where the template stands, and which branches hold
// unpushed work. A fact with nothing to say is left out.
func (m Model) detailFacts(path string, summary models.RepoSummary, width int) []string {
	data, loaded := m.prMap[path]

	candidates := []detailFact{
		{"Changed", m.detailChanged(path, summary)},
		{"Template", detailTemplate(summary.TemplateInfo)},
		{tabNameBranches, section(!loaded, detailBranches(data.Branches))},
	}

	var lines []string

	for _, fact := range candidates {
		if fact.value == "" || fact.value == emDash {
			continue
		}

		label := styles.Skin.Subtitle.Render(padCell(fact.label, detailFactLabelWidth))
		lines = append(lines, label+" "+styles.Skin.Body.Render(truncate(fact.value, width-detailFactLabelWidth-1)))
	}

	return lines
}

func (m Model) detailChanged(path string, summary models.RepoSummary) string {
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

func detailTemplate(info *models.CopierTemplateInfo) string {
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

// detailBranches counts the repo's local branches and names the ones holding
// commits their remote does not have, which are the branches with work on them.
// The default branch is left out: it is where the work lands, not work itself.
func detailBranches(branches []vcs.BranchInfo) string {
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

// detailEmptyNote tells an empty section from one still loading.
func (m Model) detailEmptyNote(path string, p panelContent, kind fetchKind) panelContent {
	if len(p.rows) > 0 {
		return p
	}

	note := "none"
	if m.fetchPending(path, kind) || m.summaryPending(path) {
		note = readingLabel
	}

	p.rows = []string{styles.Skin.Muted.Render(note)}

	return p
}

func (m Model) detailCommitsPanel(path string, width int) panelContent {
	commits, loaded := m.commitLog[path]
	layout := fitPanelCols(detailCommitSpecs, width)
	subtle := map[string]lipgloss.Style{
		colCommitHash: styles.Skin.Subtitle,
		colCommitDate: styles.Skin.Subtitle,
	}

	rows := make([]string, 0, len(commits))

	for _, commit := range commits {
		values := map[string]string{
			colCommitHash:    commit.ShortHash,
			colCommitDate:    display.RelativeTimeCompact(commit.Date),
			colCommitSubject: commit.Subject,
		}
		cells := renderCells(layout, values, subtle, &plainStyle)
		rows = append(rows, detailRow(rowCursorFor(false), layout, cells, plainStyle))
	}

	p := panelContent{
		heading:    "COMMITS",
		count:      strconv.Itoa(len(commits)),
		relevance:  preview.Idle,
		rows:       rows,
		selectable: true,
	}
	if !loaded {
		p.rows = []string{styles.Skin.Muted.Render(readingLabel)}
	}

	return p
}

// detailNotesPanel is the repo's notes read in full, elided in the middle when
// they outgrow their share so the newest entries at a note's end survive. A
// repo with no notes file has no panel.
func (m Model) detailNotesPanel(path string, summary models.RepoSummary, width int) (panelContent, bool) {
	if len(summary.NotesFiles) == 0 {
		return panelContent{}, false
	}

	p := panelContent{
		heading: "NOTES", count: strconv.Itoa(len(summary.NotesFiles)),
		relevance: preview.Present, selectable: true, elide: true,
	}

	contents, loaded := m.notesPreview[path]
	if !loaded {
		p.rows = []string{styles.Skin.Muted.Render(readingLabel)}

		return p, true
	}

	p.count = strconv.Itoa(len(contents))

	for i := range contents {
		if len(contents) > 1 {
			p.rows = append(p.rows, sectionRule(contents[i].Name, width))
		}

		p.rows = append(p.rows, notesBodyLines(contents[i].Content, width)...)
	}

	return p, true
}
