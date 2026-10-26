//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/tui/handoff"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

var errExit = errors.New("exit status 3")

func handoffModel(t *testing.T, list ...config.Task) Model {
	t.Helper()

	m := focusedModel(160, 45)
	m.taskSel = &tasks.Selector{Store: tasks.NewStore(t.TempDir()), Tasks: append(list, tasks.Builtins()...)}

	return m
}

func actionKeyNames(actions []panelAction) []string {
	names := make([]string, 0, len(actions))
	for _, a := range actions {
		names = append(names, a.name)
	}

	return names
}

func TestVerbsMenuOffersAHandoffOnlyWhereItsPlaceholdersExist(t *testing.T) {
	t.Parallel()

	review := config.Task{Name: "review", Desc: "review it", Run: "second-look {pr}", Interactive: true}
	switchTo := config.Task{Name: "lazy", Desc: "lazygit", Run: "lazygit -b {branch}", Interactive: true}

	tests := []struct {
		name  string
		panel panelID
		want  []string
		not   []string
	}{
		{"on a PR", panelPRs, []string{"review it (hand-off)", "lazygit (hand-off)"}, nil},
		{"on a branch without a PR", panelBranches, []string{"lazygit (hand-off)"}, []string{"review it (hand-off)"}},
		{"on the repo", panelStatus, []string{"a shell in the repo (hand-off)"}, []string{"review it (hand-off)"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := handoffModel(t, review, switchTo)
			m.focusedPanel = tt.panel

			got := actionKeyNames(m.actionMenu().actions)
			for _, want := range tt.want {
				if !slices.Contains(got, want) {
					t.Errorf("menu %v is missing %q", got, want)
				}
			}

			for _, not := range tt.not {
				if slices.Contains(got, not) {
					t.Errorf("menu %v offers %q without the value it needs", got, not)
				}
			}
		})
	}
}

func TestHandoffKeysAvoidTheMenusOwnVerbs(t *testing.T) {
	t.Parallel()

	m := handoffModel(t, config.Task{Name: "copy-it", Run: "true", Interactive: true})
	m.focusedPanel = panelBranches

	seen := map[string]bool{}
	for _, a := range m.actionMenu().actions {
		if seen[a.key] {
			t.Errorf("key %q is bound twice in %v", a.key, actionKeyNames(m.actionMenu().actions))
		}

		seen[a.key] = true
	}
}

func TestHandoffTasksStayOutOfTheTasksTab(t *testing.T) {
	t.Parallel()

	m := handoffModel(t, config.Task{Name: "lazy", Run: "lazygit", Interactive: true})

	for _, task := range m.taskList() {
		if task.Interactive {
			t.Errorf("the Tasks tab lists the hand-off %q", task.Name)
		}
	}
}

func TestHandoffReturnReportsTheExitAndRereadsTheRepo(t *testing.T) {
	t.Parallel()

	m := handoffModel(t)

	next, cmd := m.Update(handoff.DoneMsg{Name: "lazy", Dir: "/dev/alpha", Err: errExit})
	if got := mustModel(t, next).statusMessage; !strings.Contains(got, "exit status 3") {
		t.Errorf("status = %q, want the exit named", got)
	}

	if cmd == nil {
		t.Error("returning from a hand-off must re-read the repo")
	}
}

// Bubble Tea's ExecProcess needs a terminal to run, so this builds the command a
// hand-off would run and executes its shell script in the repo directory.
func TestHandoffScriptRunsQuotedInTheRepoDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	branch := "feat/$(touch pwned);'x"
	task := config.Task{Name: "echo", Run: "printf '%s' {branch} > out.txt && pwd > cwd.txt", Interactive: true}

	m := handoffModel(t, task)
	m.summaries["/dev/alpha"] = models.RepoSummary{
		RepoSummary: vcs.RepoSummary{Path: dir, VCSType: vcs.TypeGit, Branch: branch},
	}
	m.focusedPanel = panelStatus

	target, ok := m.focusTarget()
	if !ok || !target.supports(task) {
		t.Fatalf("target %+v should support %q", target, task.Run)
	}

	if _, cmd := m.startHandoff(task, target); cmd == nil {
		t.Fatal("startHandoff returned no command")
	}

	script, err := handoff.Expand(task.Run, target.values())
	if err != nil {
		t.Fatal(err)
	}

	//nolint:gosec // the script is built from this test's own template
	run := exec.CommandContext(t.Context(), "sh", "-c", script)
	run.Dir = target.path

	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}

	//nolint:gosec // the path is inside the test's temp dir
	got, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil || string(got) != branch {
		t.Errorf("branch arrived as %q (%v), want it verbatim", got, err)
	}

	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Error("the branch name was interpreted by the shell")
	}
}
