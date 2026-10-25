//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestParseFindQueryGrammar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw    string
		kind   findKind
		text   string
		number int
		fleet  bool
	}{
		{raw: "", kind: findAny},
		{raw: "escape user", kind: findAny, text: "escape user"},
		{raw: "#12", kind: findPR, text: "12", number: 12},
		{raw: "b fix", kind: findBranch, text: "fix"},
		{raw: "s wip", kind: findStash, text: "wip"},
		{raw: "n todo", kind: findNote, text: "todo"},
		{raw: "r dash", kind: findRepo, text: "dash"},
		{raw: "* r dash", kind: findRepo, text: "dash", fleet: true},
		{raw: "x ray", kind: findAny, text: "x ray"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			q := parseFindQuery(tt.raw)
			if q.kind != tt.kind || q.text != tt.text || q.number != tt.number || q.fleet != tt.fleet {
				t.Errorf("parseFindQuery(%q) = %+v, want kind %v text %q number %d fleet %v",
					tt.raw, q, tt.kind, tt.text, tt.number, tt.fleet)
			}
		})
	}
}

// paletteFleet returns a two-repo fleet with a PR map loaded, which is the
// cache-resident state a fleet-wide find reads.
func paletteFleet() Model {
	m := New([]string{"/dev"}, 1)
	m.width, m.height = 160, 40
	m.loading = false
	m.summaries = map[string]models.RepoSummary{
		"/dev/alpha": {
			RepoSummary: vcs.RepoSummary{Path: "/dev/alpha", Branch: mainBranchName, RemoteRepo: "acme/alpha"},
		},
		"/dev/beta": {RepoSummary: vcs.RepoSummary{Path: "/dev/beta", Branch: "fix/login", RemoteRepo: "acme/beta"}},
	}
	m.repoPaths = []string{"/dev/alpha", "/dev/beta"}
	m.prMap = map[string]PRMapLoadedMsg{
		"/dev/alpha": {Path: "/dev/alpha", PRs: []forge.PullRequest{
			{Number: 12, Title: "Composable extras", State: "OPEN"},
		}},
		"/dev/beta": {Path: "/dev/beta", PRs: []forge.PullRequest{
			{Number: 12, Title: "Bump mise-action", State: "OPEN"},
		}},
	}
	m.updateFilteredPaths()

	return m
}

func TestFindAnswersWhichReposHaveAPRNumber(t *testing.T) {
	t.Parallel()

	m := paletteFleet()
	opened, _ := m.openPalette()
	m = mustModel(t, opened)
	m.paletteInput.SetValue("#12")

	results := m.findResults()
	if len(results) != 2 {
		t.Fatalf("#12 found %d results, want both repos: %+v", len(results), results)
	}
	for _, r := range results {
		if r.kind != findPR || r.number != 12 {
			t.Errorf("result %+v is not PR 12", r)
		}
	}

	if repos := resultRepos(results); len(repos) != 2 {
		t.Errorf("result repos = %v, want one per repo holding the PR", repos)
	}
}

func TestFindPrefixNarrowsToOneType(t *testing.T) {
	t.Parallel()

	m := paletteFleet()
	opened, _ := m.openPalette()
	m = mustModel(t, opened)

	m.paletteInput.SetValue("r alpha")
	for _, r := range m.findResults() {
		if r.kind != findRepo {
			t.Errorf("a repo query returned %v", r.kindName())
		}
	}

	m.paletteInput.SetValue("b fix")
	branches := m.findResults()
	if len(branches) != 1 || branches[0].branch != "fix/login" {
		t.Errorf("branch query returned %+v, want just fix/login", branches)
	}
}

func TestActingOnARepoSetCommitsToTheTextObject(t *testing.T) {
	t.Parallel()

	m := paletteFleet()
	opened, _ := m.openPalette()
	m = mustModel(t, opened)
	m.paletteInput.SetValue("r a")

	results := m.findResults()
	acted, _ := selectResultRepos(m, results)

	if len(acted.selectedPaths) != 2 {
		t.Fatalf("selected %v, want both repos committed to the selected-repos object", acted.selectedPaths)
	}

	obj, _ := lookupTextObject("sr")
	if paths := acted.resolveTextObject(obj); len(paths) != 2 {
		t.Errorf("the selected-repos text object resolves to %v, want the find's set", paths)
	}
}

func TestMarkingLimitsTheActionSet(t *testing.T) {
	t.Parallel()

	m := paletteFleet()
	opened, _ := m.openPalette()
	m = mustModel(t, opened)
	m.paletteInput.SetValue("#12")

	marked, _ := m.togglePaletteMark(m.findResults())
	m = mustModel(t, marked)

	targets := m.paletteTargets(m.findResults())
	if len(targets) != 1 {
		t.Errorf("marked set = %+v, want only the marked row", targets)
	}
}

func TestFindOpensFromBothViews(t *testing.T) {
	t.Parallel()

	list := paletteFleet()
	opened, _ := list.Update(keyPress(';'))
	if mustModel(t, opened).viewMode != ViewModePalette {
		t.Error("; should open the palette from the fleet list, where space already selects")
	}

	stillSelects, _ := list.Update(keyPress(' '))
	if mustModel(t, stillSelects).viewMode == ViewModePalette {
		t.Error("space must keep selecting in the fleet list")
	}

	detail := focusedModel(160, 40)
	fromDetail, _ := detail.Update(keyPress(' '))
	if mustModel(t, fromDetail).viewMode != ViewModePalette {
		t.Error("space should open the palette in the focused view")
	}
}

