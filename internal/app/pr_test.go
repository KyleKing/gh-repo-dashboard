//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func prDetailModel(detail *forge.PRDetail) Model {
	m := New(nil, 1)
	m.width = 120
	m.height = 40
	m.viewMode = ViewModePRDetail
	m.selectedRepo = testRepoPath
	m.summaries[testRepoPath] = models.RepoSummary{Path: testRepoPath}
	m.prDetail = *detail

	return m
}

func TestDetailListLen(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.selectedRepo = testRepo1Path
	m.branches = make([]models.BranchInfo, 5)
	m.stashes = make([]models.StashDetail, 3)
	m.worktrees = []models.WorktreeInfo{
		{Path: testRepo1Path, Branch: mainBranchName},
		{Path: "/repos/app-wt-a", Branch: "feature/a"},
		{Path: "/repos/app-wt-b", Branch: "feature/b"},
	}
	m.prs = make([]forge.PullRequest, 3)

	tests := []struct {
		panel panelID
		want  int
	}{
		{panelBranches, 5},
		{panelStashes, 3},
		// The peers panel lists parallel checkouts, so the repo's own working
		// directory is not one of them.
		{panelPeers, 2},
	}

	for _, tt := range tests {
		m.focusedPanel = tt.panel
		if got := m.detailListLen(); got != tt.want {
			t.Errorf("panel %v: expected %d rows, got %d", tt.panel, tt.want, got)
		}
	}
}

func TestPRInfoStatusDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pr   forge.PullRequest
		want string
	}{
		{"draft outranks open", forge.PullRequest{IsDraft: true, State: "OPEN"}, "DRAFT"},
		{"open", forge.PullRequest{State: "OPEN"}, "OPEN"},
		{"merged", forge.PullRequest{State: "MERGED"}, "MERGED"},
		{"closed", forge.PullRequest{State: "CLOSED"}, "CLOSED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pr.StatusDisplay(); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestPRInfoReviewStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		pr   forge.PullRequest
		want string
	}{
		{"approved", forge.PullRequest{ReviewDecision: "APPROVED"}, "approved"},
		{"changes requested", forge.PullRequest{ReviewDecision: "CHANGES_REQUESTED"}, "changes requested"},
		{"review required", forge.PullRequest{ReviewDecision: "REVIEW_REQUIRED"}, "review required"},
		{"an approval with no decision still reads as approved", forge.PullRequest{ApprovedBy: []string{"u1"}}, "approved"},
		{"no review", forge.PullRequest{}, emDash},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pr.ReviewStatus(); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

// TestPRDetailLoadedMsg covers what Update does with a detail fetch's outcome,
// including the error case, where the basic info taken from the list must
// survive so the view does not blank out on a failed fetch.
func TestPRDetailLoadedMsg(t *testing.T) {
	t.Parallel()
	basic := forge.PullRequest{
		Number: 456, Title: "Feature PR", State: "OPEN",
		HeadRef: featureBranchName, BaseRef: mainBranchName,
	}
	loaded := forge.PRDetail{
		PullRequest: basic,
		Author:      "alice",
		Assignees:   []string{"bob"},
		Reviewers:   []string{"charlie"},
		Additions:   100,
		Deletions:   50,
	}

	tests := []struct {
		name       string
		msg        PRDetailLoadedMsg
		wantNumber int
		wantAuthor string
	}{
		{
			name:       "a loaded detail replaces the basic info",
			msg:        PRDetailLoadedMsg{Path: testRepoPath, PRNumber: 456, Detail: loaded},
			wantNumber: 456,
			wantAuthor: "alice",
		},
		{
			name:       "an error preserves the basic info already shown",
			msg:        PRDetailLoadedMsg{Path: testRepoPath, PRNumber: 456, Error: errPRDetailLoad},
			wantNumber: 456,
			wantAuthor: "",
		},
		{
			name:       "a detail for another repo is ignored",
			msg:        PRDetailLoadedMsg{Path: "/elsewhere", PRNumber: 456, Detail: loaded},
			wantNumber: 456,
			wantAuthor: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := New(nil, 1)
			m.selectedRepo = testRepoPath
			m.selectedPR = basic
			m.prDetail = forge.PRDetail{PullRequest: basic}

			m = afterUpdate(t, m, tt.msg)

			if m.prDetail.Number != tt.wantNumber {
				t.Errorf("expected PR #%d, got #%d", tt.wantNumber, m.prDetail.Number)
			}
			if m.prDetail.Author != tt.wantAuthor {
				t.Errorf("expected author %q, got %q", tt.wantAuthor, m.prDetail.Author)
			}
			if m.prDetail.Title != basic.Title {
				t.Errorf("title should survive every outcome, got %q", m.prDetail.Title)
			}
		})
	}
}

func TestPRDetailViewRender(t *testing.T) {
	t.Parallel()
	full := forge.PRDetail{
		PullRequest: forge.PullRequest{
			Number: 456, Title: "Add amazing feature", HeadRef: "feature/amazing",
			BaseRef: mainBranchName, State: "OPEN", ReviewDecision: "APPROVED",
		},
		Author:    "dev1",
		Assignees: []string{"dev2", "dev3"},
		Reviewers: []string{"reviewer1"},
		Additions: 250,
		Deletions: 100,
		Comments:  10,
		Body:      "This is the PR description",
	}

	tests := []struct {
		name   string
		detail forge.PRDetail
		want   []string
		absent []string
	}{
		{
			name:   "a full detail renders every field and the action hints",
			detail: full,
			want: []string{
				"PR #456", "Add amazing feature", "dev1", "dev2, dev3", "reviewer1",
				"feature/amazing", mainBranchName, "+250", "-100", "This is the PR description",
				"copy URL", "copy [n]umber", "copy [b]ranch",
			},
			absent: []string{"loading details"},
		},
		{
			name:   "an unloaded detail says it is loading",
			detail: forge.PRDetail{},
			want:   []string{"Loading PR details"},
		},
		{
			name: "list-only data renders at once and flags the rest as loading",
			detail: forge.PRDetail{PullRequest: forge.PullRequest{
				Number: 100, Title: "Test PR", State: "OPEN",
				HeadRef: featureBranchName, BaseRef: mainBranchName,
			}},
			want: []string{"PR #100", "Test PR", featureBranchName, mainBranchName, "loading details"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			output := plainText(prDetailModel(&tt.detail).renderScreen())

			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("expected %q in the output", want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(output, absent) {
					t.Errorf("did not expect %q in the output", absent)
				}
			}
		})
	}
}

func TestPRDetailViewShowsStatusMessage(t *testing.T) {
	t.Parallel()
	m := prDetailModel(&forge.PRDetail{PullRequest: forge.PullRequest{
		Number: 123, Title: "Test PR", HeadRef: featureBranchName, BaseRef: mainBranchName,
	}})
	m.statusMessage = "Copied to clipboard: #123"

	if output := m.renderScreen(); !strings.Contains(output, m.statusMessage) {
		t.Error("the PR detail view should render the status message")
	}
}

func TestStatusMessageHandling(t *testing.T) {
	t.Parallel()
	const url = "https://github.com/test/pr/123"

	tests := []struct {
		name string
		msg  tea.Msg
		want []string
	}{
		{"a plain status shows verbatim", StatusMsg{Message: "Test status"}, []string{"Test status"}},
		{"a copy names the clipboard and the text", CopySuccessMsg{Text: url}, []string{"Copied to clipboard", url}},
		{"an open names the browser and the URL", URLOpenedMsg{URL: url}, []string{"Opened in browser", url}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := afterUpdate(t, New(nil, 1), tt.msg)

			for _, want := range tt.want {
				if !strings.Contains(m.statusMessage, want) {
					t.Errorf("expected %q in %q", want, m.statusMessage)
				}
			}

			m = afterUpdate(t, m, ClearStatusMsg{})
			if m.statusMessage != "" {
				t.Errorf("ClearStatusMsg left %q behind", m.statusMessage)
			}
		})
	}
}

