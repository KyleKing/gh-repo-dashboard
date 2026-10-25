package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

var (
	// ErrUnknownTask reports a task name that no [[tasks]] entry defines.
	ErrUnknownTask = errors.New("unknown task")
	// ErrNoTask reports a run with neither a task name nor --due.
	ErrNoTask = errors.New("name a task to run, or pass --due")
	// ErrTasksFailed reports that at least one repo's run did not succeed.
	ErrTasksFailed = errors.New("task failed on at least one repo")
)

// RunOptions configures one `run` invocation.
type RunOptions struct {
	// Task names the task to run. Empty with Due runs every task.
	Task string
	// Due restricts each task to repos whose schedule calls for a run.
	Due bool
	// Where narrows a task's own where, and must hold in addition to it.
	Where    string
	DryRun   bool
	Yes      bool
	MaxDepth int
	Tasks    []config.Task
	Store    *tasks.Store
}

// RunTasks runs the selected task or tasks over the repos under roots and
// prints a per-repo table to w. A confirm task asks on w and reads the answer
// from in unless opts.Yes is set. It returns ErrTasksFailed after printing if
// any repo failed.
func RunTasks(ctx context.Context, w io.Writer, in io.Reader, roots []string, opts RunOptions) error {
	selected, err := chooseTasks(opts)
	if err != nil {
		return err
	}

	paths := discovery.DiscoverRepos(roots, opts.MaxDepth)
	sel := tasks.Selector{Store: opts.Store, Tasks: opts.Tasks}
	reader := bufio.NewReader(in)
	failed := false

	for _, task := range selected {
		repos, err := tasks.Select(ctx, sel, task, opts.Where, opts.Due, paths)
		if err != nil {
			return fmt.Errorf("selecting repos for %s: %w", task.Name, err)
		}

		switch {
		case opts.DryRun:
			printDryRun(w, task, repos)
		case len(repos) == 0:
			sayf(w, "%s: no repos to run on\n", task.Name)
		case task.Confirm && !opts.Yes && !confirmed(w, reader, task, len(repos)):
			sayf(w, "%s: skipped\n", task.Name)
		default:
			results := tasks.Run(ctx, opts.Store, task, repos, nil)
			printResults(w, task, results)
			for _, result := range results {
				failed = failed || result.Failed()
			}
		}
	}

	if failed {
		return ErrTasksFailed
	}

	return nil
}

func chooseTasks(opts RunOptions) ([]config.Task, error) {
	if opts.Task == "" {
		if !opts.Due {
			return nil, ErrNoTask
		}

		return opts.Tasks, nil
	}
	for _, task := range opts.Tasks {
		if task.Name == opts.Task {
			return []config.Task{task}, nil
		}
	}

	return nil, fmt.Errorf("%w %q", ErrUnknownTask, opts.Task)
}

func confirmed(w io.Writer, in *bufio.Reader, task config.Task, count int) bool {
	sayf(w, "Run %s on %d repos? [y/N] ", task.Name, count)
	answer, _ := in.ReadString('\n') //nolint:errcheck // EOF reads as "no"
	sayf(w, "\n")

	return strings.EqualFold(strings.TrimSpace(answer), "y")
}

func printDryRun(w io.Writer, task config.Task, repos []string) {
	sayf(w, "%s: would run on %d repos\n", task.Name, len(repos))
	for _, path := range repos {
		sayf(w, "  %s\n", path)
	}
}

func printResults(w io.Writer, task config.Task, results []tasks.Result) {
	sayf(w, "%s\n", task.Name)
	table := tabwriter.NewWriter(w, 0, 0, tablePadding, ' ', 0)
	sayf(table, "REPO\tSTATUS\tEXIT\tTIME\tLOG\n")
	for _, result := range results {
		sayf(table, "%s\t%s\t%d\t%s\t%s\n",
			result.Repo, result.Status, result.ExitCode, result.Duration.Round(time.Millisecond), result.LogPath)
	}
	table.Flush() //nolint:errcheck,gosec // best-effort output
	for _, result := range results {
		if result.Err != nil {
			sayf(w, "%s: %v\n", result.Repo, result.Err)
		}
	}
}

const tablePadding = 2

// sayf writes best-effort command output, where a failed write has no recovery.
//
//nolint:errcheck // see comment above
func sayf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
