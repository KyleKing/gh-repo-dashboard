//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// prTabModel is a model sitting on the PRs tab with one row loaded, from a
// repo the fleet knows.
func prTabModel() Model {
	m := focusedModel(160, 40)
	m.viewMode = ViewModePRList
	m.summaries["/dev/alpha"] = models.RepoSummary{Path: "/dev/alpha", RemoteRepo: "acme/alpha"}
	m.prSearch = []models.PRInfo{
		{Number: 11, Title: "Bump the deps", State: "OPEN", HeadRef: "deps", Repo: "acme/alpha"},
	}

	return m
}

// The bar has to name the key that opens each tab, in the case it is typed.
func TestTabBarBracketsTheKeyAsTyped(t *testing.T) {
	t.Parallel()

	bar := plainText(prTabModel().renderTabBar(160))
	for _, want := range []string{"[R]epos", "[P]Rs"} {
		if !strings.Contains(bar, want) {
			t.Errorf("tab bar is missing %q: %q", want, bar)
		}
	}
}

// Opening the PRs tab from the repo list has no one repo to scope a search
// to, so it defaults to everywhere the search reaches rather than falling
// back to an arbitrary repo (which can fail outright if that repo has no
// remote gh can resolve).
func TestOpenTabDefaultsToFleetScopeFromTheRepoList(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 40)
	m.viewMode = ViewModeRepoList

	opened := mustModel(t, mustUpdate(t, &m, keyPress('P')))
	if !opened.prFleet {
		t.Error("opening the PRs tab from the repo list should default to fleet scope")
	}
}

// Opening the PRs tab from a repo already in focus defaults the search to
// that repo, since it is the one the tab was opened to ask about.
func TestOpenTabDefaultsToThisRepoScopeFromRepoDetail(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 40)

	opened := mustModel(t, mustUpdate(t, &m, keyPress('P')))
	if opened.prFleet {
		t.Error("opening the PRs tab from a focused repo should default to this-repo scope")
	}
}

func TestTabKeysCrossBetweenTheListAndThePRTab(t *testing.T) {
	t.Parallel()

	m := focusedModel(160, 40)
	m.viewMode = ViewModeRepoList

	opened := mustModel(t, mustUpdate(t, &m, keyPress('P')))
	if opened.viewMode != ViewModePRList {
		t.Fatalf("P from the list opened %v, want the PRs tab", opened.viewMode)
	}

	back := mustModel(t, mustUpdate(t, &opened, keyPress('R')))
	if back.viewMode != ViewModeRepoList {
		t.Errorf("R from the PRs tab opened %v, want the list", back.viewMode)
	}
}

// Each saved view is a separate question, so moving between them re-reads
// rather than filtering rows already on screen.
func TestCyclingAViewAndWideningTheScopeBothReRead(t *testing.T) {
	t.Parallel()

	m := prTabModel()

	cycledModel, _ := m.cyclePRView(1)
	cycled := mustModel(t, cycledModel)
	if cycled.prViewIndex != 1 || !cycled.prSearchLoading || len(cycled.prSearch) != 0 {
		t.Errorf("cycling left view=%d loading=%v rows=%d, want the next view being read",
			cycled.prViewIndex, cycled.prSearchLoading, len(cycled.prSearch))
	}

	widenedModel, _ := m.setPRScope(true)
	widened := mustModel(t, widenedModel)
	if !widened.prFleet || !widened.prSearchLoading {
		t.Errorf("widening the scope left fleet=%v loading=%v", widened.prFleet, widened.prSearchLoading)
	}
}

// A search that lands after the question changed would otherwise be shown
// under the wrong heading.
func TestASearchAnswerForAnotherViewIsDropped(t *testing.T) {
	t.Parallel()

	m := prTabModel()
	m.prSearchLoading = true
	m.prSearch = nil

	stale := mustModel(t, mustUpdate(t, &m, PRSearchLoadedMsg{
		Query: "is:open author:@me sort:updated-desc",
		PRs:   []models.PRInfo{{Number: 99}},
	}))
	if len(stale.prSearch) != 0 {
		t.Error("an answer to a view that is no longer showing must be dropped")
	}

	fresh := mustModel(t, mustUpdate(t, &m, PRSearchLoadedMsg{
		Query: m.currentPRView().Search,
		PRs:   []models.PRInfo{{Number: 99}},
	}))
	if len(fresh.prSearch) != 1 || fresh.prSearchLoading {
		t.Errorf("the current view's answer should land, got rows=%d loading=%v",
			len(fresh.prSearch), fresh.prSearchLoading)
	}
}

func TestCheckoutFromThePRTabResolvesTheRepoAndAsksFirst(t *testing.T) {
	t.Parallel()

	m := prTabModel()

	gatedModel, _ := m.startCheckoutSearchPR()
	gated := mustModel(t, gatedModel)
	if gated.viewMode != ViewModeConfirm {
		t.Fatalf("checkout should park behind a confirmation, got %v", gated.viewMode)
	}
	if !strings.Contains(gated.pendingAction.detail, "#11") {
		t.Errorf("the prompt should name the pull request, got %q", gated.pendingAction.detail)
	}

	// A pull request from a repository this scan never saw cannot be checked
	// out anywhere, and the refusal has to say which one.
	m.prSearch[0].Repo = "someone/else"
	_, cmd := m.startCheckoutSearchPR()
	if cmd == nil {
		t.Fatal("expected a status message refusing the checkout")
	}
	if status, ok := cmd().(StatusMsg); !ok || !strings.Contains(status.Message, "someone/else") {
		t.Errorf("refusal does not name the repository, got %v", cmd())
	}
}
