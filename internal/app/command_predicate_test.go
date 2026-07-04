package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func predicateModel() Model {
	m := New([]string{"/test"}, 1)
	m.loading = false
	m.repoPaths = []string{"/test/clean", "/test/dirty", "/test/dirty-pr"}
	m.summaries = map[string]models.RepoSummary{
		"/test/clean":    {Path: "/test/clean", Branch: "main"},
		"/test/dirty":    {Path: "/test/dirty", Branch: "main", Unstaged: 2},
		"/test/dirty-pr": {Path: "/test/dirty-pr", Branch: "feat", Unstaged: 1, PRInfo: &models.PRInfo{Number: 7}},
	}
	m.updateFilteredPaths()

	return m
}

func TestFilterCommandPredicate(t *testing.T) {
	t.Parallel()
	m := predicateModel()

	m2, _ := m.ExecuteCommand("filter dirty and has_pr")
	if len(m2.filteredPaths) != 1 || m2.filteredPaths[0] != "/test/dirty-pr" {
		t.Errorf("expected only dirty-pr, got %v", m2.filteredPaths)
	}
	if m2.predicateText != "dirty and has_pr" {
		t.Errorf("expected predicate text recorded, got %q", m2.predicateText)
	}

	m3, _ := m2.ExecuteCommand("filter all")
	if len(m3.filteredPaths) != 3 {
		t.Errorf("expected reset to all repos, got %v", m3.filteredPaths)
	}
	if m3.predicateText != "" {
		t.Errorf("expected predicate cleared, got %q", m3.predicateText)
	}
}

func TestFilterCommandPredicateOr(t *testing.T) {
	t.Parallel()
	m := predicateModel()
	m2, _ := m.ExecuteCommand("filter clean or has_pr")
	if len(m2.filteredPaths) != 2 {
		t.Errorf("expected clean and dirty-pr, got %v", m2.filteredPaths)
	}
}

func TestFilterCommandPredicateParseError(t *testing.T) {
	t.Parallel()
	m := predicateModel()
	m2, cmd := m.ExecuteCommand("filter dirty and")
	if cmd == nil {
		t.Fatal("expected status cmd for parse error")
	}
	status, ok := cmd().(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", cmd())
	}
	if !strings.Contains(status.Message, "dirty and") {
		t.Errorf("expected message citing expression, got %q", status.Message)
	}
	if len(m2.filteredPaths) != 3 {
		t.Errorf("expected filter unchanged on error, got %v", m2.filteredPaths)
	}
}

func TestFilterCommandLegacyModeStillWorks(t *testing.T) {
	t.Parallel()
	m := predicateModel()
	m2, _ := m.ExecuteCommand("filter dirty")
	if m2.CurrentFilter() != models.FilterModeDirty {
		t.Errorf("expected legacy dirty mode, got %v", m2.CurrentFilter())
	}
	if m2.predicateText != "" {
		t.Errorf("single legacy mode should not set predicate, got %q", m2.predicateText)
	}
	if len(m2.filteredPaths) != 2 {
		t.Errorf("expected two dirty repos, got %v", m2.filteredPaths)
	}
}

func TestSelectWhere(t *testing.T) {
	t.Parallel()
	m := predicateModel()

	m2, cmd := m.ExecuteCommand("select where dirty and has_pr")
	if m2.SelectedCount() != 1 || !m2.selectedPaths["/test/dirty-pr"] {
		t.Errorf("expected dirty-pr selected, got %v", m2.selectedPaths)
	}
	if cmd == nil {
		t.Fatal("expected status cmd reporting selection count")
	}
	if status, ok := cmd().(StatusMsg); !ok || !strings.Contains(status.Message, "1") {
		t.Errorf("expected count in status, got %v", cmd())
	}

	if len(m2.filteredPaths) != 3 {
		t.Errorf("select must not filter the list, got %v", m2.filteredPaths)
	}
}

func TestSelectAllAndNone(t *testing.T) {
	t.Parallel()
	m := predicateModel()

	m2, _ := m.ExecuteCommand("select all")
	if m2.SelectedCount() != 3 {
		t.Errorf("expected 3 selected, got %d", m2.SelectedCount())
	}

	m3, _ := m2.ExecuteCommand("select none")
	if m3.SelectedCount() != 0 {
		t.Errorf("expected none selected, got %d", m3.SelectedCount())
	}
}

func TestSelectUsage(t *testing.T) {
	t.Parallel()
	m := predicateModel()
	_, cmd := m.ExecuteCommand("select")
	if cmd == nil {
		t.Fatal("expected usage status cmd")
	}
	if _, ok := cmd().(StatusMsg); !ok {
		t.Fatal("expected StatusMsg usage")
	}
}

func TestSelectionMarkerRendered(t *testing.T) {
	t.Parallel()
	m := predicateModel()
	m.width = 100
	m.height = 30

	m2, _ := m.ExecuteCommand("select where has_pr")
	output := m2.view()
	if !strings.Contains(output, "•") {
		t.Error("expected selection marker in view")
	}
	if !strings.Contains(output, "1 selected") {
		t.Error("expected selection badge in status bar")
	}
}

func TestPredicateBadgeRendered(t *testing.T) {
	t.Parallel()
	m := predicateModel()
	m.width = 100
	m.height = 30

	m2, _ := m.ExecuteCommand("filter dirty and has_pr")
	if !strings.Contains(m2.view(), "dirty and has_pr") {
		t.Error("expected predicate badge in status bar")
	}
}
