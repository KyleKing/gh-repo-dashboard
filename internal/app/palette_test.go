//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

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
		"/dev/alpha": {Path: "/dev/alpha", Branch: mainBranchName, RemoteRepo: "acme/alpha"},
		"/dev/beta":  {Path: "/dev/beta", Branch: "fix/login", RemoteRepo: "acme/beta"},
	}
	m.repoPaths = []string{"/dev/alpha", "/dev/beta"}
	m.prMap = map[string]PRMapLoadedMsg{
		"/dev/alpha": {Path: "/dev/alpha", PRs: []models.PRInfo{
			{Number: 12, Title: "Composable extras", State: "OPEN"},
		}},
		"/dev/beta": {Path: "/dev/beta", PRs: []models.PRInfo{
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

	rendered := plainText(m.renderPalette())
	for _, want := range []string{"find", "2 matches", "Composable extras", "act on set"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("palette is missing %q:\n%s", want, rendered)
		}
	}
}
