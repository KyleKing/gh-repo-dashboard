# Bubbletea Visual & Integration Testing Improvements

This document compares three testing approaches for Bubbletea apps and identifies gaps in the current gh-repo-dashboard test suite based on patterns from gh-lazydispatch.

## Testing Library Comparison

### 1. teatest (Golden File Testing)

**Best for:** Visual regression, ensuring UI renders correctly, catching unintended rendering changes.

**Strengths:**
- Automated snapshot comparisons
- Catches visual regressions
- Good for testing View() output
- Works well with CI pipelines

**Weaknesses:**
- Snapshots can be brittle with terminal size variations
- Requires manual review when updating
- Not ideal for testing state transitions

**Demo Test:** `internal/app/app_golden_test.go`

```go
//go:build golden

package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// TestRepoListView_Golden tests the main repo list view rendering
// using golden file comparison. Run with: go test -tags=golden ./...
// Update snapshots with: go test -tags=golden -update ./...
func TestRepoListView_Golden(t *testing.T) {
	m := New([]string{"/repos"}, 2)
	m.width = 120
	m.height = 40
	m.loading = false

	// Simulate loaded repos
	m.repoPaths = []string{"/repos/api", "/repos/web", "/repos/lib"}
	m.filteredPaths = m.repoPaths
	m.summaries = map[string]models.RepoSummary{
		"/repos/api": {
			Path:     "/repos/api",
			Branch:   "main",
			Ahead:    2,
			Staged:   1,
			VCSType:  models.VCSTypeGit,
			Modified: time.Now().Add(-2 * time.Hour).Unix(),
		},
		"/repos/web": {
			Path:     "/repos/web",
			Branch:   "feature/auth",
			Behind:   3,
			VCSType:  models.VCSTypeGit,
			Modified: time.Now().Add(-1 * time.Hour).Unix(),
			PRInfo: &models.PRInfo{
				Number: 42,
				Title:  "Add authentication",
				State:  "OPEN",
			},
		},
		"/repos/lib": {
			Path:     "/repos/lib",
			Branch:   "develop",
			VCSType:  models.VCSTypeJJ,
			Modified: time.Now().Add(-24 * time.Hour).Unix(),
		},
	}

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))

	// Wait for any initialization
	time.Sleep(100 * time.Millisecond)

	// Get final output and compare against golden file
	out := tm.FinalOutput(t)
	teatest.RequireEqualOutput(t, out)
}

// TestFilterModal_Golden tests the filter modal rendering
func TestFilterModal_Golden(t *testing.T) {
	m := New(nil, 1)
	m.width = 80
	m.height = 30
	m.viewMode = ViewModeFilter
	m.loading = false

	// Add some repos for count display
	m.summaries = map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Staged: 1},
		"/repo2": {Path: "/repo2", Ahead: 2},
		"/repo3": {Path: "/repo3"},
	}

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 30))
	out := tm.FinalOutput(t)
	teatest.RequireEqualOutput(t, out)
}

// TestBatchProgress_Golden tests the batch progress view
func TestBatchProgress_Golden(t *testing.T) {
	m := New(nil, 1)
	m.width = 100
	m.height = 25
	m.viewMode = ViewModeBatchProgress
	m.batchTask = "Fetch All"
	m.batchTotal = 5
	m.batchProgress = 3
	m.batchRunning = false
	m.batchResults = []BatchResult{
		{Path: "/repos/api", Success: true, Message: "Fetched successfully"},
		{Path: "/repos/web", Success: true, Message: "Already up to date"},
		{Path: "/repos/lib", Success: false, Message: "Remote not found"},
	}

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 25))
	out := tm.FinalOutput(t)
	teatest.RequireEqualOutput(t, out)
}
```

---

### 2. catwalk (Data-Driven Testing)

**Best for:** Complex interaction sequences, testing state machines, verifiable input/output pairs.

**Strengths:**
- Data files separate test logic from test data
- Easy to add new test cases
- Good for testing complex multi-step interactions
- Reference output updated with `-rewrite`

