//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

var errExplodedPreview = errors.New("gh exploded")

// prTabModel is a model sitting on the PRs tab with one row loaded, from a
// repo the fleet knows.
func prTabModel() Model {
	m := focusedModel(160, 40)
	m.viewMode = ViewModePRList
	m.summaries["/dev/alpha"] = models.RepoSummary{Path: "/dev/alpha", RemoteRepo: "acme/alpha"}
	m.prSearch = []models.PRInfo{
		{
			Number: 11, Title: "Bump the deps", State: "OPEN", HeadRef: "deps",
			Repo: "acme/alpha", URL: "https://github.com/acme/alpha/pull/11",
		},
	}

	return m
}

// TestPRFilterNarrowsVisiblePRsAndCursor exercises the local ":filter" on the
// PRs tab: it should narrow what's rendered without touching repo state, and
// the cursor/Enter must resolve against the filtered list, not the raw fetch.
func TestPRFilterNarrowsVisiblePRsAndCursor(t *testing.T) {
	t.Parallel()
	m := prTabModel()
	m.prSearch = []models.PRInfo{
		{Number: 11, Title: "Ready one", State: "OPEN", Repo: "acme/alpha"},
		{Number: 12, Title: "Draft one", State: "OPEN", Repo: "acme/alpha", IsDraft: true},
	}

	m2, _ := m.ExecuteCommand("filter draft")
	if m2.prPredicateText != "draft" {
		t.Fatalf("expected PR predicate recorded, got %q", m2.prPredicateText)
	}

	visible := m2.visiblePRs()
	if len(visible) != 1 || visible[0].Number != 12 {
		t.Fatalf("expected only the draft PR visible, got %v", visible)
	}

	pr, ok := m2.selectedSearchPR()
	if !ok || pr.Number != 12 {
		t.Fatalf("expected the cursor to resolve to the draft PR, got %+v ok=%v", pr, ok)
	}

	m3, _ := m2.ExecuteCommand("filter")
	if m3.prPredicateText != "" || m3.prPredicate != nil {
		t.Errorf("expected bare :filter to clear the PR predicate, got %q", m3.prPredicateText)
	}
	if len(m3.visiblePRs()) != 2 {
		t.Errorf("expected both PRs visible again, got %v", m3.visiblePRs())
	}
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

// TestPRQueryOverrideSurvivesRefetchButNotViewSwitch guards the staleness
// check: an overridden query's result must not be dropped as stale, but the
// override itself must not silently carry over onto a different view.
func TestPRQueryOverrideSurvivesRefetchButNotViewSwitch(t *testing.T) {
	t.Parallel()

	m := prTabModel()
	overridden, _ := m.ExecuteCommand("pr-query is:open label:bug")
	m2 := mustModel(t, overridden)
	if m2.prQueryOverride != "is:open label:bug" {
		t.Fatalf("expected override recorded, got %q", m2.prQueryOverride)
	}
	if !m2.prSearchLoading {
		t.Fatal("expected :pr-query to trigger a refetch")
	}

	landed := mustModel(t, mustUpdate(t, &m2, PRSearchLoadedMsg{
		Query: "is:open label:bug",
		PRs:   []models.PRInfo{{Number: 42}},
	}))
	if len(landed.prSearch) != 1 || landed.prSearchLoading {
		t.Errorf("overridden query's result should land, got rows=%d loading=%v",
			len(landed.prSearch), landed.prSearchLoading)
	}

	cycled := mustModel(t, mustUpdate(t, &landed, keyPress(']')))
	if cycled.prQueryOverride != "" {
		t.Errorf("expected the override to clear on view switch, got %q", cycled.prQueryOverride)
	}
}

// A landed search's row count is already in the heading badge
// (renderPRListHeading); a status message duplicating it never gets cleared,
// so it would sit permanently over the footer.
func TestPRSearchLandingSetsNoStatusMessage(t *testing.T) {
	t.Parallel()

	m := prTabModel()
	m.prSearchLoading = true
	m.prSearch = nil

	landed := mustModel(t, mustUpdate(t, &m, PRSearchLoadedMsg{
		Query: m.currentPRView().Search,
		PRs:   []models.PRInfo{{Number: 99}},
	}))
	if landed.statusMessage != "" {
		t.Errorf("a landed search should not set a status message, got %q", landed.statusMessage)
	}
}

// TestReviewerBadgeFlagsAnUnassignedOpenPR confirms the row-level signal:
// an open, non-draft pull request with nobody requested to review it is
// flagged, a request narrows it to a count, and neither shows once the pull
// request is no longer open for review.
func TestReviewerBadgeFlagsAnUnassignedOpenPR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pr   models.PRInfo
		want string
	}{
		{"needs a reviewer", models.PRInfo{State: "OPEN"}, "needs reviewer"},
		{"one requested", models.PRInfo{State: "OPEN", Reviewers: []string{"erin"}}, "1 reviewer"},
		{"two requested", models.PRInfo{State: "OPEN", Reviewers: []string{"erin", "dave"}}, "2 reviewers"},
		{"draft carries neither", models.PRInfo{State: "OPEN", IsDraft: true}, ""},
		{"merged carries neither", models.PRInfo{State: "MERGED"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := reviewerBadge(&tt.pr); !strings.Contains(got, tt.want) || (tt.want == "" && got != "") {
				t.Errorf("reviewerBadge(%+v) = %q, want to contain %q", tt.pr, got, tt.want)
			}
		})
	}
}

