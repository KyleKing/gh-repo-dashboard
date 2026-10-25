package tasks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
)

// DefaultConcurrency caps parallel repos when a task sets no concurrency.
const DefaultConcurrency = 8

const logDirMode = 0o750

// Status is how a repo's run ended.
type Status string

// The three ways a repo's run can end.
const (
	StatusOK       Status = "ok"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

// Result reports one repo's run. ExitCode is -1 when the shell never started
// and 1 when a built-in failed. Message is what a built-in reported.
type Result struct {
	Path     string
	Repo     string
	Status   Status
	ExitCode int
	Duration time.Duration
	LogPath  string
	Message  string
	Err      error
}

// Note is the line that explains a result: what a built-in reported, else the error.
func (r Result) Note() string {
	switch {
	case r.Message != "":
		return r.Message
	case r.Err != nil:
		return r.Err.Error()
	}

	return ""
}

// Failed reports whether the repo's run did not succeed.
func (r Result) Failed() bool {
	return r.Status != StatusOK
}

// Run executes task in each of paths, at most task.Concurrency at a time, and
// returns one Result per path in input order. A non-nil progress receives each
// Result as it finishes, never from two goroutines at once. Canceling ctx stops
// running commands and marks unstarted repos canceled. The run is appended to
// the task's history once every repo has finished.
func Run(ctx context.Context, store *Store, task config.Task, paths []string, progress func(Result)) []Result {
	started := time.Now()
	results := runAll(ctx, store, task, paths, progress)
	if len(paths) > 0 {
		//nolint:errcheck // history is advisory, and the results already carry every failure
		_ = store.appendRun(task.Name, newRecord(started, results))
	}

	return results
}

func runAll(ctx context.Context, store *Store, task config.Task, paths []string, progress func(Result)) []Result {
	workers := task.Concurrency
	if workers == 0 {
		workers = DefaultConcurrency
	}

	results := make([]Result, len(paths))
	jobs := make(chan int)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for range min(workers, max(len(paths), 1)) {
		wg.Go(func() {
			for i := range jobs {
				result := runOrCancel(ctx, store, task, paths[i])
				results[i] = result

				mu.Lock()
				if progress != nil {
					progress(result)
				}
				mu.Unlock()
			}
		})
	}
	for i := range paths {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

func runOrCancel(ctx context.Context, store *Store, task config.Task, path string) Result {
	if err := ctx.Err(); err != nil {
		return Result{Path: path, Repo: filepath.Base(path), Status: StatusCanceled, ExitCode: -1, Err: err}
	}

	return runOne(ctx, store, task, path)
}

func runOne(ctx context.Context, store *Store, task config.Task, path string) Result {
	result := Result{Path: path, Repo: filepath.Base(path), LogPath: store.LogPath(task.Name, path), ExitCode: -1}
	start := time.Now()

	message, err := execute(ctx, task, path, result)
	result.Message = message
	result.Duration = time.Since(start)

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		result.Status, result.ExitCode = StatusOK, 0
	case ctx.Err() != nil:
		result.Status, result.Err = StatusCanceled, ctx.Err()

		return result
	case task.Func != nil:
		result.Status, result.ExitCode, result.Err = StatusFailed, 1, err
	case errors.As(err, &exitErr):
		result.Status, result.ExitCode = StatusFailed, exitErr.ExitCode()
	default:
		result.Status, result.Err = StatusFailed, err
	}

	if recordErr := store.record(task.Name, path, result.ExitCode, start); recordErr != nil && result.Err == nil {
		result.Err = recordErr
	}

	return result
}

func execute(ctx context.Context, task config.Task, path string, result Result) (string, error) {
	if err := os.MkdirAll(filepath.Dir(result.LogPath), logDirMode); err != nil {
		return "", fmt.Errorf("creating log directory: %w", err)
	}
	logFile, err := os.Create(result.LogPath)
	if err != nil {
		return "", fmt.Errorf("creating log: %w", err)
	}
	defer logFile.Close() //nolint:errcheck // the log is best-effort output

	if task.Func != nil {
		return executeFunc(ctx, task, path, logFile)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", task.Run) //nolint:gosec // running the configured command is the point
	cmd.Dir = path
	cmd.Env = append(os.Environ(), "REPO_NAME="+result.Repo, "REPO_PATH="+path)
	cmd.Stdout, cmd.Stderr = logFile, logFile

	return "", cmd.Run() //nolint:wrapcheck // the caller inspects *exec.ExitError
}

func executeFunc(ctx context.Context, task config.Task, path string, log io.Writer) (string, error) {
	message, err := task.Func(ctx, path)
	if message != "" {
		fmt.Fprintln(log, message) //nolint:errcheck // the log is best-effort output
	}
	if err != nil {
		fmt.Fprintln(log, err) //nolint:errcheck // the log is best-effort output
	}

	return message, err //nolint:wrapcheck // the result carries the built-in's own error
}
