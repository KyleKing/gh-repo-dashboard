// Package batch runs repo operations (fetch, prune, cleanup) over multiple repos concurrently.
package batch

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// TaskResult reports the outcome of a batch task run on a single repo.
type TaskResult struct {
	Path       string
	RepoName   string
	Success    bool
	Message    string
	DurationMs int64
}

// TaskProgressMsg reports one repo completing within a batch task run.
type TaskProgressMsg struct {
	Result TaskResult
}

// TaskCompleteMsg reports that a batch task run finished with the given results.
type TaskCompleteMsg struct {
	TaskName string
	Results  []TaskResult
}

// TaskFunc runs a batch operation against a single repo.
type TaskFunc func(ctx context.Context, ops vcs.Operations, repoPath string) (success bool, message string, err error)

// RunTask returns a tea.Cmd that runs taskFn over paths sequentially and reports a TaskCompleteMsg.
func RunTask(taskName string, paths []string, taskFn TaskFunc) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		results := make([]TaskResult, 0, len(paths))

		for _, path := range paths {
			ops := vcs.GetOperations(path)
			start := time.Now()

			success, message, err := taskFn(ctx, ops, path)
			if err != nil {
				success = false
				message = err.Error()
			}

			duration := time.Since(start).Milliseconds()

			results = append(results, TaskResult{
				Path:       path,
				RepoName:   repoName(path),
				Success:    success,
				Message:    message,
				DurationMs: duration,
			})
		}

		return TaskCompleteMsg{
			TaskName: taskName,
			Results:  results,
		}
	}
}

func repoName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}

	return path
}