func TestPRCountLoadedAccumulates(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)

	m = afterUpdate(t, m, PRCountLoadedMsg{Path: testRepo1Path, Count: 5})
	m = afterUpdate(t, m, PRCountLoadedMsg{Path: "/repo2", Count: 3})

	if m.prCount[testRepo1Path] != 5 {
		t.Errorf("expected 5 PRs for repo1, got %d", m.prCount[testRepo1Path])
	}
	if m.prCount["/repo2"] != 3 {
		t.Errorf("expected 3 PRs for repo2, got %d", m.prCount["/repo2"])
	}
}

// TestOpenPRFromTheTab covers the progressive step: opening a pull request from
// the PRs tab seats the row's own fields immediately, so the view has something
// to render before the detail fetch lands, and any detail left from a previous
// pull request goes.
func TestOpenPRFromTheTab(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.viewMode = ViewModePRList
	m.selectedRepo = testRepoPath
	m.summaries[testRepoPath] = models.RepoSummary{Path: testRepoPath}
	m.prSearch = []forge.PullRequest{{
		Number: 456, Title: "Feature PR", State: "OPEN",
		URL: "https://github.com/test/pr/456", HeadRef: featureBranchName,
		BaseRef: mainBranchName, ReviewDecision: "APPROVED",
	}}
	m.prDetail = forge.PRDetail{PullRequest: forge.PullRequest{Number: 999}}

	m = afterUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.viewMode != ViewModePRDetail {
		t.Error("opening a PR should switch to the PR detail view")
	}
	if m.selectedPR.Number != 456 || m.prDetail.Number != 456 {
		t.Errorf("stale detail survived: selected #%d, detail #%d", m.selectedPR.Number, m.prDetail.Number)
	}
	if m.prDetail.Title != "Feature PR" || m.prDetail.State != "OPEN" || m.prDetail.HeadRef != featureBranchName {
		t.Errorf("the list's own fields should seat immediately, got %+v", m.prDetail.PullRequest)
	}
	if m.prDetail.Author != "" || len(m.prDetail.Assignees) > 0 {
		t.Error("fields only the detail fetch supplies should stay empty until it lands")
	}
}
