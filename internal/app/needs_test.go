//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

const viewerLogin = "kyle"

func repoWith(mutate func(*models.RepoSummary)) models.RepoSummary {
	s := models.RepoSummary{RepoSummary: vcs.RepoSummary{Path: "/dev/app", Branch: mainBranchName}}
	mutate(&s)

	return s
}

func failingCI() *forge.DefaultBranchCI { return workflows(passingRun, failingRun) }

func TestNeedsPromotionAndSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*models.RepoSummary)
		shared   bool
		due      []string
		promotes bool
		rank     int
	}{
		{"quiet", func(*models.RepoSummary) {}, false, nil, false, rankQuiet},
		{"failing CI", func(s *models.RepoSummary) { s.CI = failingCI() }, false, nil, true, rankFailingCI},
		{"unpushed", func(s *models.RepoSummary) { s.Ahead = 1 }, false, nil, true, rankUnpushed},
		{"dirty", func(s *models.RepoSummary) { s.Unstaged = 1 }, false, nil, true, rankDirty},
		{"untracked only", func(s *models.RepoSummary) { s.Untracked = 2 }, false, nil, true, rankDirty},
		{"shared branch", func(*models.RepoSummary) {}, true, nil, true, rankOther},
		{
			"your PR has changes requested",
			func(s *models.RepoSummary) { s.OpenPRs = []forge.PullRequest{myPR("CHANGES_REQUESTED")} },
			false, nil, true, rankOther,
		},
		{
			"failing CI outranks unpushed",
			func(s *models.RepoSummary) { s.CI, s.Ahead = failingCI(), 3 },
			false, nil, true, rankFailingCI,
		},
		{"behind", func(s *models.RepoSummary) { s.Behind = 4 }, false, nil, false, rankQuiet},
		{
			"notes",
			func(s *models.RepoSummary) { s.NotesFiles = []models.NoteFile{{Name: ".doing"}} },
			false, nil, false, rankQuiet,
		},
		{"stashes", func(s *models.RepoSummary) { s.StashCount = 2 }, false, nil, false, rankQuiet},
		{
			"template drift",
			func(s *models.RepoSummary) {
				s.TemplateInfo = &models.CopierTemplateInfo{Commit: "v1", IsTag: true, Behind: true, LatestTag: "v2"}
			},
			false, nil, false, rankQuiet,
		},
		{"due task", func(*models.RepoSummary) {}, false, []string{"gc"}, false, rankQuiet},
		{
			"CI still running",
			func(s *models.RepoSummary) { s.CI = workflows(runningRun) },
			false, nil, false, rankQuiet,
		},
		{
			"someone else's PR is not your turn",
			func(s *models.RepoSummary) { s.OpenPRs = []forge.PullRequest{{Number: 1, Author: "bot"}} },
			false, nil, false, rankQuiet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := New(nil, 1)
			m.viewer = viewerLogin

			summary := repoWith(tt.mutate)
			m.shared[summary.Path] = tt.shared
			m.due[summary.Path] = tt.due

			n := m.needsOf(summary)
			if n.promotes() != tt.promotes || n.rank() != tt.rank {
				t.Errorf("promotes=%v rank=%d, want promotes=%v rank=%d", n.promotes(), n.rank(), tt.promotes, tt.rank)
			}
		})
	}
}

func myPR(decision string) forge.PullRequest {
	return forge.PullRequest{Number: 31, Author: viewerLogin, ReviewDecision: decision}
}

