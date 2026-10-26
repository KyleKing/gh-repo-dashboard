package app

import (
	"context"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/tui/table"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// From previewMinWidth columns the repo list stops at tableMaxWidth and the
// rest of the terminal previews the repo under the cursor.
const (
	previewMinWidth    = 140
	tableMaxWidth      = 100
	previewCommitLimit = 5
)

//nolint:mnd // the numbers are this table's geometry, not constants reused elsewhere
var previewCommitSpecs = []table.Column{
	{Key: colCommitHash, Title: colCommitHash, Min: 7},
	{Key: colCommitDate, Title: colCommitDate, Min: 8},
	{Key: colCommitSubject, Title: colCommitSubject, Min: 16, Weight: 1},
}

func previewShown(termWidth int) bool {
	return termWidth >= previewMinWidth
}

// repoTableWidth is the width the repo table renders into.
func repoTableWidth(termWidth int) int {
	if previewShown(termWidth) {
		return tableMaxWidth
	}

	return listWidth(termWidth)
}

// PreviewCommitsLoadedMsg carries the newest commits of the repo the preview
// shows. A failed read arrives empty, so it is not asked for again.
type PreviewCommitsLoadedMsg struct {
	Path    string
	Commits []vcs.CommitInfo
}

// loadPreviewCommitsCmd reads the local commit log, which costs no network call.
func loadPreviewCommitsCmd(path string) tea.Cmd {
	return func() tea.Msg {
		//nolint:errcheck // a repo with no readable log previews without commits
		commits, _ := vcs.GetOperations(path).GetCommitLog(context.Background(), path, previewCommitLimit)

		return PreviewCommitsLoadedMsg{Path: path, Commits: commits}
	}
}

func (m Model) previewPath() (string, bool) {
	if !previewShown(m.width) || m.cursor >= len(m.filteredPaths) {
		return "", false
	}

	return m.filteredPaths[m.cursor], true
}

// requestPreview asks for the commits the preview still lacks. It is the only
// read the preview adds: pull requests and CI are what the list already holds.
func (m *Model) requestPreview() tea.Cmd {
	path, ok := m.previewPath()
	if !ok || m.summaryPending(path) {
		return nil
	}

	if _, cached := m.previewCommits[path]; cached || m.fetchPending(path, fetchPreviewLog) {
		return nil
	}

	m.startFetch(path, fetchPreviewLog)

	return loadPreviewCommitsCmd(path)
}

func (m Model) withPreview(body string) string {
	height := m.bodyHeight()
	rightWidth := m.width - tableMaxWidth - splitRuleWidth

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, tableMaxWidth, "")
	}

	left := padLines(lines, height, tableMaxWidth)
	right := padLines(m.renderRepoPreview(rightWidth, height), height, rightWidth)
	rule := styles.Skin.Hairline.Render(splitRuleGlyph)

	out := make([]string, height)
	for i := range out {
		out[i] = left[i] + rule + right[i]
	}

	return strings.Join(out, "\n")
}

// renderRepoPreview draws the focused view's own sections for the repo under the
// cursor, through a copy of the model pointed at it.
func (m Model) renderRepoPreview(width, height int) []string {
	path, ok := m.previewPath()
	if !ok {
		return nil
	}

	summary := m.rowSummary(path)
	pending := m.summaryPending(path)

	identity := section(pending, summary.VCSType.String()+compactSignalSep+summary.Branch)

	lines := []string{
		styles.Skin.Heading.Render(truncate(qualifiedRepoName(path), width)),
		styles.Skin.Subtitle.Render(truncate(identity, width)),
		"",
		sectionRule("NEEDS", width),
	}

	lines = append(lines, m.previewNeeds(summary, width)...)

	view := m
	view.selectedRepo = path
	view.detailLoading = true
	view.focusedPanel = panelStatus

	panels := []panelContent{
		view.previewEmptyNote(path, view.prsPanel(summary, width), fetchOpenPRs),
		view.previewEmptyNote(path, view.ciPanel(summary, width), fetchCI),
		view.previewCommitsPanel(path, width),
	}

	heights := distributePanelHeights(panels, -1, height-len(lines))

	for i := range panels {
		lines = append(lines, panelRule(&panels[i], strconv.Itoa(panels[i].count), false, width))
		lines = append(lines, panelRows(panels[i], heights[i]-panelRuleHeight, 0, false)...)
	}

	return lines
}

func (m Model) previewNeeds(summary models.RepoSummary, width int) []string {
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

// previewEmptyNote tells an empty section from one still loading.
func (m Model) previewEmptyNote(path string, p panelContent, kind fetchKind) panelContent {
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

func (m Model) previewCommitsPanel(path string, width int) panelContent {
	commits, loaded := m.previewCommits[path]
	layout := fitPanelCols(previewCommitSpecs, width)
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

	p := panelContent{heading: "COMMITS", count: len(commits), relevance: relevanceIdle, rows: rows, selectable: true}
	if !loaded {
		p.rows = []string{styles.Skin.Muted.Render(readingLabel)}
	}

	return p
}
