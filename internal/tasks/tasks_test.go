package tasks_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

func initRepos(t *testing.T, names ...string) []string {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, len(names))
	for _, name := range names {
		dir := filepath.Join(root, name)
		cmd := exec.CommandContext(t.Context(), "git", "init", "-q", dir) // #nosec G204 -- fixed binary
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		commit := exec.CommandContext(t.Context(), "git", "-c", "user.email=t@t", "-c", "user.name=t",
			"commit", "-q", "--allow-empty", "-m", "init")
		commit.Dir = dir
		if out, err := commit.CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v\n%s", err, out)
		}
		paths = append(paths, dir)
	}

	return paths
}

func assertStatuses(t *testing.T, results []tasks.Result, want ...tasks.Status) {
	t.Helper()
	if len(results) != len(want) {
		t.Fatalf("got %d results; want %d", len(results), len(want))
	}
	for i, status := range want {
		if results[i].Status != status {
			t.Errorf("%s status = %s; want %s", results[i].Repo, results[i].Status, status)
		}
	}
}

func TestRunSerialNeverOverlaps(t *testing.T) {
	t.Parallel()
	repos := initRepos(t, "a", "b", "c")
	trace := filepath.Join(t.TempDir(), "trace")
	task := config.Task{
		Name: "serial", Concurrency: 1,
		Run: `echo "start $REPO_NAME" >> ` + trace + `; echo "end $REPO_NAME" >> ` + trace,
	}

	results := tasks.Run(t.Context(), tasks.NewStore(t.TempDir()), task, repos, nil)

	assertStatuses(t, results, tasks.StatusOK, tasks.StatusOK, tasks.StatusOK)
	data, err := os.ReadFile(trace) //nolint:gosec // path built from t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if want := "start a\nend a\nstart b\nend b\nstart c\nend c\n"; string(data) != want {
		t.Errorf("trace = %q; want %q", data, want)
	}
}

func TestRunFailureCarriesExitCodeAndLog(t *testing.T) {
	t.Parallel()
	repos := initRepos(t, "a", "b", "c")
	task := config.Task{
		Name: "flaky",
		Run:  `echo "hello $REPO_NAME"; if [ "$REPO_NAME" = b ]; then echo oops >&2; exit 3; fi`,
	}

	results := tasks.Run(t.Context(), tasks.NewStore(t.TempDir()), task, repos, nil)

	assertStatuses(t, results, tasks.StatusOK, tasks.StatusFailed, tasks.StatusOK)
	if !results[1].Failed() || results[1].ExitCode != 3 {
		t.Errorf("b = %+v; want failed with exit 3", results[1])
	}
	log, err := os.ReadFile(results[1].LogPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(log); !strings.Contains(got, "hello b") || !strings.Contains(got, "oops") {
		t.Errorf("log = %q; want stdout and stderr", got)
	}
}

func TestRunStartsInRepoDirectory(t *testing.T) {
	t.Parallel()
	repos := initRepos(t, "a")
	task := config.Task{Name: "cwd", Run: `[ "$(cd "$REPO_PATH" && pwd -P)" = "$(pwd -P)" ]`}

	assertStatuses(t, tasks.Run(t.Context(), tasks.NewStore(t.TempDir()), task, repos, nil), tasks.StatusOK)
}

func TestRunParallelByDefault(t *testing.T) {
	t.Parallel()
	repos := initRepos(t, "a", "b")
	marks := t.TempDir()
	// Each repo waits for the other's marker, so a serial run never finishes.
	run := `other=a; [ "$REPO_NAME" = a ] && other=b; touch ` + marks + `/$REPO_NAME; ` +
		`while [ ! -e ` + marks + `/$other ]; do :; done`

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	results := tasks.Run(ctx, tasks.NewStore(t.TempDir()), config.Task{Name: "barrier", Run: run}, repos, nil)
	for _, result := range results {
		if result.Status != tasks.StatusOK {
			t.Errorf("%s = %+v; want ok", result.Repo, result)
		}
	}
}

func TestRunCancelMarksUnstartedCanceled(t *testing.T) {
	t.Parallel()
	repos := initRepos(t, "a", "b")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	task := config.Task{Name: "never", Run: "true", Concurrency: 1}
	results := tasks.Run(ctx, tasks.NewStore(t.TempDir()), task, repos, nil)
	for _, result := range results {
		if result.Status != tasks.StatusCanceled {
			t.Errorf("%s status = %s; want canceled", result.Repo, result.Status)
		}
	}
}

func TestDueAndSelect(t *testing.T) {
	t.Parallel()
	repos := initRepos(t, "dirty", "clean")
	if err := os.WriteFile(filepath.Join(repos[0], "wip.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := tasks.NewStore(t.TempDir())
	task := config.Task{Name: "t", Run: "true", Where: "dirty", Every: "30d"}
	now := time.Now()
	sel := tasks.Selector{Store: store, Tasks: []config.Task{task}, Now: func() time.Time { return now }}

	selectPaths := func(extra string, dueOnly bool) []string {
		t.Helper()
		got, err := tasks.Select(t.Context(), sel, task, extra, dueOnly, repos)
		if err != nil {
			t.Fatal(err)
		}

		return got
	}

	if got := selectPaths("", true); len(got) != 1 || got[0] != repos[0] {
		t.Errorf("never run: due = %v; want only the dirty repo", got)
	}
	if got := selectPaths("clean", false); len(got) != 0 {
		t.Errorf("task where AND --where: got %v; want none", got)
	}

	tasks.Run(t.Context(), store, task, repos[:1], nil)
	if got := selectPaths("", true); len(got) != 0 {
		t.Errorf("just ran: due = %v; want none", got)
	}
	if got := selectPaths("", false); len(got) != 1 {
		t.Errorf("not due-only: got %v; want the dirty repo regardless of history", got)
	}

	now = now.Add(31 * 24 * time.Hour)
	if got := selectPaths("", true); len(got) != 1 {
		t.Errorf("31 days later: due = %v; want the dirty repo", got)
	}
	if got := selectPaths("last_run(t) > 30d and due(t)", true); len(got) != 1 {
		t.Errorf("last_run and due predicates: got %v; want the dirty repo", got)
	}
}
