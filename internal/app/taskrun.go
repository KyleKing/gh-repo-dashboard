package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

// RunEngine runs task over paths and reports each repo as it finishes. It is
// the boundary between the run view and the task engine.
type RunEngine func(ctx context.Context, task config.Task, paths []string, progress func(tasks.Result)) []tasks.Result

// runRow is one repo of a run: queued until the engine reports it.
type runRow struct {
	path   string
	done   bool
	result tasks.Result
}

// taskRun is the run view's state. Its ch carries the engine's progress and is
// drained one message at a time, so the engine never waits on the UI.
type taskRun struct {
	id         int
	task       config.Task
	label      string
	paths      []string
	rows       []runRow
	running    bool
	canceling  bool
	cancel     context.CancelFunc
	ch         <-chan tea.Msg
	cursor     int
	returnMode ViewMode
}

func (r taskRun) finished() int {
	done := 0
	for i := range r.rows {
		if r.rows[i].done {
			done++
		}
	}

	return done
}

func (r taskRun) count(status tasks.Status) int {
	count := 0
	for i := range r.rows {
		if r.rows[i].done && r.rows[i].result.Status == status {
			count++
		}
	}

	return count
}

func (r taskRun) withResult(result tasks.Result) taskRun {
	for i := range r.rows {
		if r.rows[i].path == result.Path {
			r.rows[i] = runRow{path: result.Path, done: true, result: result}
		}
	}

	return r
}

// RunProgressMsg reports one repo finishing within run ID.
type RunProgressMsg struct {
	ID     int
	Result tasks.Result
}

// RunDoneMsg reports run ID finished, with the results in path order.
type RunDoneMsg struct {
	ID      int
	Results []tasks.Result
}

// WithEngine replaces the engine a task run goes through.
func (m Model) WithEngine(engine RunEngine) Model {
	m.engine = engine

	return m
}

func (m Model) taskList() []config.Task {
	if m.taskSel != nil {
		return m.taskSel.Tasks
	}

	return tasks.Builtins()
}

func (m Model) findTask(name string) (config.Task, bool) {
	for _, task := range m.taskList() {
		if task.Name == name {
			return task, true
		}
	}

	return config.Task{}, false
}

func (m Model) taskStore() *tasks.Store {
	if m.taskSel != nil && m.taskSel.Store != nil {
		return m.taskSel.Store
	}
	if store, err := tasks.DefaultStore(); err == nil {
		return store
	}

	return tasks.NewStore(filepath.Join(os.TempDir(), "gh-repo-dashboard", "tasks"))
}

func (m Model) engineFor() RunEngine {
	if m.engine != nil {
		return m.engine
	}

	store := m.taskStore()

	return func(ctx context.Context, task config.Task, paths []string, progress func(tasks.Result)) []tasks.Result {
		return tasks.Run(ctx, store, task, paths, progress)
	}
}

// confirmAndRun starts task over paths, asking first when the task says so.
func (m Model) confirmAndRun(task config.Task, label string, paths []string) (Model, tea.Cmd) {
	if !task.Confirm || len(paths) == 0 {
		return m.startRun(task, label, paths)
	}

	scope := fmt.Sprintf("across %d repos", len(paths))
	if len(paths) == 1 {
		scope = "in " + filepath.Base(paths[0])
	}

	return m.confirmRun(label+"?", repoNameList(paths), scope, func(m Model) (Model, tea.Cmd) {
		return m.startRun(task, label, paths)
	})
}

// runNamed is confirmAndRun for the task a key or :command names.
func (m Model) runNamed(name, label string, paths []string) (Model, tea.Cmd) {
	task, ok := m.findTask(name)
	if !ok {
		return m, statusErrCmd("Unknown task: " + name)
	}

	return m.confirmAndRun(task, label, paths)
}

// startRun opens the run view and starts the engine. The returned command
// waits for the first message and each handler re-arms it for the next.
func (m Model) startRun(task config.Task, label string, paths []string) (Model, tea.Cmd) {
	if len(paths) == 0 {
		return m, nil
	}

	returnMode := m.viewMode
	if returnMode == ViewModeTaskPreview {
		returnMode = ViewModeTasks
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, len(paths)+1)

	rows := make([]runRow, len(paths))
	for i, path := range paths {
		rows[i].path = path
	}

	m.run = taskRun{
		id: m.run.id + 1, task: task, label: label, paths: paths, rows: rows,
		running: true, cancel: cancel, ch: ch, returnMode: returnMode,
	}
	m.viewMode = ViewModeTaskRun

	return m, launchRunCmd(ctx, m.engineFor(), m.run, ch)
}

func launchRunCmd(ctx context.Context, engine RunEngine, run taskRun, ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			results := engine(ctx, run.task, run.paths, func(result tasks.Result) {
				ch <- RunProgressMsg{ID: run.id, Result: result}
			})
			ch <- RunDoneMsg{ID: run.id, Results: results}
		}()

		return <-ch
	}
}

func waitRunCmd(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m Model) handleRunProgress(msg RunProgressMsg) (tea.Model, tea.Cmd) {
	if msg.ID != m.run.id {
		return m, nil
	}

	m.run = m.run.withResult(msg.Result)

	return m, waitRunCmd(m.run.ch)
}

func (m Model) handleRunDone(msg RunDoneMsg) (tea.Model, tea.Cmd) {
	if msg.ID != m.run.id {
		return m, nil
	}

	m.run.cancel()
	m.run.running, m.run.canceling = false, false
	for _, result := range msg.Results {
		m.run = m.run.withResult(result)
	}

	cmds := []tea.Cmd{loadTaskHistoryCmd(m.taskStore(), m.taskList())}
	if m.taskSel != nil {
		for _, path := range m.run.paths {
			if summary, ok := m.summaries[path]; ok {
				cmds = append(cmds, loadDueTasksCmd(*m.taskSel, summary))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) cancelRun() Model {
	if !m.run.canceling {
		m.run.cancel()
		m.run.canceling = true
	}

	return m
}

func (m Model) handleRunKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Help):
		return m.openHelp(), nil
	case msg.String() == keyCtrlC && m.run.running:
		return m.cancelRun(), nil
	case key.Matches(msg, m.keys.Quit):
		if !m.run.running {
			return m, tea.Quit
		}
	case key.Matches(msg, m.keys.Back):
		if !m.run.running {
			m.viewMode = m.run.returnMode
		}
	case key.Matches(msg, m.keys.Up):
		m.run.cursor = max(m.run.cursor-1, 0)
	case key.Matches(msg, m.keys.Down):
		m.run.cursor = min(m.run.cursor+1, max(len(m.run.rows)-1, 0))
	case key.Matches(msg, m.keys.Top):
		m.run.cursor = 0
	case key.Matches(msg, m.keys.Bottom):
		m.run.cursor = max(len(m.run.rows)-1, 0)
	case key.Matches(msg, m.keys.Enter):
		return m.openRunLog()
	}

	return m, nil
}