func TestEnterOpensTheRepoOnTheObjectsPanel(t *testing.T) {
	t.Parallel()

	m := paletteFleet()
	opened, _ := m.openPalette()
	m = mustModel(t, opened)
	m.paletteInput.SetValue("#12")

	next, _ := m.runPaletteDefault(m.findResults())
	landed := mustModel(t, next)

	if landed.viewMode != ViewModeRepoDetail || landed.focusedPanel != panelBranches {
		t.Errorf("enter landed in %v on panel %v, want the repo's PRs panel",
			landed.viewMode, landed.focusedPanel)
	}
}

func TestPaletteRendersItsMatchesAndFooter(t *testing.T) {
	t.Parallel()

	m := paletteFleet()
	opened, _ := m.openPalette()
	m = mustModel(t, opened)
	m.paletteInput.SetValue("#12")

	rendered := plainText(m.renderScreen())
	for _, want := range []string{"find", "2 matches", "Composable extras", "[tab/↓] results"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("palette is missing %q:\n%s", want, rendered)
		}
	}

	rendered = plainText(m.focusPaletteResults().renderScreen())
	for _, want := range []string{"[a]ct on set", "[tab] mark", "[/] edit query"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the results footer is missing %q:\n%s", want, rendered)
		}
	}
}

func typeKeys(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		next, _ := m.Update(keyPress(r))
		m = mustModel(t, next)
	}

	return m
}

func TestPalette_TypingCalcYieldsQueryCalc(t *testing.T) {
	t.Parallel()

	opened, _ := paletteFleet().openPalette()
	m := typeKeys(t, mustModel(t, opened), "calc")

	if got := m.paletteInput.Value(); got != "calc" {
		t.Errorf("typing calc gave query %q; the a must reach the query, not open the verbs", got)
	}
	if m.paletteActions || m.paletteResults {
		t.Errorf("typing must leave focus on the query line: actions %v results %v", m.paletteActions, m.paletteResults)
	}
}

func TestPaletteFocusMovesBetweenTheQueryAndTheResults(t *testing.T) {
	t.Parallel()

	tab, down := tea.KeyPressMsg{Code: tea.KeyTab}, tea.KeyPressMsg{Code: tea.KeyDown}
	ctrlN := tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}

	tests := []struct {
		name        string
		keys        []tea.KeyPressMsg
		wantQuery   string
		wantResults bool
		wantActions bool
		wantMarks   int
	}{
		{
			"g, :, and a are query text",
			[]tea.KeyPressMsg{keyPress('g'), keyPress(':'), keyPress('a')},
			"g:a", false, false, 0,
		},
		{"tab moves into the results", []tea.KeyPressMsg{keyPress('#'), keyPress('1'), tab}, "#1", true, false, 0},
		{"down moves in", []tea.KeyPressMsg{down}, "", true, false, 0},
		{"ctrl-n moves in", []tea.KeyPressMsg{ctrlN}, "", true, false, 0},
		{"a opens the verbs from the results", []tea.KeyPressMsg{down, keyPress('a')}, "", true, true, 0},
		{"tab marks from the results", []tea.KeyPressMsg{down, tab, tab}, "", true, false, 2},
		{"slash returns to the query", []tea.KeyPressMsg{down, keyPress('/'), keyPress('a')}, "a", false, false, 0},
		{"i returns to the query", []tea.KeyPressMsg{down, keyPress('i'), keyPress('b')}, "b", false, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := paletteFleet()
			opened, _ := m.openPalette()
			m = mustModel(t, opened)
			m.paletteInput.SetValue("")
			for _, k := range tt.keys {
				next, _ := m.Update(k)
				m = mustModel(t, next)
			}

			if m.paletteInput.Value() != tt.wantQuery || m.paletteResults != tt.wantResults ||
				m.paletteActions != tt.wantActions || len(m.paletteMarks) != tt.wantMarks {
				t.Errorf("query %q results %v actions %v marks %d; want %q %v %v %d",
					m.paletteInput.Value(), m.paletteResults, m.paletteActions, len(m.paletteMarks),
					tt.wantQuery, tt.wantResults, tt.wantActions, tt.wantMarks)
			}
			if m.viewMode != ViewModePalette {
				t.Errorf("view = %v; want the palette still open", m.viewMode)
			}
		})
	}
}

func TestPaletteQuitPathsEndTheProgramFromEitherFocus(t *testing.T) {
	t.Parallel()

	opened, _ := paletteFleet().openPalette()
	query := mustModel(t, opened)
	results := query.focusPaletteResults()

	for name, tt := range map[string]struct {
		m   Model
		key tea.KeyPressMsg
	}{
		"ctrl-c in the query":   {query, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
		"ctrl-c in the results": {results, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
		"q in the results":      {results, keyPress('q')},
	} {
		if _, cmd := tt.m.Update(tt.key); cmd == nil {
			t.Errorf("%s did not quit", name)
		}
	}

	next, _ := query.Update(keyPress('q'))
	if got := mustModel(t, next).paletteInput.Value(); got != "q" {
		t.Errorf("q in the query gave %q; want it typed", got)
	}
}
