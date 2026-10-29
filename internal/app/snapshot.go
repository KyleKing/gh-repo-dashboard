package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/snapshot"
)

// seedSnapshot repaints the row the checkout last drew where one was kept and
// the checkout has not moved since. Everything it fills is read again by the
// loads discovery just started, so a seeded row cannot outlive them.
func (m *Model) seedSnapshot(path string) {
	row, _, ok := m.snapshots.Load(vcs.RemoteIdentityAt(path), path, vcs.Stamp(path))
	if !ok {
		return
	}

	summary := row.Summary
	summary.Path = path
	m.summaries[path] = summary
	m.branchCount[path] = row.BranchCount
	m.due[path] = row.Due
	m.changedPaths[path] = row.ChangedPaths

	for _, kind := range []fetchKind{fetchPR, fetchOpenPRs, fetchCI, fetchTemplate, fetchBranchCount} {
		m.freshness[fetchKey{path, kind}] = freshnessStale
	}
}

// snapshotCmd records the row as it currently stands, stamped with what the
// checkout looks like, so the next run paints it before any read lands. It
// runs off the UI thread because a save rewrites the upstream's whole file.
func (m Model) snapshotCmd(path string) tea.Cmd {
	summary, ok := m.summaries[path]
	if !ok || m.snapshots == nil {
		return nil
	}

	row := snapshot.Row{
		Summary:      summary,
		BranchCount:  m.branchCount[path],
		Due:          m.due[path],
		ChangedPaths: m.changedPaths[path],
	}
	store := m.snapshots

	return func() tea.Msg {
		store.Save(summary.RemoteID, path, vcs.Stamp(path), row)

		return nil
	}
}
