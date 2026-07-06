package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// RunScript executes ":command" lines from script against the repos under
// scanPaths, headlessly and sequentially, writing human-readable results to w.
// Blank lines and #-comments are skipped; a "quit" command stops early.
func RunScript(ctx context.Context, w io.Writer, scanPaths []string, maxDepth int, script io.Reader) error {
	m := newScriptModel(ctx, scanPaths, maxDepth)

	scanner := bufio.NewScanner(script)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var quit bool
		m, quit = runScriptLine(m, line, w) //nolint:contextcheck // tea.Model.Update has no context parameter
		if quit {
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading script: %w", err)
	}

	return nil
}

// newScriptModel builds a Model with summaries loaded synchronously, since
// script mode has no Tea event loop to deliver progressive updates.
func newScriptModel(ctx context.Context, scanPaths []string, maxDepth int) Model {
	m := New(scanPaths, maxDepth)
	m.loading = false
	m.repoPaths = discovery.DiscoverRepos(scanPaths, maxDepth)

	for _, path := range m.repoPaths {
		ops := vcs.GetOperations(path)
		summary, err := ops.GetRepoSummary(ctx, path)
		if err != nil {
			summary = models.RepoSummary{Path: path, VCSType: vcs.DetectVCSType(path), Error: err}
		} else {
			summary.NotesFile, summary.NotesFirstLine = models.DetectNotes(path)
		}
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

// runScriptLine executes one command line and prints its outcome; quit is
// true when the command asked to exit.
func runScriptLine(m Model, line string, w io.Writer) (Model, bool) {
	scriptPrintf(w, "> %s\n", line)

	m, cmd := m.ExecuteCommand(strings.TrimPrefix(line, ":"))
	if cmd == nil {
		scriptPrintf(w, "  %d repos visible\n", len(m.filteredPaths))
		return m, false
	}

	msg := cmd()
	switch msg := msg.(type) {
	case batch.TaskCompleteMsg:
		for _, r := range msg.Results {
			outcome := "ok"
			if !r.Success {
				outcome = "fail"
			}
			scriptPrintf(w, "  %s\t%s\t%s\n", outcome, r.RepoName, r.Message)
		}
	case StatusMsg:
		scriptPrintf(w, "  %s\n", msg.Message)
	case tea.QuitMsg:
		return m, true
	}

	if newModel, _ := m.Update(msg); newModel != nil {
		if updated, ok := newModel.(Model); ok {
			m = updated
		}
	}

	return m, false
}
