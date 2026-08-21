package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// errScriptFailed reports that the script ran to the end with at least one line
// rejected or failed, so the caller exits nonzero.
var errScriptFailed = errors.New("script had failing commands")

// RunScript executes ":command" lines from script against the repos under
// scanPaths, headlessly and sequentially, writing human-readable results to w.
// Blank lines and #-comments are skipped; a "quit" command stops early.
//
// Every line runs even after one fails; the failures are counted and reported
// once at the end.
func RunScript(ctx context.Context, w io.Writer, scanPaths []string, maxDepth int, script io.Reader) error {
	m := newScriptModel(ctx, scanPaths, maxDepth)

	failed := 0
	quit := false

	scanner := bufio.NewScanner(script)
	for scanner.Scan() && !quit {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var outcome lineOutcome
		//nolint:contextcheck // tea.Model.Update has no context parameter
		m, outcome = runScriptLine(m, line, w)
		quit = outcome.quit
		if outcome.failed {
			failed++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading script: %w", err)
	}

	if failed > 0 {
		return fmt.Errorf("%d of the lines failed: %w", failed, errScriptFailed)
	}

	return nil
}

// newScriptModel builds a Model with summaries loaded synchronously, since
// script mode has no Tea event loop to deliver progressive updates.
func newScriptModel(ctx context.Context, scanPaths []string, maxDepth int) Model {
	m := New(scanPaths, maxDepth)
	m.headless = true
	m.loading = false
	m.repoPaths = discovery.DiscoverRepos(scanPaths, maxDepth)

	for _, path := range m.repoPaths {
		//nolint:errcheck // the summary carries the error for the row to render
		summary, _ := models.ReadSummary(ctx, vcs.GetOperations(path), path)
		m.summaries[path] = summary
	}
	m.updateFilteredPaths()

	return m
}

// scriptPrintf writes formatted script output, ignoring write errors since
// script output is best-effort progress reporting.
//
//nolint:errcheck // see comment above
func scriptPrintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// lineOutcome is what one script line did beyond changing the model: quit is
// set by a command that asked to exit, failed by one the app rejected or by a
// batch task any repo failed.
type lineOutcome struct {
	quit   bool
	failed bool
}

// runScriptLine executes one command line and prints its outcome.
func runScriptLine(m Model, line string, w io.Writer) (Model, lineOutcome) {
	scriptPrintf(w, "> %s\n", line)

	m, cmd := m.ExecuteCommand(strings.TrimPrefix(line, ":"))
	if cmd == nil {
		scriptPrintf(w, "  %d repos visible\n", len(m.filteredPaths))
		return m, lineOutcome{}
	}

	var outcome lineOutcome

	msg := cmd()
	switch msg := msg.(type) {
	case batch.TaskCompleteMsg:
		for _, r := range msg.Results {
			result := "ok"
			if !r.Success {
				result = "fail"
				outcome.failed = true
			}
			scriptPrintf(w, "  %s\t%s\t%s\n", result, r.RepoName, r.Message)
		}
	case StatusMsg:
		scriptPrintf(w, "  %s\n", strings.ReplaceAll(msg.Message, "\n", "\n  "))
		outcome.failed = msg.IsError
	case tea.QuitMsg:
		return m, lineOutcome{quit: true}
	}

	if newModel, _ := m.Update(msg); newModel != nil {
		if updated, ok := newModel.(Model); ok {
			m = updated
		}
	}

	return m, outcome
}