**Weaknesses:**
- Requires learning datadriven syntax
- Data files can become complex
- Less intuitive than direct Go code

**Demo Test:** `internal/app/testdata/filter_cycle.txt` and `internal/app/app_catwalk_test.go`

```
# testdata/filter_cycle.txt
# Tests filter cycling through disabled -> enabled -> inverted -> disabled

init
width=80 height=30 mode=filter
----

# Initial state: FilterModeAll enabled, others disabled
view
----
Filter Repositories

  check  Key  Filter           Count
>  ✓     >    Ahead            0
   ...

# Press enter to enable Ahead filter
key enter
----
state: Ahead enabled, not inverted

# Press enter again to invert
key enter
----
state: Ahead enabled, inverted (NOT Ahead)

# Press enter again to disable
key enter
----
state: Ahead disabled

# Press * to reset all filters
key *
----
state: all filters reset to default
```

```go
//go:build catwalk

package app

import (
	"testing"

	"github.com/cockroachdb/datadriven"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/knz/catwalk"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestFilterCycling_Catwalk(t *testing.T) {
	datadriven.Walk(t, "testdata", func(t *testing.T, path string) {
		m := New(nil, 1)
		m.width = 80
		m.height = 30
		m.viewMode = ViewModeFilter

		catwalk.RunModel(t, path, &m,
			catwalk.WithUpdater(func(m tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
				return m.(Model).Update(msg)
			}),
			catwalk.WithViewer(func(m tea.Model) string {
				return m.(Model).View()
			}),
		)
	})
}
```

---

### 3. Direct Testing (gh-lazydispatch Style)

**Best for:** State transitions, business logic, command verification, fast feedback.

**Strengths:**
- No external dependencies
- Fast execution
- Clear Go code
- Easy to debug
- Tests logic without visual coupling

**Weaknesses:**
- No visual regression detection
- Manual assertions for each state change
- Can miss rendering bugs

**Demo Test:** `internal/app/app_update_test.go`

