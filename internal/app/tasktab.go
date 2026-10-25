package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

const (
	maxLogBytes     = 1 << 20
	logTabWidth     = "    "
	sourceMarked    = "marked rows"
	sourceEveryRepo = "every repo"
)

type runLog struct {
	repo    string
	lines   []string
	at      int
	loading bool
	err     error
}

type previewRow struct {
	path string
	last time.Time
	ran  bool
	due  bool
}

type taskPreview struct {
	task       config.Task
	source     string
	rows       []previewRow
	cursor     int
	returnMode ViewMode
}

// RunLogLoadedMsg carries a repo's log for the run's log pane.
type RunLogLoadedMsg struct {
	ID   int
	Repo string
	Text string
	Err  error
}

// TaskHistoryLoadedMsg carries each task's recorded runs, newest first.
type TaskHistoryLoadedMsg struct {
	History map[string][]tasks.Record
}

// dueCount is how many repos have the task due, as the NEEDS column counts them.
func (m Model) dueCount(name string) int {
	count := 0
	for _, due := range m.due {
		if slices.Contains(due, name) {
			count++
		}
	}

	return count
}

func (m Model) selector() tasks.Selector {
	return tasks.Selector{Store: m.taskStore(), Tasks: m.taskList()}
}

func loadTaskHistoryCmd(store *tasks.Store, list []config.Task) tea.Cmd {
	return func() tea.Msg {
		history := make(map[string][]tasks.Record, len(list))
		for _, task := range list {
			history[task.Name] = store.History(task.Name)
		}

		return TaskHistoryLoadedMsg{History: history}
	}
}

func loadRunLogCmd(id int, repo, path string) tea.Cmd {
	return func() tea.Msg {
		data, err := readTail(path, maxLogBytes)

		return RunLogLoadedMsg{ID: id, Repo: repo, Text: data, Err: err}
	}
}

func readTail(path string, limit int64) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the path is a log the engine wrote
	if err != nil {
		return "", fmt.Errorf("opening log: %w", err)
	}
	defer file.Close() //nolint:errcheck // read-only

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("reading log: %w", err)
	}
	if info.Size() > limit {
		if _, err := file.Seek(info.Size()-limit, 0); err != nil {
			return "", fmt.Errorf("reading log: %w", err)
		}
	}

	buf := make([]byte, min(info.Size(), limit))
	n, err := file.Read(buf)
	if err != nil && n == 0 && info.Size() > 0 {
		return "", fmt.Errorf("reading log: %w", err)
	}

	return string(buf[:n]), nil
}

func (m Model) handleRunLogLoaded(msg RunLogLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.ID != m.run.id || m.runLog.repo != msg.Repo {
		return m, nil
	}

	text := strings.ReplaceAll(ansi.Strip(msg.Text), "\t", logTabWidth)
	m.runLog.lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	m.runLog.loading, m.runLog.err = false, msg.Err

	return m, nil
}

func (m Model) switchTab(tab Tab) (tea.Model, tea.Cmd) {
	if tab == TabTasks {
		return m.openTasks()
	}

	m.viewMode = ViewModeRepoList

	return m, nil
}

func (m Model) openTasks() (tea.Model, tea.Cmd) {
	m.viewMode = ViewModeTasks

	return m, loadTaskHistoryCmd(m.taskStore(), m.taskList())
}

// openTasksOnDue opens the Tasks tab with the cursor on the first task due for
// the repo under the list's cursor, which is what its NEEDS token names.
func (m Model) openTasksOnDue() (tea.Model, tea.Cmd) {
	return m.withTasksCursor(m.dueTaskIndex()).openTasks()
}

// withTasksCursor moves the task cursor to at, and leaves it where it is when at is negative.
func (m Model) withTasksCursor(at int) Model {
	if at >= 0 {
		m.tasksCursor = at
	}

	return m
}

// dueTaskIndex is the list position of the first task due on the repo under the
// list's cursor, or -1 when none is.
func (m Model) dueTaskIndex() int {
	if m.cursor >= len(m.filteredPaths) {
		return -1
	}

	due := m.due[m.filteredPaths[m.cursor]]
	if len(due) == 0 {
		return -1
	}

	return slices.IndexFunc(m.taskList(), func(t config.Task) bool { return t.Name == due[0] })
}

// taskTargets is what a task runs on when started from the Tasks tab: the
// marked rows when there are any, else every repo its where selects.
func (m Model) taskTargets(task config.Task) ([]string, string, error) {
	if marked := m.markedPaths(); len(marked) > 0 {
		return marked, sourceMarked, nil
	}

	where, err := m.selector().Where(task, "")
	if err != nil {
		return nil, "", fmt.Errorf("selecting repos: %w", err)
	}

	var paths []string
	for _, path := range m.listableRepos() {
		if summary, ok := m.summaries[path]; ok && where(summary) {
			paths = append(paths, path)
		}
	}

	source := sourceEveryRepo
	if task.Where != "" {
		source = "where " + task.Where
	}

	return paths, source, nil
}

func (m Model) openPreview(task config.Task) (tea.Model, tea.Cmd) {
	paths, source, err := m.taskTargets(task)
	if err != nil {
		return m, statusErrCmd(err.Error())
	}

	sel := m.selector()
	rows := make([]previewRow, len(paths))
	for i, path := range paths {
		stamp, ran := sel.Store.LastStamp(task.Name, path)
		rows[i] = previewRow{path: path, last: stamp.At, ran: ran, due: sel.Stale(task, path)}
	}

	m.preview = taskPreview{task: task, source: source, rows: rows, returnMode: m.viewMode}
	m.viewMode = ViewModeTaskPreview

	return m, nil
}

