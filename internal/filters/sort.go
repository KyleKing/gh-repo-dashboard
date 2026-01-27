package filters

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func SortPaths(paths []string, summaries map[string]models.RepoSummary, mode models.SortMode, reverse bool) []string {
	if len(paths) == 0 {
		return paths
	}

	sorted := make([]string, len(paths))
	copy(sorted, paths)

	sort.Slice(sorted, func(i, j int) bool {
		si := summaries[sorted[i]]
		sj := summaries[sorted[j]]

		less := comparePaths(si, sj, mode)
		if reverse {
			return !less
		}
		return less
	})

	return sorted
}

func comparePaths(a, b models.RepoSummary, mode models.SortMode) bool {
	switch mode {
	case models.SortModeName:
		return compareByName(a, b)
	case models.SortModeModified:
		return compareByModified(a, b)
	case models.SortModeStatus:
		return compareByStatus(a, b)
	case models.SortModeBranch:
		return compareByBranch(a, b)
	default:
		return compareByName(a, b)
	}
}

func compareByName(a, b models.RepoSummary) bool {
	return strings.ToLower(filepath.Base(a.Path)) < strings.ToLower(filepath.Base(b.Path))
}

func compareByModified(a, b models.RepoSummary) bool {
	if a.LastModified.Equal(b.LastModified) {
		return compareByName(a, b)
	}
	return a.LastModified.After(b.LastModified)
}

func compareByStatus(a, b models.RepoSummary) bool {
	aDirty := a.IsDirty()
	bDirty := b.IsDirty()

	if aDirty != bDirty {
		return aDirty
	}

	aCount := a.UncommittedCount()
	bCount := b.UncommittedCount()
	if aCount != bCount {
		return aCount > bCount
	}

	return compareByName(a, b)
}

func compareByBranch(a, b models.RepoSummary) bool {
	if a.Branch != b.Branch {
		return strings.ToLower(a.Branch) < strings.ToLower(b.Branch)
	}
	return compareByName(a, b)
}

func SortPathsMulti(paths []string, summaries map[string]models.RepoSummary, activeSorts []models.ActiveSort) []string {
	if len(paths) == 0 {
		return paths
	}

	enabledSorts := []models.ActiveSort{}
	for _, s := range activeSorts {
		if s.IsEnabled() {
			enabledSorts = append(enabledSorts, s)
		}
	}

	if len(enabledSorts) == 0 {
		return paths
	}

	sort.Slice(enabledSorts, func(i, j int) bool {
		return enabledSorts[i].Priority < enabledSorts[j].Priority
	})

	sorted := make([]string, len(paths))
	copy(sorted, paths)

	sort.Slice(sorted, func(i, j int) bool {
		si := summaries[sorted[i]]
		sj := summaries[sorted[j]]

		for _, activeSort := range enabledSorts {
			less := comparePaths(si, sj, activeSort.Mode)
			if activeSort.Direction == models.SortDirectionDesc {
				less = !less
			}

			greater := comparePaths(sj, si, activeSort.Mode)
			if activeSort.Direction == models.SortDirectionDesc {
				greater = !greater
			}

			if less != greater {
				return less
			}
		}

		return false
	})

	return sorted
}
