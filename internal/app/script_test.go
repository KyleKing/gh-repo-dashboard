//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
)

func TestRunScriptLineFilter(t *testing.T) {
	t.Parallel()
	m := operatorModel()

	var out strings.Builder
	m2, quit := runScriptLine(m, ":filter dirty", &out)
	if quit {
		t.Fatal("filter must not quit")
	}
	if len(m2.filteredPaths) != 2 {
		t.Errorf("filtered = %v; want 2 dirty repos", m2.filteredPaths)
	}
	if !strings.Contains(out.String(), "2 repos visible") {
		t.Errorf("output = %q; want visible count", out.String())
	}
}

func TestRunScriptLineStatusMessage(t *testing.T) {
	t.Parallel()
	m := operatorModel()

	var out strings.Builder
	_, quit := runScriptLine(m, "select where has_pr", &out)
	if quit {
		t.Fatal("select must not quit")
	}
	if !strings.Contains(out.String(), "Selected 1 repos") {
		t.Errorf("output = %q; want selection status", out.String())
	}
}

func TestRunScriptLineUnknownCommand(t *testing.T) {
	t.Parallel()
	m := operatorModel()

	var out strings.Builder
	_, quit := runScriptLine(m, "bogus", &out)
	if quit {
		t.Fatal("unknown command must not quit")
	}
	if !strings.Contains(out.String(), "Unknown command: bogus") {
		t.Errorf("output = %q; want unknown-command message", out.String())
	}
}

func TestRunScriptLineQuit(t *testing.T) {
	t.Parallel()
	m := operatorModel()

	var out strings.Builder
	if _, quit := runScriptLine(m, "quit", &out); !quit {
		t.Error("quit command must stop the script")
	}
}

func TestRunScriptSkipsCommentsAndBlanks(t *testing.T) {
	t.Parallel()
	m := operatorModel()

	var out strings.Builder
	script := "# comment\n\n:filter dirty\nquit\n:filter all\n"
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var quit bool
		m, quit = runScriptLine(m, line, &out)
		if quit {
			break
		}
	}

	if strings.Contains(out.String(), "filter all") {
		t.Errorf("output = %q; quit must stop before 'filter all'", out.String())
	}
	if len(m.filteredPaths) != 2 {
		t.Errorf("filter dirty should have applied before quit, got %v", m.filteredPaths)
	}
}
