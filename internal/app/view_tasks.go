package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

const (
	colTaskName    = "TASK"
	colTaskDesc    = "DESC"
	colTaskWhere   = "WHERE"
	colTaskDue     = "DUE"
	colTaskLast    = "LAST RUN"
	colPreviewRepo = "REPO"
	colPreviewBr   = "BRANCH"
	colPreviewLast = "LAST RUN"
	colPreviewDue  = "DUE"

	neverRan = "never"
)

//nolint:mnd // the numbers are each table's geometry, not constants reused elsewhere
var (
	taskColSpecs = []table.Column{
		{Key: colTaskName, Title: colTaskName, Min: 12, Weight: 1},
		{Key: colTaskDesc, Title: colTaskDesc, Min: 20, Weight: 3, Priority: 2},
		{Key: colTaskWhere, Title: colTaskWhere, Min: 16, Weight: 2, Priority: 1},
		{Key: colTaskDue, Title: colTaskDue, Min: 3},
		{Key: colTaskLast, Title: colTaskLast, Min: 22, Weight: 1},
	}

	previewColSpecs = []table.Column{
		{Key: colPreviewRepo, Title: colPreviewRepo, Min: 16, Weight: 2},
		{Key: colPreviewBr, Title: colPreviewBr, Min: 14, Weight: 1, Priority: 1},
		{Key: colPreviewLast, Title: colPreviewLast, Min: 14, Priority: 2},
		{Key: colPreviewDue, Title: colPreviewDue, Min: 3},
	}
)

// recordSummary reads a run's outcome as counts, leaving out the zeros.
func recordSummary(rec tasks.Record) string {
	parts := make([]string, 0, runOutcomes)
	for _, part := range []struct {
		count int
		word  string
	}{{rec.OK, "ok"}, {rec.Failed, "failed"}, {rec.Canceled, "canceled"}} {
		if part.count > 0 {
			parts = append(parts, strconv.Itoa(part.count)+" "+part.word)
		}
	}

	return strings.Join(parts, " ")
}

const (
	runOutcomes   = 3
	historyChrome = 2
)

func (m Model) tasksScreen() screen {
	list := m.taskList()

	due := 0
	for _, task := range list {
		due += m.dueCount(task.Name)
	}

	facts := []frame.Fact{{Text: strconv.Itoa(len(list)) + " " + plural(len(list), "task", "tasks")}}
	if due > 0 {
		facts = append(facts, frame.Fact{Text: strconv.Itoa(due) + " due", Tone: frame.Accent})
	}

	return screen{title: appTitle, facts: facts, body: m.renderTasks(), hints: tasksHints()}
}

func (m Model) renderTasks() string {
	list := m.taskList()
	layout := fitDetailCols(taskColSpecs, listWidth(m.width))

	var history []tasks.Record
	if m.tasksCursor < len(list) {
		history = m.taskHistory[list[m.tasksCursor].Name]
	}
	histRows := min(len(history), max(m.bodyHeight()/halfPageDivisor-historyChrome, 0))
	histRoom := 0
	if histRows > 0 {
		histRoom = histRows + historyChrome
	}

	rows := []string{detailHeader(layout)}
	window := visibleRange(m.tasksCursor, len(list), max(m.bodyHeight()-1-histRoom, 1))
	for i := window.start; i < window.end; i++ {
		rows = append(rows, m.taskRow(layout, list[i], i == m.tasksCursor))
	}

	if histRows > 0 {
		rows = append(rows, "", sectionRule("RUNS"+compactSignalSep+list[m.tasksCursor].Name, listWidth(m.width)))
		for _, rec := range history[:histRows] {
			rows = append(rows, historyLine(rec))
		}
	}

	return strings.Join(rows, "\n")
}

func historyLine(rec tasks.Record) string {
	face := styles.Skin.Subtitle
	if rec.Failed > 0 {
		face = styles.Skin.Error
	}

	return strings.Repeat(" ", gutterWidth) +
		styles.Skin.Subtitle.Render(display.RelativeTime(rec.At)+"  ") +
		face.Render(recordSummary(rec)) +
		styles.Skin.Subtitle.Render("  "+rec.Duration.Round(time.Second).String())
}

func (m Model) taskRow(layout table.Layout, task config.Task, selected bool) string {
	style := rowStyleFor(selected)
	values := map[string]string{
		colTaskName:  task.Name,
		colTaskDesc:  task.Desc,
		colTaskWhere: orDash(task.Where),
		colTaskDue:   emDash,
		colTaskLast:  neverRan,
	}
	cellStyles := map[string]lipgloss.Style{
		colTaskDesc:  withSelection(styles.Skin.Subtitle, selected),
		colTaskWhere: withSelection(styles.Skin.Subtitle, selected),
		colTaskLast:  withSelection(styles.Skin.Subtitle, selected),
		colTaskDue:   withSelection(styles.Skin.Subtitle, selected),
	}

	if due := m.dueCount(task.Name); due > 0 {
		values[colTaskDue] = strconv.Itoa(due)
		cellStyles[colTaskDue] = withSelection(styles.Skin.Accent, selected)
	}

	if history := m.taskHistory[task.Name]; len(history) > 0 {
		last := history[0]
		values[colTaskLast] = display.RelativeTime(last.At) + compactSignalSep + recordSummary(last)
		if last.Failed > 0 {
			cellStyles[colTaskLast] = withSelection(styles.Skin.Error, selected)
		}
	}

	cells := renderCells(layout, values, cellStyles, &style)

	return detailRow(rowCursorFor(selected), layout, cells, style)
}

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func tasksHints() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 4},
		{Key: keyEnter, What: "preview", Priority: 8},
		{Key: tabKeyRepos + "/" + keyTab, What: "repos", Priority: 6},
		{Key: ":", What: "command", Priority: 3},
		{Key: "?", What: nameHelp, Priority: 7},
		{Key: "q", What: nameQuit, Priority: 5},
	}
}

