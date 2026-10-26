//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"context"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

var tabTasks = []config.Task{
	{Name: "fix", Desc: "tidy dirty repos", Run: "true", Where: "dirty", Every: "30d"},
	{Name: "ask", Desc: "needs a yes first", Run: "true", Confirm: true},
}

func tabModel(t *testing.T) Model {
	t.Helper()

	m := New([]string{"/r"}, 1)
	m.loading = false
	m.width, m.height = 120, 35
	m.repoPaths = []string{"/r/clean", "/r/dirty", "/r/messy"}
	for _, path := range m.repoPaths {
		summary := models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: path, Branch: mainBranchName}}
		if path != "/r/clean" {
			summary.Unstaged = 1
		}
		m.summaries[path] = summary
	}
	m.updateFilteredPaths()

	registry, err := tasks.Registry(tabTasks)
	if err != nil {
		t.Fatal(err)
	}

	return m.WithTasks(tasks.Selector{Store: tasks.NewStore(t.TempDir()), Tasks: registry})
}

func press(t *testing.T, m Model, keys ...tea.KeyPressMsg) Model {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(k)
		m = mustModel(t, next)
	}

	return m
}

func idleEngine(context.Context, config.Task, []string, func(tasks.Result)) []tasks.Result {
	return nil
}

var (
	keyUpper = func(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }
	keyEscP  = tea.KeyPressMsg{Code: tea.KeyEscape}
	keyEntP  = tea.KeyPressMsg{Code: tea.KeyEnter}
	keyTabP  = tea.KeyPressMsg{Code: tea.KeyTab}
)

func TestTabKeysSwitchBetweenReposAndTasks(t *testing.T) {
	t.Parallel()

	m := tabModel(t)
	m.due["/r/messy"] = []string{"fix"}
	m.cursor = slices.Index(m.filteredPaths, "/r/messy")

	tests := []struct {
		name       string
		keys       []tea.KeyPressMsg
		wantView   ViewMode
		wantCursor int
	}{
		{"T opens the tab on the task due for the cursor row", []tea.KeyPressMsg{keyUpper('T')}, ViewModeTasks, 0},
		{"R returns to the repos", []tea.KeyPressMsg{keyUpper('T'), keyUpper('R')}, ViewModeRepoList, 0},
		{"tab toggles both ways", []tea.KeyPressMsg{keyTabP, keyTabP}, ViewModeRepoList, 0},
		{"esc leaves the tab", []tea.KeyPressMsg{keyTabP, keyEscP}, ViewModeRepoList, 0},
		{
			"j stops at the last task",
			[]tea.KeyPressMsg{keyUpper('T'), keyUpper('G'), keyUpper('j')},
			ViewModeTasks,
			len(tabTasks) + len(config.Fleet(tasks.Builtins())) - 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := press(t, m, tt.keys...)
			if got.viewMode != tt.wantView || (tt.wantView == ViewModeTasks && got.tasksCursor != tt.wantCursor) {
				t.Errorf("view %v cursor %d; want %v cursor %d",
					got.viewMode, got.tasksCursor, tt.wantView, tt.wantCursor)
			}
			if got.currentTab() == TabTasks != (tt.wantView == ViewModeTasks) {
				t.Errorf("tab bar highlights %v for view %v", got.currentTab(), got.viewMode)
			}
		})
	}
}

func TestTaskPreviewListsMarksElseWhereAndEscRunsNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		marks  []string
		want   []string
		source string
	}{
		{"where picks the dirty repos", nil, []string{"/r/dirty", "/r/messy"}, "where dirty"},
		{"marks override where", []string{"/r/clean"}, []string{"/r/clean"}, sourceMarked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := tabModel(t)
			m.selectedPaths = map[string]bool{}
			for _, path := range tt.marks {
				m.selectedPaths[path] = true
			}

			m = press(t, m, keyUpper('T'), keyEntP)
			if m.viewMode != ViewModeTaskPreview || m.preview.source != tt.source {
				t.Fatalf("view %v source %q; want the preview of %q", m.viewMode, m.preview.source, tt.source)
			}
			if got := m.previewPaths(); !slices.Equal(got, tt.want) {
				t.Errorf("preview rows = %v; want %v", got, tt.want)
			}
			if !m.preview.rows[0].due && tt.source != sourceMarked {
				t.Error("a never-run task with every set is due")
			}

			m = press(t, m, keyEscP)
			if m.viewMode != ViewModeTasks || m.run.id != 0 {
				t.Errorf("esc: view %v, runs started %d; want the task list and no run", m.viewMode, m.run.id)
			}
		})
	}
}

