//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

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

// parseFixtureWhen builds the fixtureStep for a "when ..." line.
func parseFixtureWhen(path string, lineNo int, line string) (fixtureStep, error) {
	input := strings.TrimSpace(strings.TrimPrefix(line, "when "))
	step := fixtureStep{Input: input}

	switch {
	case strings.HasPrefix(input, ":"):
		step.IsCommand = true
	case strings.HasPrefix(input, "keys "):
		step.Keys = strings.Fields(strings.TrimPrefix(input, "keys "))
	default:
		return fixtureStep{}, fmt.Errorf("%s:%d: %w", path, lineNo+1, errFixtureBadWhen)
	}

	return step, nil
}

// applyFixtureThen appends the assertion from a "then field = value" line to
// the most recent step in f.
func applyFixtureThen(f *fixture, path string, lineNo int, line string) error {
	if len(f.Steps) == 0 {
		return fmt.Errorf("%s:%d: %w", path, lineNo+1, errFixtureThenBeforeWhen)
	}

	field, value, found := strings.Cut(strings.TrimPrefix(line, "then "), "=")
	if !found {
		return fmt.Errorf("%s:%d: %w", path, lineNo+1, errFixtureBadThen)
	}

	last := &f.Steps[len(f.Steps)-1]
	last.Assertions = append(last.Assertions, fixtureAssertion{
		Field: strings.TrimSpace(field),
		Value: strings.TrimSpace(value),
	})

	return nil
}

func parseFixtureLine(f *fixture, path string, lineNo int, line string) error {
	switch {
	case strings.HasPrefix(line, "doc:"):
		f.Doc = strings.TrimSpace(strings.TrimPrefix(line, "doc:"))
		return nil

	case strings.HasPrefix(line, "given "):
		f.Given = strings.TrimSpace(strings.TrimPrefix(line, "given "))
		return nil

	case strings.HasPrefix(line, "when "):
		step, err := parseFixtureWhen(path, lineNo, line)
		if err != nil {
			return err
		}
		f.Steps = append(f.Steps, step)

		return nil

	case strings.HasPrefix(line, "then "):
		return applyFixtureThen(f, path, lineNo, line)

	default:
		return fmt.Errorf("%s:%d: %w: %q", path, lineNo+1, errFixtureUnrecognized, line)
	}
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

		if err := parseFixtureLine(&f, path, lineNo, line); err != nil {
			return fixture{}, err
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
		"/repos/behind":   {Path: "/repos/behind", Branch: mainBranchName, Behind: 2},
		"/repos/clean":    {Path: "/repos/clean", Branch: mainBranchName},
		"/repos/dirty":    {Path: "/repos/dirty", Branch: mainBranchName, Unstaged: 2},
		"/repos/dirty-pr": {Path: "/repos/dirty-pr", Branch: "feat", Unstaged: 1, PRInfo: &models.PRInfo{Number: 7}},
	}
	m.updateFilteredPaths()

	return m
}

func fixtureKeyMsg(name string) tea.KeyPressMsg {
	switch name {
	case keyEnter:
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case keyEsc:
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
		r, _ := utf8.DecodeRuneInString(name)
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
		got, ok := fixtureAssertionValue(snap, assertion.Field)
		if !ok {
			t.Fatalf("unknown assertion field %q", assertion.Field)
		}
		if got != assertion.Value {
			t.Errorf("after %q: %s = %q; want %q", step.Input, assertion.Field, got, assertion.Value)
		}
	}

	return m
}

// fixtureAssertionValue reads the named field off a Model snapshot for fixture assertions.
func fixtureAssertionValue(snap Snapshot, field string) (string, bool) {
	switch field {
	case "cursor":
		return strconv.Itoa(snap.Cursor), true
	case "filtered":
		return joinOrNone(snap.Filtered), true
	case "input":
		return snap.CommandInput, true
	case "predicate":
		return snap.Predicate, true
	case "search":
		return snap.Search, true
	case "selected":
		return joinOrNone(snap.Selected), true
	case "status":
		return snap.StatusMessage, true
	case "task":
		return snap.BatchTask, true
	case "total":
		return strconv.Itoa(snap.BatchTotal), true
	case "view":
		return snap.View, true
	default:
		return "", false
	}
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
