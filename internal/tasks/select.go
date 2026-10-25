package tasks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kyleking/aragonite/filter"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/copier"
	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

const summaryWorkers = 8

// Selector decides which repos a task applies to and which are due, reading
// run history from Store and sibling tasks from Tasks (for due(name)).
type Selector struct {
	Store *Store
	Tasks []config.Task
	Now   func() time.Time
}

func (sel Selector) now() time.Time {
	if sel.Now != nil {
		return sel.Now()
	}

	return time.Now()
}

// Runs exposes the run history to predicate expressions.
func (sel Selector) Runs() filters.Runs {
	return filters.Runs{
		Now: sel.now,
		LastRun: func(task, repoPath string) (time.Time, bool) {
			stamp, ok := sel.Store.LastStamp(task, repoPath)
			return stamp.At, ok
		},
		Due: func(name string, s models.RepoSummary) bool {
			task, ok := sel.task(name)
			return ok && sel.Stale(task, s.Path)
		},
	}
}

func (sel Selector) task(name string) (config.Task, bool) {
	for _, task := range sel.Tasks {
		if task.Name == name {
			return task, true
		}
	}

	return config.Task{}, false
}

// Stale reports whether task's schedule calls for a run on repoPath: it has an
// interval and the repo never succeeded or last succeeded longer ago than it.
// A task without an interval is never stale.
func (sel Selector) Stale(task config.Task, repoPath string) bool {
	interval, err := task.Interval()
	if err != nil || interval <= 0 {
		return false
	}
	stamp, ok := sel.Store.LastStamp(task.Name, repoPath)
	if !ok || stamp.LastSuccess.IsZero() {
		return true
	}

	return sel.now().Sub(stamp.LastSuccess) > interval
}

// Where compiles task.Where and the optional extra expression into one
// predicate that requires both. Either may be empty.
func (sel Selector) Where(task config.Task, extra string) (filters.Predicate[models.RepoSummary], error) {
	grammar := filters.RepoGrammar(sel.Runs())
	preds := []filters.Predicate[models.RepoSummary]{func(models.RepoSummary) bool { return true }}
	for _, expr := range []string{task.Where, extra} {
		if expr == "" {
			continue
		}
		pred, err := filters.ParseGrammar(expr, grammar)
		if err != nil {
			return nil, fmt.Errorf("task %s: %w", task.Name, err)
		}
		preds = append(preds, pred)
	}

	return filter.All(preds...), nil
}

// Due reports whether task is scheduled for the repo: its where holds and
// Stale is true.
func (sel Selector) Due(task config.Task, s models.RepoSummary) (bool, error) {
	where, err := sel.Where(task, "")
	if err != nil {
		return false, err
	}

	return where(s) && sel.Stale(task, s.Path), nil
}

// Select returns the paths whose summary satisfies where and, when dueOnly is
// set, whose task is Stale. Order follows paths.
func Select(
	ctx context.Context, sel Selector, task config.Task, extraWhere string, dueOnly bool, paths []string,
) ([]string, error) {
	where, err := sel.Where(task, extraWhere)
	if err != nil {
		return nil, err
	}

	keep := make([]bool, len(paths))
	sem := make(chan struct{}, summaryWorkers)
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			keep[i] = where(ReadSummary(ctx, path)) && (!dueOnly || sel.Stale(task, path))
		}()
	}
	wg.Wait()

	selected := make([]string, 0, len(paths))
	for i, path := range paths {
		if keep[i] {
			selected = append(selected, path)
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("reading summaries: %w", err)
	}

	return selected, nil
}

// ReadSummary reads the fields predicates match against: the VCS summary and
// the copier template. A read error leaves a partial summary, which matches
// only error-tolerant predicates.
func ReadSummary(ctx context.Context, path string) models.RepoSummary {
	//nolint:errcheck // the summary carries Error and is still matched
	summary, _ := models.ReadSummary(ctx, vcs.GetOperations(path), path)
	//nolint:errcheck // a repo with no template just leaves the field empty
	summary.TemplateInfo, _ = copier.GetTemplateInfo(ctx, path)

	return summary
}
