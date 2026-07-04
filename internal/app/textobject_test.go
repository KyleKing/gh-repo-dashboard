package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func operatorModel() Model {
	m := New([]string{"/test"}, 1)
	m.loading = false
	m.repoPaths = []string{"/test/behind", "/test/clean", "/test/dirty", "/test/dirty-pr"}
	m.summaries = map[string]models.RepoSummary{
		"/test/behind":   {Path: "/test/behind", Branch: "main", Behind: 2},
		"/test/clean":    {Path: "/test/clean", Branch: "main"},
		"/test/dirty":    {Path: "/test/dirty", Branch: "main", Unstaged: 2},
		"/test/dirty-pr": {Path: "/test/dirty-pr", Branch: "feat", Unstaged: 1, PRInfo: &models.PRInfo{Number: 7}},
	}
	m.updateFilteredPaths()

	return m
}

func pressKeys(t *testing.T, m Model, keys string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, r := range keys {
		newModel, c := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = mustModel(t, newModel)
		cmd = c
	}

	return m, cmd
}

func TestResolveTextObjects(t *testing.T) {
	t.Parallel()
	m := operatorModel()

	tests := []struct {
		key      string
		expected []string
	}{
		{"ar", []string{"/test/behind", "/test/clean", "/test/dirty", "/test/dirty-pr"}},
		{"br", []string{"/test/behind"}},
		{"dr", []string{"/test/dirty", "/test/dirty-pr"}},
		{"pr", []string{"/test/dirty-pr"}},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			obj, ok := lookupTextObject(tt.key)
			if !ok {
				t.Fatalf("text object %q not found", tt.key)
			}
			paths := m.resolveTextObject(obj)
			if len(paths) != len(tt.expected) {
				t.Fatalf("resolve(%s) = %v; want %v", tt.key, paths, tt.expected)
			}
			for i, p := range tt.expected {
				if paths[i] != p {
					t.Errorf("resolve(%s)[%d] = %q; want %q", tt.key, i, paths[i], p)
				}
			}
		})
	}
}

func TestSelectedTextObject(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m.selectedPaths = map[string]bool{"/test/clean": true, "/test/dirty": true}

	obj, _ := lookupTextObject("sr")
	paths := m.resolveTextObject(obj)
	if len(paths) != 2 {
		t.Fatalf("expected 2 selected paths, got %v", paths)
	}
}

func TestOperatorComposition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		keys          string
		expectedTask  string
		expectedTotal int
	}{
		{"fetch dirty", "Fdr", "Fetch All (dirty)", 2},
		{"fetch behind", "Fbr", "Fetch All (behind)", 1},
		{"cleanup with PRs", "Cpr", "Cleanup Merged (with PRs)", 1},
		{"prune all", "Par", "Prune Remote (all)", 4},
		{"doubled operator", "FF", "Fetch All", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := operatorModel()
			m2, cmd := pressKeys(t, m, tt.keys)
			if m2.viewMode != ViewModeBatchProgress {
				t.Fatalf("expected batch progress view, got %v", m2.viewMode)
			}
			if m2.batchTask != tt.expectedTask {
				t.Errorf("expected task %q, got %q", tt.expectedTask, m2.batchTask)
			}
			if m2.batchTotal != tt.expectedTotal {
				t.Errorf("expected total %d, got %d", tt.expectedTotal, m2.batchTotal)
			}
			if cmd == nil {
				t.Error("expected batch cmd")
			}
		})
	}
}

func TestOperatorSelectedComposition(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m2, _ := m.ExecuteCommand("select where dirty")
	m3, cmd := pressKeys(t, m2, "Fsr")
	if m3.batchTask != "Fetch All (selected)" {
		t.Errorf("expected selected scope, got %q", m3.batchTask)
	}
	if m3.batchTotal != 2 {
		t.Errorf("expected 2 repos, got %d", m3.batchTotal)
	}
	if cmd == nil {
		t.Error("expected batch cmd")
	}
}

func TestOperatorEscCancels(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m2, _ := pressKeys(t, m, "F")
	if m2.pendingOperator != "F" {
		t.Fatalf("expected pending operator F, got %q", m2.pendingOperator)
	}

	newModel, _ := m2.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m3 := mustModel(t, newModel)
	if m3.pendingOperator != "" {
		t.Error("expected esc to cancel pending operator")
	}
	if m3.viewMode != ViewModeRepoList {
		t.Error("expected view unchanged after cancel")
	}
}

func TestOperatorUnknownObjectCancels(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m2, cmd := pressKeys(t, m, "Fz")
	if m2.pendingOperator != "" {
		t.Error("expected unknown object to cancel pending operator")
	}
	if cmd == nil {
		t.Fatal("expected status cmd")
	}
	if status, ok := cmd().(StatusMsg); !ok || !strings.Contains(status.Message, "z") {
		t.Errorf("expected unknown object status, got %v", cmd())
	}
}

func TestOperatorEmptyScope(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m.summaries["/test/behind"] = models.RepoSummary{Path: "/test/behind", Branch: "main"}
	m.updateFilteredPaths()

	m2, cmd := pressKeys(t, m, "Fbr")
	if m2.viewMode == ViewModeBatchProgress {
		t.Error("empty scope must not start a batch")
	}
	if cmd == nil {
		t.Fatal("expected status cmd")
	}
	if status, ok := cmd().(StatusMsg); !ok || !strings.Contains(status.Message, "behind") {
		t.Errorf("expected no-match status, got %v", cmd())
	}
}

func TestOperatorPendingFooterHint(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m.width = 100
	m.height = 30

	m2, _ := pressKeys(t, m, "F")
	if !strings.Contains(m2.view(), "pending") {
		t.Error("expected pending hint in footer")
	}
}

func TestBatchCommandWithPredicate(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m2, cmd := m.ExecuteCommand("fetch dirty and has_pr")
	if m2.batchTask != "Fetch All (dirty and has_pr)" {
		t.Errorf("expected scoped task label, got %q", m2.batchTask)
	}
	if m2.batchTotal != 1 {
		t.Errorf("expected 1 repo, got %d", m2.batchTotal)
	}
	if cmd == nil {
		t.Error("expected batch cmd")
	}
}

func TestBatchCommandPredicateNoMatch(t *testing.T) {
	t.Parallel()
	m := operatorModel()
	m2, cmd := m.ExecuteCommand("prune has_stash")
	if m2.viewMode == ViewModeBatchProgress {
		t.Error("no-match predicate must not start a batch")
	}
	if cmd == nil {
		t.Fatal("expected status cmd")
	}
}