```go
package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// TestUpdate_ReposDiscovered tests the full repo discovery flow
func TestUpdate_ReposDiscovered(t *testing.T) {
	m := New([]string{"/test"}, 2)

	// Simulate discovering repos
	msg := ReposDiscoveredMsg{Paths: []string{"/repo1", "/repo2", "/repo3"}}
	result, cmd := m.Update(msg)
	m = result.(Model)

	// Verify state changes
	if len(m.repoPaths) != 3 {
		t.Errorf("expected 3 repoPaths, got %d", len(m.repoPaths))
	}
	if m.loadingCount != 3 {
		t.Errorf("expected loadingCount=3, got %d", m.loadingCount)
	}
	if m.loadedCount != 0 {
		t.Errorf("expected loadedCount=0, got %d", m.loadedCount)
	}

	// Verify batch commands were returned (one per repo)
	if cmd == nil {
		t.Fatal("expected batch command for loading summaries")
	}
}

// TestUpdate_RepoSummaryLoaded tests progressive loading completion
func TestUpdate_RepoSummaryLoaded(t *testing.T) {
	m := New(nil, 1)
	m.loadingCount = 2
	m.loadedCount = 0
	m.repoPaths = []string{"/repo1", "/repo2"}
	m.loading = true

	// First repo loads
	msg1 := RepoSummaryLoadedMsg{
		Path: "/repo1",
		Summary: models.RepoSummary{
			Path:     "/repo1",
			Branch:   "main",
			Upstream: "origin/main",
		},
	}
	result, _ := m.Update(msg1)
	m = result.(Model)

	if m.loadedCount != 1 {
		t.Errorf("expected loadedCount=1, got %d", m.loadedCount)
	}
	if !m.loading {
		t.Error("should still be loading")
	}
	if _, ok := m.summaries["/repo1"]; !ok {
		t.Error("summary for /repo1 should be stored")
	}

	// Second repo loads - should complete loading
	msg2 := RepoSummaryLoadedMsg{
		Path:    "/repo2",
		Summary: models.RepoSummary{Path: "/repo2", Branch: "develop"},
	}
	result, _ = m.Update(msg2)
	m = result.(Model)

	if m.loadedCount != 2 {
		t.Errorf("expected loadedCount=2, got %d", m.loadedCount)
	}
	if m.loading {
		t.Error("should no longer be loading")
	}
}

// TestUpdate_BatchTaskProgress tests batch operation progress updates
func TestUpdate_BatchTaskProgress(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeBatchProgress
	m.batchRunning = true
	m.batchTotal = 3

	// Receive progress updates
	progressMsgs := []batch.TaskProgressMsg{
		{Result: batch.TaskResult{Path: "/repo1", Success: true, Message: "OK"}},
		{Result: batch.TaskResult{Path: "/repo2", Success: false, Message: "Error"}},
	}

	for i, msg := range progressMsgs {
		result, _ := m.Update(msg)
		m = result.(Model)

		if m.batchProgress != i+1 {
			t.Errorf("expected batchProgress=%d, got %d", i+1, m.batchProgress)
		}
		if len(m.batchResults) != i+1 {
			t.Errorf("expected %d results, got %d", i+1, len(m.batchResults))
		}
	}

	// Verify result details
	if !m.batchResults[0].Success {
		t.Error("first result should be success")
	}
	if m.batchResults[1].Success {
		t.Error("second result should be failure")
	}
}

// TestUpdate_NavigationKeys tests cursor movement in repo list
func TestUpdate_NavigationKeys(t *testing.T) {
	m := New(nil, 1)
	m.filteredPaths = []string{"/repo1", "/repo2", "/repo3"}
	m.cursor = 0

	tests := []struct {
		name       string
		key        tea.KeyType
		wantCursor int
	}{
		{"down from top", tea.KeyDown, 1},
		{"down again", tea.KeyDown, 2},
		{"down at bottom stays", tea.KeyDown, 2},
		{"up from bottom", tea.KeyUp, 1},
		{"up again", tea.KeyUp, 0},
		{"up at top stays", tea.KeyUp, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tea.KeyMsg{Type: tt.key}
			result, _ := m.Update(msg)
			m = result.(Model)

			if m.cursor != tt.wantCursor {
				t.Errorf("expected cursor=%d, got %d", tt.wantCursor, m.cursor)
			}
		})
	}
}

// TestUpdate_ViewModeTransitions tests view mode state machine
func TestUpdate_ViewModeTransitions(t *testing.T) {
	tests := []struct {
		name      string
		initial   ViewMode
		key       string
		expected  ViewMode
		setup     func(*Model)
	}{
		{
			name:     "repo list to filter",
			initial:  ViewModeRepoList,
			key:      "f",
			expected: ViewModeFilter,
		},
		{
			name:     "repo list to sort",
			initial:  ViewModeRepoList,
			key:      "s",
			expected: ViewModeSort,
		},
		{
			name:     "repo list to help",
			initial:  ViewModeRepoList,
			key:      "?",
			expected: ViewModeHelp,
		},
		{
			name:     "filter back to repo list",
			initial:  ViewModeFilter,
			key:      "esc",
			expected: ViewModeRepoList,
		},
		{
			name:     "repo list to detail",
			initial:  ViewModeRepoList,
			key:      "enter",
			expected: ViewModeRepoDetail,
			setup: func(m *Model) {
				m.filteredPaths = []string{"/repo1"}
				m.summaries = map[string]models.RepoSummary{
					"/repo1": {Path: "/repo1"},
				}
			},
		},
		{
			name:     "detail back to list",
			initial:  ViewModeRepoDetail,
			key:      "esc",
			expected: ViewModeRepoList,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(nil, 1)
			m.viewMode = tt.initial
			if tt.setup != nil {
				tt.setup(&m)
			}

			var msg tea.KeyMsg
			if tt.key == "esc" {
				msg = tea.KeyMsg{Type: tea.KeyEscape}
			} else if tt.key == "enter" {
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			} else {
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			result, _ := m.Update(msg)
			m = result.(Model)

			if m.viewMode != tt.expected {
				t.Errorf("expected viewMode=%d, got %d", tt.expected, m.viewMode)
			}
		})
	}
}

// TestUpdate_SearchMode tests search activation and deactivation
func TestUpdate_SearchMode(t *testing.T) {
	m := New(nil, 1)
	m.filteredPaths = []string{"/api", "/web", "/lib"}

	// Activate search with /
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	result, _ := m.Update(msg)
	m = result.(Model)

	if !m.searching {
		t.Error("expected searching=true after /")
	}

	// Type search text
	for _, r := range "api" {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		result, _ := m.Update(msg)
		m = result.(Model)
	}

	if m.searchText != "api" {
		t.Errorf("expected searchText='api', got %q", m.searchText)
	}

	// Confirm search with Enter
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	result, _ = m.Update(msg)
	m = result.(Model)

	if m.searching {
		t.Error("expected searching=false after Enter")
	}
	if m.searchText != "api" {
		t.Errorf("search text should persist: %q", m.searchText)
	}
}

// TestHandleDetailKey_TabCycling tests tab cycling in detail view
func TestHandleDetailKey_TabCycling(t *testing.T) {
	m := New(nil, 1)
	m.viewMode = ViewModeRepoDetail
	m.selectedRepo = "/repo1"
	m.summaries = map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1"},
	}

	tabs := []DetailTab{
		DetailTabBranches,
		DetailTabStashes,
		DetailTabWorktrees,
		DetailTabPRs,
		DetailTabBranches, // wraps
	}

	for i, expected := range tabs[1:] {
		msg := tea.KeyMsg{Type: tea.KeyTab}
		result, _ := m.Update(msg)
		m = result.(Model)

		if m.detailTab != expected {
			t.Errorf("step %d: expected tab=%d, got %d", i, expected, m.detailTab)
		}
		if m.detailCursor != 0 {
			t.Errorf("cursor should reset to 0 on tab change")
		}
	}
}
```

