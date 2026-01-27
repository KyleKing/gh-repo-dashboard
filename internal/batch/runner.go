package batch

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

type TaskResult struct {
	Path       string
	RepoName   string
	Success    bool
	Message    string
	DurationMs int64
}

type TaskProgressMsg struct {
	Result TaskResult
}

type TaskCompleteMsg struct {
	TaskName string
	Results  []TaskResult
}

type TaskFunc func(ctx context.Context, ops vcs.Operations, repoPath string) (success bool, message string, err error)

func RunTask(taskName string, paths []string, taskFn TaskFunc) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var results []TaskResult

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