func TestPRTurn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prs    []forge.PullRequest
		viewer string
		want   int
	}{
		{"changes requested on yours", []forge.PullRequest{myPR("CHANGES_REQUESTED")}, viewerLogin, 31},
		{"approved and ready", []forge.PullRequest{myPR("APPROVED")}, viewerLogin, 31},
		{
			"approved draft waits on nobody",
			[]forge.PullRequest{{Number: 31, Author: viewerLogin, ReviewDecision: "APPROVED", IsDraft: true}},
			viewerLogin, 0,
		},
		{
			"approved with failing checks",
			[]forge.PullRequest{{
				Number: 31, Author: viewerLogin, ReviewDecision: "APPROVED",
				Checks: forge.ChecksStatus{Total: 2, Failing: 1},
			}},
			viewerLogin, 0,
		},
		{
			"review requested of you",
			[]forge.PullRequest{{Number: 9, Author: "bot", Reviewers: []string{viewerLogin}}},
			viewerLogin, 9,
		},
		{"awaiting review is theirs", []forge.PullRequest{myPR("REVIEW_REQUIRED")}, viewerLogin, 0},
		{"unknown viewer", []forge.PullRequest{myPR("CHANGES_REQUESTED")}, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := prTurn(tt.prs, tt.viewer); got != tt.want {
				t.Errorf("prTurn = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNeedsTokensRenderInFixedOrder(t *testing.T) {
	t.Parallel()

	everything := needs{
		changed: 3, untracked: 1, ahead: 2, behind: 1,
		ciFailing: 7, ciTotal: 9, shared: true, note: true, stashes: 1,
		drift: "v0.15.2", due: []string{"gc"},
	}

	tests := []struct {
		name  string
		n     needs
		text  string
		tones []tone
	}{
		{"quiet is blank", needs{}, "", nil},
		{
			"every token", everything,
			"~3 ?1  ↑2  ↓1  ✗ ci 7/9  ⚠ shared  ✎ note  ≡1  → v0.15.2  ⟳ gc",
			[]tone{
				toneWarning, toneWarning, toneMuted, toneError, toneWarning,
				toneAccent, toneMuted, toneMuted, toneMuted,
			},
		},
		{"running CI is muted", needs{ciRunning: true}, "… ci", []tone{toneMuted}},
		{"stale failing CI is muted", needs{ciFailing: 1, ciTotal: 2, ciStale: true}, "✗ ci 1/2", []tone{toneMuted}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tokens := tt.n.tokens()
			texts := make([]string, len(tokens))
			tones := make([]tone, len(tokens))

			for i, token := range tokens {
				texts[i], tones[i] = token.text, token.tone
			}

			if got := strings.Join(texts, needsGap); got != tt.text {
				t.Errorf("tokens = %q, want %q", got, tt.text)
			}

			for i := range tt.tones {
				if tones[i] != tt.tones[i] {
					t.Errorf("token %q has tone %d, want %d", texts[i], tones[i], tt.tones[i])
				}
			}
		})
	}
}

func TestRenderNeedsDropsTrailingTokensThatDoNotFit(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	n := needs{ahead: 2, ciFailing: 1, ciTotal: 2, note: true}

	got := strings.TrimRight(plainText(m.renderNeeds(n, 16, plainStyle, false)), " ")
	if want := "↑2  ✗ ci 1/2…"; got != want {
		t.Errorf("needs cell = %q, want %q; the urgent tokens must survive and the cut must be marked", got, want)
	}
}

func TestStaleValuesDrawBeforeLiveOnesAndNeverReplaceThem(t *testing.T) {
	t.Parallel()

	const path = "/dev/app"

	m := New(nil, 1)
	m.viewer = viewerLogin
	m.summaries[path] = repoWith(func(*models.RepoSummary) {})
	m.startFetch(path, fetchOpenPRs)

	stale := []forge.PullRequest{myPR("CHANGES_REQUESTED")}
	live := []forge.PullRequest{{Number: 5, Author: "bot"}}

	m = afterUpdate(t, m, OpenPRsLoadedMsg{Path: path, PRs: stale, Stale: true})

	n := m.needsOf(m.summaries[path])
	if n.turn != 31 || !n.prStale || n.prPending {
		t.Fatalf("stale list: turn=%d stale=%v pending=%v, want the cached turn drawn quietly with no spinner",
			n.turn, n.prStale, n.prPending)
	}

	cell := m.prsCell(n, 14, plainStyle, false)
	if !strings.Contains(plainText(cell), "#31 ←you") || strings.Contains(cell, m.spinner.View()) {
		t.Errorf("stale cell = %q", plainText(cell))
	}

	m = afterUpdate(t, m, OpenPRsLoadedMsg{Path: path, PRs: live})

	if n := m.needsOf(m.summaries[path]); n.turn != 0 || n.prStale || n.open != 1 {
		t.Errorf("live list did not replace the stale one: turn=%d stale=%v open=%d", n.turn, n.prStale, n.open)
	}

	m = afterUpdate(t, m, OpenPRsLoadedMsg{Path: path, PRs: stale, Stale: true})

	if got := m.summaries[path].OpenPRs; len(got) != 1 || got[0].Number != 5 {
		t.Errorf("a late stale read overwrote the live list: %+v", got)
	}
}

func TestSpinnerShowsOnlyOnceTheCacheCameUpEmpty(t *testing.T) {
	t.Parallel()

	const path = "/dev/app"

	m := New(nil, 1)
	m.summaries[path] = repoWith(func(*models.RepoSummary) {})
	m.startFetch(path, fetchOpenPRs)
	m.startFetch(path, fetchCI)

	if n := m.needsOf(m.summaries[path]); n.prPending || n.ciPending {
		t.Fatalf("prPending=%v ciPending=%v before the cache answered, want a blank cell, not a spinner",
			n.prPending, n.ciPending)
	}

	m = afterUpdate(t, m, OpenPRsLoadedMsg{Path: path, Stale: true, Miss: true})
	m = afterUpdate(t, m, WorkflowLoadedMsg{Path: path, Stale: true, Miss: true})

	if n := m.needsOf(m.summaries[path]); !n.prPending || !n.ciPending {
		t.Errorf("prPending=%v ciPending=%v with nothing cached, want both", n.prPending, n.ciPending)
	}

	m = afterUpdate(t, m, OpenPRsLoadedMsg{Path: path})
	if n := m.needsOf(m.summaries[path]); n.prPending {
		t.Error("the live read landed but the PRs cell still spins")
	}
}

func TestAttentionSortGroupsAndOrdersTheList(t *testing.T) {
	t.Parallel()

	now := time.Now()
	m := New([]string{"/dev"}, 1)
	m.width, m.height = 120, 30
	m.loading = false
	m.viewer = viewerLogin

	add := func(name string, age time.Duration, mutate func(*models.RepoSummary)) {
		path := "/dev/" + name
		s := models.RepoSummary{RepoSummary: vcs.RepoSummary{
			Path: path, Branch: mainBranchName, LastModified: now.Add(-age),
		}}
		mutate(&s)
		m.summaries[path] = s
		m.repoPaths = append(m.repoPaths, path)
	}

	add("quiet-new", time.Minute, func(*models.RepoSummary) {})
	add("quiet-old", time.Hour, func(*models.RepoSummary) {})
	add("dirty", time.Minute, func(s *models.RepoSummary) { s.Unstaged = 1 })
	add("unpushed", time.Hour, func(s *models.RepoSummary) { s.Ahead = 1 })
	add("failing", 2*time.Hour, func(s *models.RepoSummary) { s.CI = failingCI() })
	add("review", time.Hour, func(s *models.RepoSummary) {
		s.OpenPRs = []forge.PullRequest{{Number: 4, Author: "bot", Reviewers: []string{viewerLogin}}}
	})
	m.updateFilteredPaths()

	order := make([]string, 0, len(m.filteredPaths))
	for _, path := range m.filteredPaths {
		order = append(order, m.summaries[path].Name())
	}

	want := "failing unpushed dirty review quiet-new quiet-old"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("order = %q, want %q", got, want)
	}

	body := plainText(m.renderTable())
	for _, rule := range []string{"NEEDS YOU · 4", "QUIET · 2"} {
		if !strings.Contains(body, rule) {
			t.Errorf("list is missing the %q rule:\n%s", rule, body)
		}
	}

	sortByName(&m)
	m.updateFilteredPaths()

	if body := plainText(m.renderTable()); strings.Contains(body, "NEEDS YOU") {
		t.Errorf("a name sort still draws attention groups:\n%s", body)
	}
}

func TestParseChangedPaths(t *testing.T) {
	t.Parallel()

	got := parseChangedPaths(" M internal/a.go\n?? notes.md\nA docs/new file.md\n\n")
	want := []string{"internal/a.go", "notes.md", "docs/new file.md"}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("parseChangedPaths = %q, want %q", got, want)
	}
}