---

## Missing Testing Patterns from gh-lazydispatch

Based on analysis of gh-lazydispatch's comprehensive test suite, the following patterns are missing or underutilized in gh-repo-dashboard:

### 1. Message Handling Tests

**Gap:** gh-repo-dashboard lacks systematic tests for how `Update()` handles each message type.

**gh-lazydispatch pattern:**
```go
func TestHandleSelectResult(t *testing.T) {
    m := New(testWorkflows(), testHistory(), "owner/repo")
    m.pendingInputName = "environment"

    result, _ := m.handleSelectResult(modal.SelectResultMsg{Value: "production"})
    m = result.(Model)

    if m.inputs["environment"] != "production" {
        t.Errorf("expected environment=production, got %q", m.inputs["environment"])
    }
}
```

**Recommended addition:** `internal/app/messages_test.go`
```go
func TestPRLoadedMsg_UpdatesSummary(t *testing.T) {
    m := New(nil, 1)
    m.summaries = map[string]models.RepoSummary{
        "/repo1": {Path: "/repo1", Branch: "main"},
    }

    msg := PRLoadedMsg{
        Path: "/repo1",
        PRInfo: &models.PRInfo{Number: 42, Title: "Feature"},
    }
    result, _ := m.Update(msg)
    m = result.(Model)

    summary := m.summaries["/repo1"]
    if summary.PRInfo == nil {
        t.Fatal("PRInfo should be set")
    }
    if summary.PRInfo.Number != 42 {
        t.Errorf("expected PR #42, got #%d", summary.PRInfo.Number)
    }
}

func TestDetailLoadedMsg_OnlyUpdatesSelectedRepo(t *testing.T) {
    m := New(nil, 1)
    m.selectedRepo = "/repo1"

    // Message for different repo should be ignored
    msg := DetailLoadedMsg{
        Path:     "/repo2",
        Branches: []models.BranchInfo{{Name: "main"}},
    }
    result, _ := m.Update(msg)
    m = result.(Model)

    if len(m.branches) != 0 {
        t.Error("branches should not update for non-selected repo")
    }
}
```