func (m Model) previewPaths() []string {
	paths := make([]string, len(m.preview.rows))
	for i, row := range m.preview.rows {
		paths[i] = row.path
	}

	return paths
}

func runCommand() Command {
	return Command{
		Name:        nameRun,
		Description: "Run a task over its where, or the marked rows: :run <task>",
		Complete: func(m Model, args []string) []string {
			if len(args) > 1 {
				return nil
			}

			prefix := ""
			if len(args) == 1 {
				prefix = args[0]
			}

			var names []string
			for _, task := range m.taskList() {
				if strings.HasPrefix(task.Name, prefix) {
					names = append(names, task.Name)
				}
			}

			return names
		},
		Describe: func(m Model, candidate string) string {
			task, _ := m.findTask(candidate)

			return task.Desc
		},
		Run: runTaskCommand,
	}
}

func runTaskCommand(m Model, args []string) (Model, tea.Cmd) {
	if len(args) != 1 {
		return m, statusErrCmd("Usage: :run <task>")
	}

	task, ok := m.findTask(args[0])
	if !ok {
		return m, statusErrCmd("Unknown task: " + args[0])
	}

	if !m.headless {
		next, cmd := m.openPreview(task)

		return mustAppModel(next), cmd
	}

	return runHeadless(m, task)
}

func runHeadless(m Model, task config.Task) (Model, tea.Cmd) {
	if task.Confirm {
		return m, statusErrCmd(fmt.Sprintf("%s asks first; run it with: run %s --yes", task.Name, task.Name))
	}

	paths := m.markedPaths()
	if len(paths) == 0 {
		var err error
		if paths, err = tasks.Select(context.Background(), m.selector(), task, "", false, m.repoPaths); err != nil {
			return m, statusErrCmd(err.Error())
		}
	}
	if len(paths) == 0 {
		return m, statusCmd("No repos match")
	}

	return m.startRun(task, task.Name, paths)
}

func (m Model) handleTasksKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.taskList()

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		return m.openHelp(), nil
	case key.Matches(msg, m.keys.Up):
		m.tasksCursor = max(m.tasksCursor-1, 0)
	case key.Matches(msg, m.keys.Down):
		m.tasksCursor = min(m.tasksCursor+1, len(list)-1)
	case key.Matches(msg, m.keys.Top):
		m.tasksCursor = 0
	case key.Matches(msg, m.keys.Bottom):
		m.tasksCursor = len(list) - 1
	case key.Matches(msg, m.keys.Enter):
		if m.tasksCursor < len(list) {
			return m.openPreview(list[m.tasksCursor])
		}
	case key.Matches(msg, m.keys.Tab), key.Matches(msg, m.keys.Back), msg.String() == tabKeyRepos:
		return m.switchTab(TabRepos)
	}

	return m, nil
}

func (m Model) handlePreviewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := len(m.preview.rows)

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		return m.openHelp(), nil
	case key.Matches(msg, m.keys.Back):
		m.viewMode = m.preview.returnMode
	case key.Matches(msg, m.keys.Up):
		m.preview.cursor = max(m.preview.cursor-1, 0)
	case key.Matches(msg, m.keys.Down):
		m.preview.cursor = min(m.preview.cursor+1, max(rows-1, 0))
	case key.Matches(msg, m.keys.Top):
		m.preview.cursor = 0
	case key.Matches(msg, m.keys.Bottom):
		m.preview.cursor = max(rows-1, 0)
	case key.Matches(msg, m.keys.Enter):
		return m.runPreview()
	}

	return m, nil
}

func (m Model) runPreview() (tea.Model, tea.Cmd) {
	if len(m.preview.rows) == 0 {
		return m, statusCmd("No repos to run on")
	}

	paths := m.previewPaths()
	if m.preview.source == sourceMarked {
		m, _ = m.clearMarks()
	}

	next, cmd := m.confirmAndRun(m.preview.task, m.preview.task.Name, paths)

	return next, cmd
}

func (m Model) openRunLog() (tea.Model, tea.Cmd) {
	if m.run.cursor >= len(m.run.rows) {
		return m, nil
	}

	row := m.run.rows[m.run.cursor]
	if !row.done || row.result.LogPath == "" {
		return m, statusCmd("No log yet for " + filepath.Base(row.path))
	}

	repo := filepath.Base(row.path)
	m.runLog = runLog{repo: repo, loading: true}
	m.viewMode = ViewModeTaskLog

	return m, loadRunLogCmd(m.run.id, repo, row.result.LogPath)
}

func (m Model) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	last := max(len(m.runLog.lines)-1, 0)
	page := max(m.bodyHeight()-1, 1)

	switch {
	case msg.String() == keyCtrlC && m.run.running:
		return m.cancelRun(), nil
	case key.Matches(msg, m.keys.Quit) && !m.run.running:
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		return m.openHelp(), nil
	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeTaskRun
	case key.Matches(msg, m.keys.Up):
		m.runLog.at--
	case key.Matches(msg, m.keys.Down):
		m.runLog.at++
	case key.Matches(msg, m.keys.Top):
		m.runLog.at = 0
	case key.Matches(msg, m.keys.Bottom):
		m.runLog.at = last
	case msg.String() == "pgup", msg.String() == "ctrl+u":
		m.runLog.at -= page
	case msg.String() == "pgdown", msg.String() == "ctrl+d":
		m.runLog.at += page
	}

	m.runLog.at = max(0, min(m.runLog.at, last))

	return m, nil
}
