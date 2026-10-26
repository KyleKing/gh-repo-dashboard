package config

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kyleking/aragonite/tui/handoff"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// TaskFunc is a built-in task body. It reports what it did as message and
// signals failure with a non-nil error.
type TaskFunc func(ctx context.Context, repoPath string) (message string, err error)

// Task is a named command run in each selected repo: a shell command from
// config, or a built-in Func.
type Task struct {
	Name string `toml:"name"`
	Desc string `toml:"desc"`
	Run  string `toml:"run"`
	// Func backs a built-in task and is never read from config.
	Func TaskFunc `toml:"-"`
	// Where is a predicate over the repo summary that selects the repos the
	// task applies to. Empty means every repo.
	Where string `toml:"where"`
	// Concurrency caps simultaneous repos. Zero means the engine's default
	// parallelism, and 1 runs serially.
	Concurrency int  `toml:"concurrency"`
	Confirm     bool `toml:"confirm"`
	// Every is how long a successful run keeps a repo from being due again,
	// in the form "30d" or "12h". Empty means never due.
	Every string `toml:"every"`
	// Interactive hands the terminal to Run for the one row under the cursor
	// instead of running it over repos, so Where, Every, and Concurrency do not
	// apply.
	Interactive bool `toml:"interactive"`
}

// Fleet returns the tasks that run over repos, leaving out the hand-offs.
func Fleet(list []Task) []Task {
	return slices.DeleteFunc(slices.Clone(list), func(t Task) bool { return t.Interactive })
}

// Handoffs returns the interactive tasks.
func Handoffs(list []Task) []Task {
	return slices.DeleteFunc(slices.Clone(list), func(t Task) bool { return !t.Interactive })
}

// Interval returns Every as a duration, or zero when unset.
func (t Task) Interval() (time.Duration, error) {
	if t.Every == "" {
		return 0, nil
	}

	interval, err := filters.ParseAge(t.Every)
	if err != nil {
		return 0, fmt.Errorf("parsing every: %w", err)
	}

	return interval, nil
}

// TaskError reports an invalid [[tasks]] entry. Field is the toml key at fault.
type TaskError struct {
	Index int
	Name  string
	Field string
	Err   error
}

func (e *TaskError) Error() string {
	label := fmt.Sprintf("tasks[%d]", e.Index)
	if e.Name != "" {
		label += " " + e.Name
	}

	return fmt.Sprintf("%s: %s: %v", label, e.Field, e.Err)
}

func (e *TaskError) Unwrap() error { return e.Err }

// Each Err below is the reason a [[tasks]] key was rejected, carried by a TaskError.
var (
	ErrTaskNameMissing     = errors.New("is required")
	ErrTaskNameInvalid     = errors.New("must not contain whitespace")
	ErrTaskNameDuplicate   = errors.New("duplicates an earlier task")
	ErrTaskRunMissing      = errors.New("is required")
	ErrTaskNegativeWorkers = errors.New("must be non-negative")
	ErrTaskEveryInvalid    = errors.New("must be a positive duration such as 30d or 12h")
	ErrTaskInteractive     = errors.New("does not apply to an interactive task")
	ErrTaskPlaceholder     = errors.New("names a placeholder other than path, repo, pr, or branch")
)

func validateTasks(tasks []Task) error {
	seen := make(map[string]bool, len(tasks))
	for i, task := range tasks {
		if err := validateTask(i, task, seen); err != nil {
			return err
		}
		seen[task.Name] = true
	}

	return nil
}

func validateTask(i int, task Task, seen map[string]bool) error {
	fail := func(field string, err error) error {
		return &TaskError{Index: i, Name: task.Name, Field: field, Err: err}
	}

	switch {
	case task.Name == "":
		return fail("name", ErrTaskNameMissing)
	case strings.ContainsAny(task.Name, " \t\n"):
		return fail("name", ErrTaskNameInvalid)
	case seen[task.Name]:
		return fail("name", ErrTaskNameDuplicate)
	case strings.TrimSpace(task.Run) == "":
		return fail("run", ErrTaskRunMissing)
	case task.Concurrency < 0:
		return fail("concurrency", ErrTaskNegativeWorkers)
	}

	if task.Interactive {
		return validateHandoff(task, fail)
	}

	if task.Where != "" {
		grammar := filters.RepoGrammar(filters.Runs{})
		if _, err := filters.ParseGrammar[models.RepoSummary](task.Where, grammar); err != nil {
			return fail("where", err)
		}
	}

	if interval, err := task.Interval(); err != nil || (task.Every != "" && interval <= 0) {
		return fail("every", ErrTaskEveryInvalid)
	}

	return nil
}

// HandoffPlaceholders are the values an interactive task's run may use.
var HandoffPlaceholders = []string{"branch", "path", "pr", "repo"}

func validateHandoff(task Task, fail func(string, error) error) error {
	switch {
	case task.Where != "":
		return fail("where", ErrTaskInteractive)
	case task.Every != "":
		return fail("every", ErrTaskInteractive)
	case task.Concurrency != 0:
		return fail("concurrency", ErrTaskInteractive)
	}

	for _, name := range handoff.Needs(task.Run) {
		if !slices.Contains(HandoffPlaceholders, name) {
			return fail("run", fmt.Errorf("%w: {%s}", ErrTaskPlaceholder, name))
		}
	}

	return nil
}
