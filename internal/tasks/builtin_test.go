package tasks_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

var errNetwork = errors.New("network error")

func builtin(t *testing.T, name string) config.Task {
	t.Helper()
	for _, task := range tasks.Builtins() {
		if task.Name == name {
			return task
		}
	}
	t.Fatalf("no built-in named %s", name)

	return config.Task{}
}

func stubOperations(t *testing.T, ops *vcs.MockOperations, heads map[string]string) {
	t.Helper()
	restoreOps := tasks.SetGetOperationsForTest(func(string) vcs.Operations { return ops })
	restoreHeads := tasks.SetGetMergedPRHeadsForTest(
		func(context.Context, string, string) (map[string]string, error) { return heads, nil },
	)
	t.Cleanup(func() { restoreOps(); restoreHeads() })
}

//nolint:paralleltest // stubs shared package-level seam state
func TestBuiltinsMapVCSOutcomeToFailure(t *testing.T) {
	tests := []struct {
		name    string
		task    string
		ops     *vcs.MockOperations
		wantMsg string
		wantErr error
	}{
		{
			name: "fetch succeeds",
			task: tasks.NameFetch,
			ops: &vcs.MockOperations{
				FetchAllFn: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
			},
			wantMsg: "ok",
		},
		{
			name: "prune reporting failure",
			task: tasks.NamePrune,
			ops: &vcs.MockOperations{
				PruneRemoteFn: func(context.Context, string) (bool, string, error) { return false, "no remote", nil },
			},
			wantMsg: "no remote",
			wantErr: tasks.ErrNotOK,
		},
		{
			name: "fetch error propagates",
			task: tasks.NameFetch,
			ops: &vcs.MockOperations{
				FetchAllFn: func(context.Context, string) (bool, string, error) { return false, "", errNetwork },
			},
			wantErr: errNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubOperations(t, tt.ops, nil)

			msg, err := builtin(t, tt.task).Func(t.Context(), "/repo")
			if msg != tt.wantMsg || !errors.Is(err, tt.wantErr) {
				t.Errorf("got (%q, %v); want (%q, %v)", msg, err, tt.wantMsg, tt.wantErr)
			}
		})
	}
}

//nolint:paralleltest // stubs shared package-level seam state
func TestCleanupPassesSquashMergedBranchesAndPreviewListsThem(t *testing.T) {
	var gotSquashMerged []string
	ops := &vcs.MockOperations{
		GetBranchListFn: func(context.Context, string) ([]vcs.BranchInfo, error) {
			return []vcs.BranchInfo{{Name: "feature-a", Head: "abc123"}, {Name: "feature-b", Head: "zzz999"}}, nil
		},
		CleanupMergedBranchesFn: func(_ context.Context, _ string, squashMerged []string) (bool, string, error) {
			gotSquashMerged = squashMerged

			return true, "Deleted 1 branches", nil
		},
	}
	stubOperations(t, ops, map[string]string{"feature-a": "abc123"})

	if _, err := builtin(t, tasks.NameCleanup).Func(t.Context(), "/repo"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSquashMerged, []string{"feature-a"}) {
		t.Errorf("squash-merged = %v; want [feature-a]", gotSquashMerged)
	}

	msg, err := builtin(t, tasks.NameCleanupPreview).Func(t.Context(), "/repo")
	if err != nil || msg != "Would delete 1 branches: feature-a" {
		t.Errorf("preview = (%q, %v)", msg, err)
	}
}

func TestRegistryListsConfiguredFirstAndRejectsBuiltinNames(t *testing.T) {
	t.Parallel()

	got, err := tasks.Registry([]config.Task{{Name: "gc", Run: "true"}})
	if err != nil || got[0].Name != "gc" || len(got) != 1+len(tasks.Builtins()) {
		t.Errorf("Registry = %v, %v; want gc first then the built-ins", got, err)
	}

	_, err = tasks.Registry([]config.Task{{Name: tasks.NameFetch, Run: "true"}})
	if !errors.Is(err, tasks.ErrReserved) {
		t.Errorf("err = %v; want ErrReserved", err)
	}
}

func TestRunRecordsFuncTaskLogStampAndHistory(t *testing.T) {
	t.Parallel()
	repos := initRepos(t, "a", "b")
	store := tasks.NewStore(t.TempDir())
	task := config.Task{
		Name: "fn",
		Func: func(_ context.Context, path string) (string, error) {
			if path == repos[1] {
				return "bad repo", tasks.ErrNotOK
			}

			return "good repo", nil
		},
	}

	for range tasks.HistoryLimit + 2 {
		tasks.Run(t.Context(), store, task, repos, nil)
	}
	results := tasks.Run(t.Context(), store, task, repos, nil)

	assertStatuses(t, results, tasks.StatusOK, tasks.StatusFailed)
	if results[1].ExitCode != 1 || results[1].Note() != "bad repo" {
		t.Errorf("failed result = %+v; want exit 1 and the reported message", results[1])
	}
	if log, err := os.ReadFile(results[0].LogPath); err != nil || string(log) != "good repo\n" {
		t.Errorf("log = %q, %v", log, err)
	}
	if stamp, ok := store.LastStamp("fn", repos[1]); !ok || stamp.Success {
		t.Errorf("stamp = %+v, %v; want a failed stamp", stamp, ok)
	}

	history := store.History("fn")
	if len(history) != tasks.HistoryLimit || history[0].OK != 1 || history[0].Failed != 1 || history[0].Repos != 2 {
		t.Errorf("history = %d runs, newest %+v; want the last %d", len(history), history[0], tasks.HistoryLimit)
	}
}
