package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

const (
	colRunRepo   = "REPO"
	colRunStatus = "STATUS"
	colRunExit   = "EXIT"
	colRunTime   = "TIME"
	colRunNote   = "NOTE"

	statusRunning = "running"
)

//nolint:mnd // the numbers are this table's geometry, not constants reused elsewhere
var runColSpecs = []table.Column{
	{Key: colRunRepo, Title: colRunRepo, Min: 16, Weight: 2},
	{Key: colRunStatus, Title: colRunStatus, Min: 8},
	{Key: colRunExit, Title: colRunExit, Min: 4, Priority: 2},
	{Key: colRunTime, Title: colRunTime, Min: 7, Priority: 3},
	{Key: colRunNote, Title: colRunNote, Min: 20, Weight: 3, Priority: 1},
}

func (m Model) runScreen() screen {
	run := m.run
	facts := []frame.Fact{{Text: strconv.Itoa(run.finished()) + "/" + strconv.Itoa(len(run.rows)) + " done"}}

	if failed := run.count(tasks.StatusFailed); failed > 0 {
		facts = append(facts, frame.Fact{Text: strconv.Itoa(failed) + " failed", Tone: frame.Error})
	}
	if run.canceling {
		facts = append(facts, frame.Fact{Text: "canceling", Tone: frame.Warning})
	}

	return screen{title: run.label, facts: facts, body: m.renderRun(), hints: runHints(run.running)}
}

func (m Model) renderRun() string {
	layout := fitDetailCols(runColSpecs, listWidth(m.width))
	lines := []string{detailHeader(layout)}

	window := visibleRange(m.run.cursor, len(m.run.rows), max(m.bodyHeight()-1, 1))
	for i := window.start; i < window.end; i++ {
		lines = append(lines, renderRunRow(layout, m.run.rows[i], i == m.run.cursor))
	}

	return strings.Join(lines, "\n")
}

func renderRunRow(layout table.Layout, row runRow, selected bool) string {
	style := rowStyleFor(selected)
	values := map[string]string{
		colRunRepo: filepath.Base(row.path), colRunStatus: statusRunning,
		colRunExit: emDash, colRunTime: emDash,
	}
	cellStyles := map[string]lipgloss.Style{
		colRunStatus: withSelection(styles.Skin.Accent, selected),
		colRunExit:   withSelection(styles.Skin.Subtitle, selected),
		colRunTime:   withSelection(styles.Skin.Subtitle, selected),
		colRunNote:   withSelection(styles.Skin.Subtitle, selected),
	}

	if row.done {
		result := row.result
		values[colRunStatus] = string(result.Status)
		values[colRunNote] = result.Note()
		if result.Status != tasks.StatusCanceled {
			values[colRunExit] = strconv.Itoa(result.ExitCode)
			values[colRunTime] = result.Duration.Round(time.Millisecond).String()
		}
		cellStyles[colRunStatus] = withSelection(statusFace(result.Status), selected)
	}

	cells := renderCells(layout, values, cellStyles, &style)

	return detailRow(rowCursorFor(selected), layout, cells, style)
}

func statusFace(status tasks.Status) lipgloss.Style {
	switch status {
	case tasks.StatusOK:
		return styles.Skin.Success
	case tasks.StatusFailed:
		return styles.Skin.Error
	case tasks.StatusCanceled:
		return styles.Skin.Muted
	}

	return styles.Skin.Accent
}

//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func runHints(running bool) []keyhint.Hint {
	hints := []keyhint.Hint{
		{Key: keyNavPair, What: descNav, Priority: 3},
		{Key: keyEnter, What: "log", Priority: 8},
		{Key: "?", What: nameHelp, Priority: 5},
	}

	if running {
		return append(hints, keyhint.Hint{Key: keyCtrlC, What: "cancel", Priority: 9})
	}

	return append(hints,
		keyhint.Hint{Key: keyEsc, What: descBack, Priority: 9},
		keyhint.Hint{Key: "q", What: nameQuit, Priority: 4})
}
