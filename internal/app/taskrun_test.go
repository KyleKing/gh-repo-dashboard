//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

func tempRepos(t *testing.T, names ...string) []string {
	t.Helper()

	root := t.TempDir()
	paths := make([]string, 0, len(names))
	for _, name := range names {
		dir := filepath.Join(root, name)
		//nolint:gosec // fixed binary, directory from t.TempDir
		if out, err := exec.CommandContext(t.Context(), "git", "init", "-q", dir).CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		paths = append(paths, dir)
	}

	return paths
}

// drainRun feeds the run's messages back to the model until the engine reports
// it finished, returning the model and every message handled.
func drainRun(t *testing.T, m Model, cmd tea.Cmd) (Model, []tea.Msg) {
	t.Helper()

	var seen []tea.Msg
	for cmd != nil {
		msg := cmd()
		seen = append(seen, msg)

		next, nextCmd := m.Update(msg)
		m, cmd = mustModel(t, next), nextCmd

		if _, done := msg.(RunDoneMsg); done {
			break
		}
	}

	return m, seen
}

func runModelOver(tasksList ...config.Task) Model {
	m := New(nil, 1)
	m.loading = false

	return m.WithTasks(tasks.Selector{Store: tasks.NewStore(os.TempDir()), Tasks: tasksList})
}

func TestRunViewFillsRowsAsTheEngineReports(t *testing.T) {
	t.Parallel()

	paths := []string{"/r/a", "/r/b", "/r/c"}
	task := config.Task{Name: "t", Run: "true"}
	engine := func(_ context.Context, _ config.Task, paths []string, progress func(tasks.Result)) []tasks.Result {
		results := make([]tasks.Result, len(paths))
		for i, path := range paths {
			status, exit := tasks.StatusOK, 0
			if path == "/r/b" {
				status, exit = tasks.StatusFailed, 3
			}
			results[i] = tasks.Result{Path: path, Repo: filepath.Base(path), Status: status, ExitCode: exit}
			progress(results[i])
		}

		return results
	}

	m := runModelOver(task).WithEngine(engine)
	m, cmd := m.startRun(task, "t", paths)
	if m.viewMode != ViewModeTaskRun || !m.run.running || m.run.finished() != 0 {
		t.Fatalf("a started run is running with every repo queued: %+v", m.run)
	}

	m, seen := drainRun(t, m, cmd)

	if len(seen) != len(paths)+1 {
		t.Errorf("handled %d messages; want one per repo plus the done message", len(seen))
	}
	if m.run.running || m.run.finished() != 3 || m.run.count(tasks.StatusFailed) != 1 {
		t.Errorf("finished run = %+v; want 3 done and 1 failed", m.run)
	}
	if got := m.run.rows[1].result.ExitCode; got != 3 {
		t.Errorf("row b exit = %d; want 3", got)
	}

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if got := mustModel(t, next).viewMode; got != ViewModeRepoList {
		t.Errorf("esc after the run returns to %v; want the repo list", got)
	}
}

func TestRunViewCtrlCCancelsARunningTask(t *testing.T) {
	t.Parallel()

	paths := []string{"/r/a", "/r/b"}
	task := config.Task{Name: "t", Run: "true"}
	engine := func(ctx context.Context, _ config.Task, paths []string, _ func(tasks.Result)) []tasks.Result {
		<-ctx.Done()

		results := make([]tasks.Result, len(paths))
		for i, path := range paths {
			results[i] = tasks.Result{Path: path, Repo: filepath.Base(path), Status: tasks.StatusCanceled, ExitCode: -1}
		}

		return results
	}

	m := runModelOver(task).WithEngine(engine)
	m, cmd := m.startRun(task, "t", paths)

	first := make(chan tea.Msg, 1)
	go func() { first <- cmd() }()

	quit, quitCmd := m.Update(keyPress('q'))
	if quitCmd != nil || !mustModel(t, quit).run.running {
		t.Fatal("q must not leave a running task")
	}

	canceled, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	m = mustModel(t, canceled)
	if !m.run.canceling {
		t.Fatal("ctrl-c should mark the run canceling")
	}

	done, _ := m.Update(<-first)
	m = mustModel(t, done)

	if m.run.running || m.run.count(tasks.StatusCanceled) != 2 {
		t.Errorf("canceled run = %+v; want both repos canceled and nothing running", m.run)
	}
	if _, cmd := m.Update(keyPress('q')); cmd == nil {
		t.Error("q should quit once the run is over")
	}
}

func TestRunViewRunsTheRealEngineAgainstTempRepos(t *testing.T) {
	t.Parallel()

	paths := tempRepos(t, "good", "bad")
	store := tasks.NewStore(t.TempDir())
	task := config.Task{Name: "pick", Run: `[ "$REPO_NAME" = good ]`}

	m := New(nil, 1)
	m.loading = false
	m = m.WithTasks(tasks.Selector{Store: store, Tasks: []config.Task{task}})
	m, cmd := m.startRun(task, "pick", paths)
	m, _ = drainRun(t, m, cmd)

	good, bad := m.run.rows[0].result, m.run.rows[1].result
	if good.Status != tasks.StatusOK || bad.Status != tasks.StatusFailed || bad.ExitCode != 1 {
		t.Errorf("results = %+v, %+v; want good ok and bad failed with exit 1", good, bad)
	}
	if _, err := os.Stat(bad.LogPath); err != nil {
		t.Errorf("the failed repo should have a log: %v", err)
	}
	if history := store.History("pick"); len(history) != 1 || history[0].OK != 1 || history[0].Failed != 1 {
		t.Errorf("history = %+v; want one run, 1 ok and 1 failed", history)
	}
}
