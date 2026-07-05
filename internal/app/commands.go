package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
)

func refreshCmd(_ []string, _ int) tea.Cmd {
	return func() tea.Msg {
		cache.ClearAll()
		return nil
	}
}

func batchFetchAllCmd(paths []string) tea.Cmd {
	return batch.RunTask("Fetch All", paths, batch.FetchAll)
}

func batchPruneRemoteCmd(paths []string) tea.Cmd {
	return batch.RunTask("Prune Remote", paths, batch.PruneRemote)
}

func batchCleanupMergedCmd(paths []string) tea.Cmd {
	return batch.RunTask("Cleanup Merged", paths, batch.CleanupMerged)
}
