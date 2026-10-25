//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

func TestDetailListLen(t *testing.T) {
	t.Parallel()
	m := New(nil, 1)
	m.selectedRepo = testRepo1Path
	m.branches = make([]vcs.BranchInfo, 5)
	m.stashes = make([]vcs.StashDetail, 3)
	m.worktrees = []vcs.WorktreeInfo{
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
			if got := display.PRStatusDisplay(tt.pr); got != tt.want {
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
		{
			"an approval with no decision still reads as approved",
			forge.PullRequest{ApprovedBy: []string{"u1"}},
			"approved",
		},
		{"no review", forge.PullRequest{}, emDash},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := display.PRReviewStatus(tt.pr); got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
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