### 2. Command Return Testing

**Gap:** gh-repo-dashboard doesn't verify that actions return the expected commands.

**gh-lazydispatch pattern:**
```go
func TestUpdate_Tab(t *testing.T) {
    m := New(testWorkflows(), testHistory(), "owner/repo")
    msg := tea.KeyMsg{Type: tea.KeyTab}
    result, cmd := m.Update(msg)
    // They track the cmd return value
}
```

**Recommended addition:**
```go
func TestEnterRepoDetail_ReturnsLoadCmd(t *testing.T) {
    m := New(nil, 1)
    m.filteredPaths = []string{"/repo1"}
    m.summaries = map[string]models.RepoSummary{
        "/repo1": {Path: "/repo1"},
    }

    msg := tea.KeyMsg{Type: tea.KeyEnter}
    _, cmd := m.Update(msg)

    if cmd == nil {
        t.Fatal("expected loadDetailCmd to be returned")
    }

    // Execute the command and verify the message type
    resultMsg := cmd()
    if _, ok := resultMsg.(DetailLoadedMsg); !ok {
        t.Errorf("expected DetailLoadedMsg, got %T", resultMsg)
    }
}
```

### 3. Integration Tests with Mocks

**Gap:** gh-repo-dashboard lacks end-to-end tests with mocked VCS/GitHub operations.

**gh-lazydispatch pattern:** `internal/integration_test.go`
```go
func TestEndToEnd_ChainExecutionWithLogs(t *testing.T) {
    mockExec := exec.NewMockExecutor()
    setupChainExecutionMocks(mockExec)
    runner.SetExecutor(mockExec)
    defer runner.SetExecutor(nil)
    // ... full flow test
}
```

**Recommended addition:** `internal/app/integration_test.go`
```go
func TestEndToEnd_DiscoverAndLoadRepos(t *testing.T) {
    // Setup mock VCS operations
    mockVCS := &vcs.MockOperations{
        GetRepoSummaryFn: func(ctx context.Context, path string) (models.RepoSummary, error) {
            return models.RepoSummary{
                Path:   path,
                Branch: "main",
                Ahead:  2,
            }, nil
        },
    }
    // Inject mock
    vcs.SetMockOperations(mockVCS)
    defer vcs.ClearMockOperations()

    m := New([]string{t.TempDir()}, 1)
    // Create test repos
    // ... run through discovery -> load -> display flow
}
```

### 4. View Component Tests

**Gap:** gh-repo-dashboard doesn't test individual view components in isolation.

**gh-lazydispatch pattern:** `internal/ui/panes/panes_test.go`
```go
func TestWorkflowModel_SelectedWorkflow(t *testing.T) {
    m := NewWorkflowModel(testWorkflows())
    m.SetSize(40, 20)

    wf := m.SelectedWorkflow()
    if wf == nil {
        t.Fatal("expected non-nil workflow")
    }
}
```

**Recommended addition:** Extract and test view helpers
```go
func TestRenderStatusBar(t *testing.T) {
    m := New(nil, 1)

    // Enable a filter
    m.CycleFilterState(models.FilterModeAhead)

    // Enable a sort
    m.CycleSortState(models.SortModeModified)

    // Set search text
    m.searchText = "api"

    bar := m.renderStatusBar()

    if !strings.Contains(bar, "Ahead") {
        t.Error("status bar should show Ahead filter")
    }
    if !strings.Contains(bar, "Modified") {
        t.Error("status bar should show Modified sort")
    }
    if !strings.Contains(bar, "api") {
        t.Error("status bar should show search text")
    }
}

func TestRenderTableRow_Styles(t *testing.T) {
    tests := []struct {
        name     string
        summary  models.RepoSummary
        selected bool
        contains []string
    }{
        {
            name:     "dirty repo shows status",
            summary:  models.RepoSummary{Path: "/repo", Staged: 2, Unstaged: 1},
            contains: []string{"+2", "~1"},
        },
        {
            name:     "repo with PR shows number",
            summary:  models.RepoSummary{Path: "/repo", PRInfo: &models.PRInfo{Number: 42}},
            contains: []string{"#42"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := New(nil, 1)
            m.width = 120

            colWidths := struct {
                name, branch, status, pr, prs, modified int
            }{20, 15, 12, 12, 6, 12}

            row := m.renderTableRow(tt.summary, tt.selected, colWidths)

            for _, want := range tt.contains {
                if !strings.Contains(row, want) {
                    t.Errorf("row should contain %q: %s", want, row)
                }
            }
        })
    }
}
```

