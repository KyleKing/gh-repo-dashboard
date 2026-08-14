//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// loadingFleetModel drives a two-repo fleet from discovery through both
// summaries, which is the point where m.loading drops while the fetches those
// summaries started are still out.
func loadingFleetModel(t *testing.T) Model {
	t.Helper()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = 120, 30

	paths := []string{"/dev/alpha", "/dev/bravo"}
	updated, _ := m.Update(ReposDiscoveredMsg{Paths: paths})
	m = mustModel(t, updated)

	for _, path := range paths {
		updated, _ = m.Update(RepoSummaryLoadedMsg{Path: path, Summary: models.RepoSummary{
			Path:     path,
			Branch:   mainBranchName,
			Upstream: "origin/main",
			RemoteID: "dev/" + path,
		}})
		m = mustModel(t, updated)
	}

	if m.loading {
		t.Fatal("both summaries landed but the model still reports discovery in flight")
	}

	return m
}

// settleFetches feeds back every message the outstanding fetches would answer
// with, so the spinner's other direction is exercised by real messages rather
// than by clearing the map.
func settleFetches(t *testing.T, start *Model) Model {
	t.Helper()

	m := *start
	for key := range m.fetching {
		var msg tea.Msg
		switch key.kind {
		case fetchPR:
			msg = PRLoadedMsg{Path: key.path}
		case fetchPRCount:
			msg = PRCountLoadedMsg{Path: key.path}
		case fetchBranchCount:
			msg = BranchCountLoadedMsg{Path: key.path}
		case fetchTemplate:
			msg = CopierInfoLoadedMsg{Path: key.path}
		case fetchCI:
			msg = WorkflowLoadedMsg{Path: key.path, Error: errGHFailed}
		case fetchExpand:
			msg = PRMapLoadedMsg{Path: key.path}
		case fetchPeerBranches:
			msg = PeerBranchesLoadedMsg{Path: key.path}
		}

		updated, _ := m.Update(msg)
		m = mustModel(t, updated)
	}

	return m
}

func TestSecondaryFetchesKeepTheDashboardLoading(t *testing.T) {
	t.Parallel()

	m := loadingFleetModel(t)

	if m.fetchesInFlight() == 0 {
		t.Fatal("no fetch was recorded for the PR, PR count, template, and CI reads the summaries started")
	}
	if !m.anyLoading() {
		t.Error("the spinner stopped while per-repo fetches were still out")
	}
	if got := m.loadingBadge(); got == "" {
		t.Error("the progress badge went away while per-repo fetches were still out")
	}

	m = settleFetches(t, &m)

	if got := m.fetchesInFlight(); got != 0 {
		t.Fatalf("%d fetches still recorded after every one answered", got)
	}
	if m.anyLoading() {
		t.Error("the spinner never settled, so the dashboard repaints on a timer forever")
	}
	if got := m.loadingBadge(); got != "" {
		t.Errorf("progress badge = %q after everything settled, want it gone", got)
	}
}

func TestPendingCellsAreDistinctFromAbsentOnes(t *testing.T) {
	t.Parallel()

	// Two fleets rather than one: a Model's maps are shared by reference, so
	// settling one would empty the other's record of what is in flight.
	pending := loadingFleetModel(t)
	other := loadingFleetModel(t)
	settled := settleFetches(t, &other)

	path := "/dev/alpha"
	cells := []struct {
		name string
		cell func(Model) string
	}{
		{"PR", func(m Model) string { return m.prCell(m.summaries[path], maxContentWidth) }},
		{"template", func(m Model) string { return m.templateCell(m.summaries[path], maxContentWidth) }},
		{"CI", func(m Model) string {
			text := m.ciCell(m.summaries[path])

			return text
		}},
	}

	for _, cell := range cells {
		t.Run(cell.name, func(t *testing.T) {
			t.Parallel()

			if got := plainText(cell.cell(pending)); got != pendingGlyph {
				t.Errorf("in-flight %s cell = %q, want %q", cell.name, got, pendingGlyph)
			}
			if got := cell.cell(settled); got != emDash {
				t.Errorf("settled %s cell = %q, want %q", cell.name, got, emDash)
			}
		})
	}
}

// A repo discovered but not yet read has no state to report, and the row must
// not claim a clean tree on its behalf.
func TestUnreadSummaryRowsReadAsPending(t *testing.T) {
	t.Parallel()

	m := New([]string{"/dev"}, 1)
	m.width, m.height = 120, 30
	updated, _ := m.Update(ReposDiscoveredMsg{Paths: []string{"/dev/alpha", "/dev/bravo"}})
	m = mustModel(t, updated)

	summary := m.rowSummary("/dev/alpha")
	if summary.Name() != "alpha" {
		t.Errorf("unread row names the repo %q, want %q", summary.Name(), "alpha")
	}

	if got := plainText(m.statusCell(summary, 12, plainStyle, false)); !strings.Contains(got, pendingGlyph) {
		t.Errorf("unread status cell = %q, want the pending glyph %q", got, pendingGlyph)
	}

	rows := m.overviewRows(summary, false)
	for _, row := range rows {
		if row.value != readingLabel {
			t.Errorf("overview row %q = %q while the summary is unread, want %q",
				row.label, row.value, readingLabel)
		}
	}
}
