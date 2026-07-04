package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

var (
	errFixtureBadWhen        = errors.New("when must be ':command' or 'keys ...'")
	errFixtureThenBeforeWhen = errors.New("then before any when")
	errFixtureBadThen        = errors.New("then requires 'field = value'")
	errFixtureUnrecognized   = errors.New("unrecognized line")
	errFixtureMissingGiven   = errors.New("missing given")
	errFixtureNoSteps        = errors.New("no steps")
)

type fixtureStep struct {
	Input      string
	IsCommand  bool
	Keys       []string
	Assertions []fixtureAssertion
}

type fixtureAssertion struct {
	Field string
	Value string
}

type fixture struct {
	Path  string
	Doc   string
	Given string
	Steps []fixtureStep
}

func parseFixture(path string) (fixture, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from a glob over our own testdata dir
	if err != nil {
		return fixture{}, fmt.Errorf("reading fixture %s: %w", path, err)
	}

	f := fixture{Path: path}
	for lineNo, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		switch {
		case strings.HasPrefix(line, "doc:"):
			f.Doc = strings.TrimSpace(strings.TrimPrefix(line, "doc:"))

		case strings.HasPrefix(line, "given "):
			f.Given = strings.TrimSpace(strings.TrimPrefix(line, "given "))

		case strings.HasPrefix(line, "when "):
			input := strings.TrimSpace(strings.TrimPrefix(line, "when "))
			step := fixtureStep{Input: input}
			switch {
			case strings.HasPrefix(input, ":"):
				step.IsCommand = true
			case strings.HasPrefix(input, "keys "):
				step.Keys = strings.Fields(strings.TrimPrefix(input, "keys "))
			default:
				return fixture{}, fmt.Errorf("%s:%d: %w", path, lineNo+1, errFixtureBadWhen)
			}
			f.Steps = append(f.Steps, step)

		case strings.HasPrefix(line, "then "):
			if len(f.Steps) == 0 {
				return fixture{}, fmt.Errorf("%s:%d: %w", path, lineNo+1, errFixtureThenBeforeWhen)
			}
			field, value, found := strings.Cut(strings.TrimPrefix(line, "then "), "=")
			if !found {
				return fixture{}, fmt.Errorf("%s:%d: %w", path, lineNo+1, errFixtureBadThen)
			}
			last := &f.Steps[len(f.Steps)-1]
			last.Assertions = append(last.Assertions, fixtureAssertion{
				Field: strings.TrimSpace(field),
				Value: strings.TrimSpace(value),
			})

		default:
			return fixture{}, fmt.Errorf("%s:%d: %w: %q", path, lineNo+1, errFixtureUnrecognized, line)
		}
	}

	if f.Given == "" {
		return fixture{}, fmt.Errorf("%s: %w", path, errFixtureMissingGiven)
	}
	if len(f.Steps) == 0 {
		return fixture{}, fmt.Errorf("%s: %w", path, errFixtureNoSteps)
	}

	return f, nil
}

func fixtureDataset(t *testing.T, name string) Model {
	t.Helper()
	if name != "standard" {
		t.Fatalf("unknown dataset %q", name)
	}

	m := New([]string{"/repos"}, 1)
	m.width = 100
	m.height = 30
	m.loading = false
	m.repoPaths = []string{"/repos/behind", "/repos/clean", "/repos/dirty", "/repos/dirty-pr"}
	m.summaries = map[string]models.RepoSummary{
		"/repos/behind":   {Path: "/repos/behind", Branch: "main", Behind: 2},
		"/repos/clean":    {Path: "/repos/clean", Branch: "main"},
		"/repos/dirty":    {Path: "/repos/dirty", Branch: "main", Unstaged: 2},
		"/repos/dirty-pr": {Path: "/repos/dirty-pr", Branch: "feat", Unstaged: 1, PRInfo: &models.PRInfo{Number: 7}},
	}
	m.updateFilteredPaths()

	return m
}

func fixtureKeyMsg(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	default:
		r := []rune(name)[0]
		return tea.KeyPressMsg{Code: r, Text: string(r)}
	}
}

// runFixtureStep applies one step; the returned cmd is executed only when a
// status assertion needs the resulting message (batch cmds shell out).
func runFixtureStep(t *testing.T, m Model, step fixtureStep) Model {
	t.Helper()

	var cmd tea.Cmd
	if step.IsCommand {
		m, cmd = m.ExecuteCommand(strings.TrimPrefix(step.Input, ":"))
	} else {
		for _, keyName := range step.Keys {
			newModel, c := m.Update(fixtureKeyMsg(keyName))
			m = mustModel(t, newModel)
			cmd = c
		}
	}

	for _, assertion := range step.Assertions {
		if assertion.Field == "status" && cmd != nil {
			if msg := cmd(); msg != nil {
				newModel, _ := m.Update(msg)
				m = mustModel(t, newModel)
			}
			cmd = nil
		}
	}

	snap := m.Snapshot()
	for _, assertion := range step.Assertions {
		var got string
		switch assertion.Field {
		case "cursor":
			got = strconv.Itoa(snap.Cursor)
		case "filtered":
			got = joinOrNone(snap.Filtered)
		case "input":
			got = snap.CommandInput
		case "predicate":
			got = snap.Predicate
		case "search":
			got = snap.Search
		case "selected":
			got = joinOrNone(snap.Selected)
		case "status":
			got = snap.StatusMessage
		case "task":
			got = snap.BatchTask
		case "total":
			got = strconv.Itoa(snap.BatchTotal)
		case "view":
			got = snap.View
		default:
			t.Fatalf("unknown assertion field %q", assertion.Field)
		}
		if got != assertion.Value {
			t.Errorf("after %q: %s = %q; want %q", step.Input, assertion.Field, got, assertion.Value)
		}
	}

	return m
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}

	return strings.Join(items, " ")
}

func loadFixtures(t *testing.T) []fixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.fix"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no fixtures found")
	}

	fixtures := make([]fixture, 0, len(paths))
	for _, path := range paths {
		f, err := parseFixture(path)
		if err != nil {
			t.Fatal(err)
		}
		fixtures = append(fixtures, f)
	}

	return fixtures
}

func TestFixtures(t *testing.T) {
	t.Parallel()
	for _, f := range loadFixtures(t) {
		t.Run(strings.TrimSuffix(filepath.Base(f.Path), ".fix"), func(t *testing.T) {
			t.Parallel()
			m := fixtureDataset(t, f.Given)
			for _, step := range f.Steps {
				m = runFixtureStep(t, m, step)
			}
		})
	}
}