### 5. Boundary Condition Tests

**Gap:** Edge cases like empty lists, max cursor positions, window size changes.

**gh-lazydispatch pattern:**
```go
func TestConfigModel_SelectUpDown_Boundaries(t *testing.T) {
    m := NewConfigModel()
    m.SetSize(80, 30)
    m.selectedRow = -1

    m.SelectDown()
    if m.selectedRow != 0 {
        t.Errorf("expected selectedRow = 0 after first SelectDown")
    }

    m.SelectUp()
    if m.selectedRow != 0 {
        t.Errorf("expected selectedRow = 0 at top boundary")
    }
}
```

**Recommended addition:**
```go
func TestCursorBoundaries(t *testing.T) {
    tests := []struct {
        name          string
        paths         []string
        initialCursor int
        action        func(*Model)
        wantCursor    int
    }{
        {
            name:          "empty list cursor stays 0",
            paths:         []string{},
            initialCursor: 0,
            action:        func(m *Model) { m.cursor++ },
            wantCursor:    0,
        },
        {
            name:          "cursor clamps to list end on filter",
            paths:         []string{"/a", "/b"},
            initialCursor: 5,
            action:        func(m *Model) { m.updateFilteredPaths() },
            wantCursor:    1,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := New(nil, 1)
            m.filteredPaths = tt.paths
            m.cursor = tt.initialCursor

            tt.action(&m)

            if m.cursor != tt.wantCursor {
                t.Errorf("expected cursor=%d, got %d", tt.wantCursor, m.cursor)
            }
        })
    }
}
```

### 6. Time-Based Behavior Tests

**Gap:** Testing relative time formatting, cache expiration behavior.

**gh-lazydispatch pattern:**
```go
func TestFormatTimeAgo(t *testing.T) {
    now := time.Now()
    tests := []struct {
        name     string
        timeAgo  time.Duration
        expected string
    }{
        {"just now", 30 * time.Second, "just now"},
        {"5 minutes ago", 5 * time.Minute, "5m ago"},
    }
    // ...
}
```

**Recommended addition:** Already partially covered in `models/repo_test.go`, but should be comprehensive.

---

## Recommended Test File Structure

```
internal/
├── app/
│   ├── app_test.go           # Basic model tests (existing)
│   ├── app_update_test.go    # Update() message handling tests (NEW)
│   ├── app_view_test.go      # View component tests (NEW)
│   ├── app_golden_test.go    # Golden file tests (NEW, build tag: golden)
│   ├── integration_test.go   # End-to-end with mocks (NEW)
│   └── testdata/
│       └── *.golden          # Golden file snapshots
├── vcs/
│   └── mock.go               # Already exists, good!
└── testutil/
    ├── testutil.go           # Test helpers (NEW)
    ├── fixtures.go           # Test data factories (NEW)
    └── mocks.go              # Additional mocks (NEW)
```

## Quick Wins

1. **Add message handling tests** - High value, low effort
2. **Add view mode transition tests** - Documents expected behavior
3. **Add boundary condition tests** - Catches edge case bugs
4. **Create test fixtures package** - Reduces test boilerplate

## Long-term Improvements

1. **Add teatest golden file tests** for visual regression
2. **Create integration test suite** with full mocking
3. **Add benchmark tests** for filtering/sorting large repo lists
4. **Consider catwalk** for complex interaction sequences
