package filters

import "github.com/kyleking/gh-repo-dashboard/internal/models"

func FilterRepos(paths []string, summaries map[string]models.RepoSummary, mode models.FilterMode) []string {
	if mode == models.FilterModeAll {
		return paths
	}

	var filtered []string
	for _, path := range paths {
		summary, ok := summaries[path]
		if !ok {
			continue
		}

		if passesFilter(summary, mode) {
			filtered = append(filtered, path)
		}
	}

	return filtered
}

func FilterReposMulti(paths []string, summaries map[string]models.RepoSummary, activeFilters []models.ActiveFilter) []string {
	enabledFilters := []models.ActiveFilter{}
	for _, f := range activeFilters {
		if f.Enabled && f.Mode != models.FilterModeAll {
			enabledFilters = append(enabledFilters, f)
		}
	}

	if len(enabledFilters) == 0 {
		return paths
	}

	var filtered []string
	for _, path := range paths {
		summary, ok := summaries[path]
		if !ok {
			continue
		}

		passesAll := true
		for _, f := range enabledFilters {
			passes := passesFilter(summary, f.Mode)
			if f.Inverted {
				passes = !passes
			}
			if !passes {
				passesAll = false
				break
			}
		}

		if passesAll {
			filtered = append(filtered, path)
		}
	}

	return filtered
}

func passesFilter(s models.RepoSummary, mode models.FilterMode) bool {
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
	default:
		return true
	}
}

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
