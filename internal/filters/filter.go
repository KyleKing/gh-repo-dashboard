// Package filters implements repo list filtering, fuzzy search, and sorting.
// The generic half is aragonite/filter — the predicate type, the and/or/not
// combinators, and Keep are its. What stays here is the repo's own domain:
// the GlobMatch syntax the search box spells, the scope prefixes it takes,
// the recency queries, and the predicate-atom language.
package filters

import (
	"github.com/kyleking/aragonite/filter"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// FilterRepos returns the subset of paths whose summary passes the single filter mode.
func FilterRepos(paths []string, summaries map[string]models.RepoSummary, mode models.FilterMode) []string {
	if mode == models.FilterModeAll {
		return paths
	}

	return filter.Keep(paths, func(path string) bool {
		summary, ok := summaries[path]
		return ok && passesFilter(&summary, mode)
	})
}

// FilterReposMulti returns the subset of paths whose summary passes every enabled filter, honoring inversion.
func FilterReposMulti(
	paths []string, summaries map[string]models.RepoSummary, activeFilters []models.ActiveFilter,
) []string {
	enabledFilters := []models.ActiveFilter{}
	for _, f := range activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			enabledFilters = append(enabledFilters, f)
		}
	}

	if len(enabledFilters) == 0 {
		return paths
	}

	return filter.Keep(paths, func(path string) bool {
		summary, ok := summaries[path]
		if !ok {
			return false
		}

		for _, f := range enabledFilters {
			passes := passesFilter(&summary, f.Mode)
			if f.Inverted {
				passes = !passes
			}
			if !passes {
				return false
			}
		}

		return true
	})
}

func passesFilter(s *models.RepoSummary, mode models.FilterMode) bool {
	switch mode {
	case models.FilterModeAll:
		return true
	case models.FilterModeAhead:
		return s.Ahead > 0
	case models.FilterModeBehind:
		return s.Behind > 0
	case models.FilterModeDirty:
		return s.IsDirty()
	case models.FilterModeHasPR:
		return s.PRInfo != nil
	case models.FilterModeHasStash:
		return s.StashCount > 0
	case models.FilterModeHasNotes:
		return s.HasNotes()
	case models.FilterModeGit:
		return s.VCSType == vcs.TypeGit
	default:
		return true
	}
}

// FilterAndSort filters paths by a single mode and search text, then sorts the result.
func FilterAndSort(
	paths []string,
	summaries map[string]models.RepoSummary,
	filterMode models.FilterMode,
	sortMode models.SortMode,
	searchText string,
	reverse bool,
) []string {
	filtered := FilterRepos(paths, summaries, filterMode)

	if searchText != "" {
		filtered = SearchRepos(filtered, summaries, searchText)
	}

	sorted := SortPaths(filtered, summaries, sortMode, reverse)

	return sorted
}

// FilterAndSortMulti filters paths by multiple active filters and search text, then sorts by multiple active sorts.
func FilterAndSortMulti(
	paths []string,
	summaries map[string]models.RepoSummary,
	activeFilters []models.ActiveFilter,
	activeSorts []models.ActiveSort,
	searchText string,
) []string {
	filtered := FilterReposMulti(paths, summaries, activeFilters)

	if searchText != "" {
		filtered = SearchRepos(filtered, summaries, searchText)
	}

	sorted := SortPathsMulti(filtered, summaries, activeSorts)

	return sorted
}
