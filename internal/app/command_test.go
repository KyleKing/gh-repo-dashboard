package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

var (
	errBoom         = errors.New("boom")
	errGHFailed     = errors.New("gh failed")
	errPRDetailLoad = errors.New("failed to load PR details")
)

func mustModel(t *testing.T, tm tea.Model) Model {
	t.Helper()
	m, ok := tm.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}

	return m
}

func commandModel() Model {
	m := New([]string{"/test"}, 1)
	m.loading = false
	m.repoPaths = []string{"/test/clean", "/test/dirty"}
	m.summaries = map[string]models.RepoSummary{
		"/test/clean": {Path: "/test/clean", Branch: "main"},
		"/test/dirty": {Path: "/test/dirty", Branch: "main", Unstaged: 2},
	}
	m.updateFilteredPaths()

	return m
}

func TestRegistryLookup(t *testing.T) {
	registry := DefaultRegistry()

	tests := []struct {
		name     string
		query    string
		expected string
		found    bool
	}{
		{"exact match", "filter", "filter", true},
		{"unique prefix", "q", "quit", true},
		{"unique prefix multi-char", "re", "refresh", true},
		{"ambiguous prefix", "f", "", false},
		{"unknown", "bogus", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ok := registry.Lookup(tt.query)
			if ok != tt.found {
				t.Fatalf("Lookup(%q) found = %v; want %v", tt.query, ok, tt.found)
			}
			if ok && cmd.Name != tt.expected {
				t.Errorf("Lookup(%q) = %q; want %q", tt.query, cmd.Name, tt.expected)
			}
		})
	}
}

func TestRegistryCandidates(t *testing.T) {
	registry := DefaultRegistry()

	candidates := registry.Candidates("f")
	expected := []string{"fetch", "filter"}
	if len(candidates) != len(expected) {
		t.Fatalf("Candidates(\"f\") = %v; want %v", candidates, expected)
	}
	for i, name := range expected {
		if candidates[i] != name {
			t.Errorf("Candidates(\"f\")[%d] = %q; want %q", i, candidates[i], name)
		}
	}

	if got := registry.Candidates(""); len(got) != len(registry.Commands()) {
		t.Errorf("Candidates(\"\") returned %d names; want all %d", len(got), len(registry.Commands()))
	}
}

func TestExecuteCommandFilter(t *testing.T) {
	m := commandModel()

	m2, _ := m.ExecuteCommand("filter dirty")
	if m2.CurrentFilter() != models.FilterModeDirty {
		t.Errorf("expected dirty filter, got %v", m2.CurrentFilter())
	}
	if len(m2.filteredPaths) != 1 || m2.filteredPaths[0] != "/test/dirty" {
		t.Errorf("expected only dirty repo, got %v", m2.filteredPaths)
	}

	m3, _ := m2.ExecuteCommand("filter all")
	if m3.CurrentFilter() != models.FilterModeAll {
		t.Errorf("expected all filter after reset, got %v", m3.CurrentFilter())
	}
	if len(m3.filteredPaths) != 2 {
		t.Errorf("expected both repos after reset, got %v", m3.filteredPaths)
	}
}

func TestExecuteCommandFilterNoArgsOpensModal(t *testing.T) {
	m := commandModel()
	m2, _ := m.ExecuteCommand("filter")
	if m2.viewMode != ViewModeFilter {
		t.Errorf("expected ViewModeFilter, got %v", m2.viewMode)
	}
}

func TestExecuteCommandSort(t *testing.T) {
	m := commandModel()
	m2, _ := m.ExecuteCommand("sort modified")

	for _, s := range m2.activeSorts {
		if s.Mode == models.SortModeModified && s.Direction != models.SortDirectionAsc {
			t.Errorf("expected modified sort ascending, got %v", s.Direction)
		}
	}
}

func TestExecuteCommandUnknown(t *testing.T) {
	m := commandModel()
	_, cmd := m.ExecuteCommand("bogus")
	if cmd == nil {
		t.Fatal("expected status cmd for unknown command")
	}
	msg := cmd()
	status, ok := msg.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", msg)
	}
	if !strings.Contains(status.Message, "bogus") {
		t.Errorf("expected message naming the command, got %q", status.Message)
	}
}