func (m Model) previewScreen() screen {
	rows := len(m.preview.rows)
	facts := []frame.Fact{
		{Text: strconv.Itoa(rows) + " " + plural(rows, "repo", "repos")},
		{Text: m.preview.source},
	}
	if m.preview.task.Confirm {
		facts = append(facts, frame.Fact{Text: "asks first", Tone: frame.Warning})
	}

	return screen{
		title: m.preview.task.Name,
		facts: facts,
		body:  m.renderPreview(),
		hints: previewHints(m.preview.task.Confirm),
	}
}

const previewHeaderRows = 3

func (m Model) renderPreview() string {
	task := m.preview.task
	command := task.Run
	if task.Func != nil {
		command = "built-in"
	}

	lines := []string{
		styles.Skin.Body.Render(task.Desc),
		detailField("runs", truncate(command, max(contentWidth(m.width)-previewHeaderRows-1, 1))),
		"",
	}

	if len(m.preview.rows) == 0 {
		lines = append(lines, styles.Skin.Subtitle.Render("No repos match "+m.preview.source+"."))

		return strings.Join(lines, "\n")
	}

	layout := fitDetailCols(previewColSpecs, listWidth(m.width))
	lines = append(lines, detailHeader(layout))

	window := visibleRange(m.preview.cursor, len(m.preview.rows), max(m.bodyHeight()-previewHeaderRows-1, 1))
	for i := window.start; i < window.end; i++ {
		row := m.preview.rows[i]
		selected := i == m.preview.cursor
		style := rowStyleFor(selected)

		last := neverRan
		if row.ran {
			last = display.RelativeTime(row.last)
		}
		values := map[string]string{
			colPreviewRepo: filepath.Base(row.path),
			colPreviewBr:   m.summaries[row.path].Branch,
			colPreviewLast: last,
			colPreviewDue:  emDash,
		}
		cellStyles := map[string]lipgloss.Style{
			colPreviewBr:   withSelection(styles.Skin.Subtitle, selected),
			colPreviewLast: withSelection(styles.Skin.Subtitle, selected),
			colPreviewDue:  withSelection(styles.Skin.Subtitle, selected),
		}
		if row.due {
			values[colPreviewDue] = "due"
			cellStyles[colPreviewDue] = withSelection(styles.Skin.Accent, selected)
		}

		cells := renderCells(layout, values, cellStyles, &style)
		lines = append(lines, detailRow(rowCursorFor(selected), layout, cells, style))
	}

	return strings.Join(lines, "\n")
}

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func previewHints(confirm bool) []keyhint.Hint {
	run := keyhint.Hint{Key: keyEnter, What: nameRun, Priority: 8}
	if confirm {
		run.What = "run (asks first)"
	}

	return []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 4},
		run,
		{Key: keyEsc, What: descBack, Priority: 9},
		{Key: "?", What: nameHelp, Priority: 6},
		{Key: "q", What: nameQuit, Priority: 5},
	}
}

func (m Model) logScreen() screen {
	log := m.runLog

	return screen{
		title: m.run.label + compactSignalSep + log.repo,
		facts: []frame.Fact{{Text: strconv.Itoa(len(log.lines)) + " " + plural(len(log.lines), "line", "lines")}},
		body:  m.renderLog(),
		hints: logHints(),
	}
}

func (m Model) renderLog() string {
	log := m.runLog
	switch {
	case log.loading:
		return styles.Skin.Subtitle.Render("Reading the log...")
	case log.err != nil:
		return styles.Skin.Error.Render(log.err.Error())
	case len(log.lines) == 1 && log.lines[0] == "":
		return styles.Skin.Subtitle.Render("The run wrote nothing.")
	}

	width := max(contentWidth(m.width), 1)
	lines := make([]string, 0, m.bodyHeight())
	for _, line := range log.lines[min(log.at, len(log.lines)):] {
		for _, wrapped := range strings.Split(ansi.Hardwrap(line, width, true), "\n") {
			lines = append(lines, styles.Skin.Body.Render(wrapped))
		}
		if len(lines) >= m.bodyHeight() {
			break
		}
	}

	return strings.Join(lines, "\n")
}

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func logHints() []keyhint.Hint {
	return []keyhint.Hint{
		{Key: keyNavPair, What: descScroll, Priority: 4},
		{Key: keysEnds, What: "ends", Priority: 2},
		{Key: keyEsc, What: descBack, Priority: 9},
		{Key: "?", What: nameHelp, Priority: 5},
	}
}