func TestTaskPreviewEnterAsksOnlyWhenTheTaskDoes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		task     string
		wantView ViewMode
	}{
		{"fix runs at once", "fix", ViewModeTaskRun},
		{"ask confirms first", "ask", ViewModeConfirm},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := tabModel(t).WithEngine(idleEngine)
			m.tasksCursor = slices.IndexFunc(m.taskList(), func(c config.Task) bool { return c.Name == tt.task })

			m = press(t, m, keyUpper('T'), keyEntP, keyEntP)
			if m.viewMode != tt.wantView {
				t.Fatalf("view = %v; want %v", m.viewMode, tt.wantView)
			}
			if tt.wantView != ViewModeConfirm {
				return
			}

			canceled := press(t, m, keyUpper('n'))
			if canceled.viewMode != ViewModeTaskPreview || canceled.run.id != 0 {
				t.Errorf("n: view %v, runs %d; want the preview and no run", canceled.viewMode, canceled.run.id)
			}

			confirmed := press(t, m, keyUpper('y'))
			if confirmed.viewMode != ViewModeTaskRun || confirmed.run.returnMode != ViewModeTasks {
				t.Errorf("y: view %v return %v; want the run, returning to the task list",
					confirmed.viewMode, confirmed.run.returnMode)
			}
		})
	}
}

func TestRunLogOpensFromARowAndEscWalksBackToTheTaskList(t *testing.T) {
	t.Parallel()

	paths := tempRepos(t, "noisy")
	store := tasks.NewStore(t.TempDir())
	task := config.Task{Name: "talk", Run: `seq 1 40`}

	m := New(nil, 1)
	m.loading, m.width, m.height = false, 100, 24
	m = m.WithTasks(tasks.Selector{Store: store, Tasks: []config.Task{task}})
	m.viewMode = ViewModeTasks
	m, cmd := m.startRun(task, "talk", paths)
	m, _ = drainRun(t, m, cmd)

	next, logCmd := m.Update(keyEntP)
	m = mustModel(t, next)
	if m.viewMode != ViewModeTaskLog || logCmd == nil {
		t.Fatalf("enter on a finished row opens its log: view %v", m.viewMode)
	}
	next, _ = m.Update(logCmd())
	m = mustModel(t, next)
	if len(m.runLog.lines) != 40 || m.runLog.lines[39] != "40" {
		t.Fatalf("log has %d lines; want the run's 40, ending 40", len(m.runLog.lines))
	}

	m = press(t, m, keyUpper('G'))
	if m.runLog.at != 39 || !strings.Contains(m.logScreen().body, "40") {
		t.Errorf("G scrolled to line %d; want the last line on screen", m.runLog.at)
	}

	m = press(t, m, keyEscP)
	if m.viewMode != ViewModeTaskRun {
		t.Fatalf("esc from the log: view %v; want the run", m.viewMode)
	}
	if m = press(t, m, keyEscP); m.viewMode != ViewModeTasks {
		t.Errorf("esc from a finished run: view %v; want the task list", m.viewMode)
	}

	loaded, ok := loadTaskHistoryCmd(store, []config.Task{task})().(TaskHistoryLoadedMsg)
	if !ok {
		t.Fatal("the history command should report a TaskHistoryLoadedMsg")
	}
	if history := loaded.History["talk"]; len(history) != 1 || history[0].OK != 1 {
		t.Errorf("history = %+v; want the one finished run", history)
	}
}

func TestRunCommandOpensThePreviewFromAnyScreen(t *testing.T) {
	t.Parallel()

	m := tabModel(t)
	got, _ := m.ExecuteCommand("run fix")
	if got.viewMode != ViewModeTaskPreview || got.preview.returnMode != ViewModeRepoList {
		t.Errorf(":run fix: view %v return %v; want the preview returning to the list",
			got.viewMode, got.preview.returnMode)
	}

	for _, line := range []string{"run nope", "run"} {
		bad, cmd := m.ExecuteCommand(line)
		if bad.viewMode != ViewModeRepoList || cmd == nil {
			t.Fatalf(":%s: view %v; want to stay on the list with a status", line, bad.viewMode)
		}
		if msg, ok := cmd().(StatusMsg); !ok || !msg.IsError {
			t.Errorf(":%s: got %v; want an error status", line, msg)
		}
	}
}