func TestExecuteCommandUnknownFilterArg(t *testing.T) {
	m := commandModel()
	_, cmd := m.ExecuteCommand("filter bogus")
	if cmd == nil {
		t.Fatal("expected status cmd for unknown filter")
	}
	if _, ok := cmd().(StatusMsg); !ok {
		t.Fatal("expected StatusMsg for unknown filter")
	}
}

func TestExecuteCommandQuit(t *testing.T) {
	m := commandModel()
	_, cmd := m.ExecuteCommand("quit")
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestExecuteCommandEmptyLine(t *testing.T) {
	m := commandModel()
	m2, cmd := m.ExecuteCommand("   ")
	if cmd != nil {
		t.Error("expected nil cmd for empty line")
	}
	if m2.viewMode != m.viewMode {
		t.Error("expected no state change for empty line")
	}
}

func TestExecuteCommandFetch(t *testing.T) {
	m := commandModel()
	m2, cmd := m.ExecuteCommand("fetch")
	if m2.viewMode != ViewModeBatchProgress {
		t.Errorf("expected ViewModeBatchProgress, got %v", m2.viewMode)
	}
	if !m2.batchRunning || m2.batchTotal != 2 {
		t.Errorf("expected batch running over 2 repos, got running=%v total=%d", m2.batchRunning, m2.batchTotal)
	}
	if cmd == nil {
		t.Error("expected batch cmd")
	}
}

func TestCommandModeKeyFlow(t *testing.T) {
	m := commandModel()

	newModel, _ := m.Update(keyPress(':'))
	m = mustModel(t, newModel)
	if !m.commandMode {
		t.Fatal("expected command mode after ':'")
	}

	for _, r := range "filter dirty" {
		newModel, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = mustModel(t, newModel)
	}
	if m.commandInput.Value() != "filter dirty" {
		t.Fatalf("expected input %q, got %q", "filter dirty", m.commandInput.Value())
	}

	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mustModel(t, newModel)
	if m.commandMode {
		t.Error("expected command mode exit after enter")
	}
	if m.CurrentFilter() != models.FilterModeDirty {
		t.Errorf("expected dirty filter applied, got %v", m.CurrentFilter())
	}
}

func TestCommandModeEscCancels(t *testing.T) {
	m := commandModel()

	newModel, _ := m.Update(keyPress(':'))
	m = mustModel(t, newModel)
	newModel, _ = m.Update(keyPress('q'))
	m = mustModel(t, newModel)

	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mustModel(t, newModel)
	if m.commandMode {
		t.Error("expected command mode exit after esc")
	}
	if m.CurrentFilter() != models.FilterModeAll {
		t.Error("expected no command executed on esc")
	}
}

func TestCommandCompletionCyclesCommands(t *testing.T) {
	m := commandModel()

	newModel, _ := m.Update(keyPress(':'))
	m = mustModel(t, newModel)
	newModel, _ = m.Update(keyPress('f'))
	m = mustModel(t, newModel)

	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = mustModel(t, newModel)
	if m.commandInput.Value() != "fetch" {
		t.Fatalf("expected first candidate %q, got %q", "fetch", m.commandInput.Value())
	}

	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = mustModel(t, newModel)
	if m.commandInput.Value() != "filter" {
		t.Fatalf("expected second candidate %q, got %q", "filter", m.commandInput.Value())
	}

	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = mustModel(t, newModel)
	if m.commandInput.Value() != "fetch" {
		t.Errorf("expected wrap to %q, got %q", "fetch", m.commandInput.Value())
	}
}

func TestCommandCompletionArgs(t *testing.T) {
	m := commandModel()

	newModel, _ := m.Update(keyPress(':'))
	m = mustModel(t, newModel)
	for _, r := range "filter d" {
		newModel, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = mustModel(t, newModel)
	}

	newModel, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = mustModel(t, newModel)
	if m.commandInput.Value() != "filter dirty" {
		t.Errorf("expected %q, got %q", "filter dirty", m.commandInput.Value())
	}
}

func TestCommandBarRendered(t *testing.T) {
	m := commandModel()
	m.width = 80
	m.height = 24

	newModel, _ := m.Update(keyPress(':'))
	m = mustModel(t, newModel)

	output := m.view()
	lines := strings.Split(output, "\n")
	if len(lines) != m.height {
		t.Fatalf("expected %d lines, got %d", m.height, len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], ":") {
		t.Errorf("expected command prompt on last line, got %q", lines[len(lines)-1])
	}
}