// TestPRPreviewLoadsAndRendersTheCursorRow exercises the "v" inline preview:
// opening it requests the cursor row's own detail, and once that detail
// lands it renders the row's reviewers rather than a loading placeholder.
func TestPRPreviewLoadsAndRendersTheCursorRow(t *testing.T) {
	t.Parallel()

	m := prTabModel()

	opened, cmd := m.togglePRPreview()
	o := mustModel(t, opened)
	if !o.prPreviewOpen {
		t.Fatal("expected the preview region to open")
	}
	if cmd == nil {
		t.Fatal("expected opening the preview to request the cursor row's detail")
	}

	key := "https://github.com/acme/alpha/pull/11"
	if !o.prPreviewRequested[key] {
		t.Fatalf("expected the row's fetch to be marked requested, got %+v", o.prPreviewRequested)
	}

	loading := plainText(o.renderPRList())
	if !strings.Contains(loading, readingLabel) {
		t.Errorf("expected the preview to show its loading placeholder, got:\n%s", loading)
	}

	landed := mustModel(t, mustUpdate(t, &o, PRPreviewLoadedMsg{
		Key:     key,
		Preview: models.PRPreview{Reviewers: []string{"erin"}},
	}))

	rendered := plainText(landed.renderPRList())
	if !strings.Contains(rendered, "erin") {
		t.Errorf("expected the loaded reviewer to render, got:\n%s", rendered)
	}
}

// TestPRPreviewFailureIsSaidOnceAndRetried covers the read that never lands:
// the region has to say so rather than sit on its loading placeholder, and
// the row must be requestable again rather than marked in-flight forever.
func TestPRPreviewFailureIsSaidOnceAndRetried(t *testing.T) {
	t.Parallel()

	opened, _ := prTabModel().togglePRPreview()
	o := mustModel(t, opened)

	key := "https://github.com/acme/alpha/pull/11"
	failed := mustModel(t, mustUpdate(t, &o, PRPreviewLoadedMsg{Key: key, Error: errExplodedPreview}))

	if failed.prPreviewRequested[key] {
		t.Error("a failed read should clear its in-flight mark so the row can be retried")
	}

	rendered := plainText(failed.renderPRList())
	if !strings.Contains(rendered, "gh exploded") {
		t.Errorf("expected the failure to render, got:\n%s", rendered)
	}
	if strings.Contains(rendered, readingLabel) {
		t.Errorf("a failed read must not keep showing the loading placeholder, got:\n%s", rendered)
	}
}

// TestPRPreviewDebouncesAMovingCursor covers holding j: only the row the
// cursor settles on may be read, since every intermediate row used to spawn
// its own gh invocation for the settled row to queue behind.
func TestPRPreviewDebouncesAMovingCursor(t *testing.T) {
	t.Parallel()

	m := prTabModel()
	m.prSearch = append(m.prSearch, models.PRInfo{
		Number: 12, State: "OPEN", Repo: "acme/alpha", URL: "https://github.com/acme/alpha/pull/12",
	})
	m.prPreviewOpen = true

	moved, cmd := m.schedulePRPreview()
	if cmd == nil {
		t.Fatal("a cursor move with the preview open should schedule a read")
	}

	stale := mustModel(t, mustUpdate(t, &moved, PRPreviewTickMsg{Seq: moved.prPreviewSeq - 1}))
	if len(stale.prPreviewRequested) != 0 {
		t.Errorf("a tick from an earlier cursor position must not read, got %+v", stale.prPreviewRequested)
	}

	settled := mustModel(t, mustUpdate(t, &moved, PRPreviewTickMsg{Seq: moved.prPreviewSeq}))
	if !settled.prPreviewRequested["https://github.com/acme/alpha/pull/11"] {
		t.Errorf("the settled row should be read, got %+v", settled.prPreviewRequested)
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

// TestPRsTabOpensTheCursorRowInTheBrowser covers "o" reaching the row under
// the cursor without the "!" leader, which is what the tab's own footer
// promises. A row with no URL is the observable half: the refusal proves the
// key routed to the open action rather than falling through unhandled.
func TestPRsTabOpensTheCursorRowInTheBrowser(t *testing.T) {
	t.Parallel()

	m := prTabModel()
	m.prSearch[0].URL = ""

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	if cmd == nil {
		t.Fatal("expected o to act on the cursor row")
	}

	status, ok := cmd().(StatusMsg)
	if !ok || !strings.Contains(status.Message, "No pull request URL") {
		t.Errorf("expected the missing-URL refusal, got %#v", cmd())
	}

	if mustModel(t, updated).viewMode != ViewModePRList {
		t.Error("o must not leave the PRs tab")
	}
}
