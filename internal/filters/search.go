package filters

import (
	"path/filepath"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/sahilm/fuzzy"
)

const fuzzyThreshold = 0.6

func SearchRepos(paths []string, summaries map[string]models.RepoSummary, searchText string) []string {
	if searchText == "" {
		return paths
	}

	searchLower := strings.ToLower(searchText)

	var substringMatches []string
	var nonMatches []string

	for _, path := range paths {
		name := strings.ToLower(filepath.Base(path))
		if strings.Contains(name, searchLower) {
			substringMatches = append(substringMatches, path)
		} else {
			nonMatches = append(nonMatches, path)
		}
	}

	if len(substringMatches) > 0 {
		return substringMatches
	}

	names := make([]string, len(nonMatches))
	for i, path := range nonMatches {
		names[i] = filepath.Base(path)
	}

	matches := fuzzy.Find(searchText, names)

	var results []string
	for _, match := range matches {
		score := float64(match.Score) / float64(len(searchText)*len(names[match.Index]))
		if score >= fuzzyThreshold || match.Score > 0 {
			results = append(results, nonMatches[match.Index])
		}
	}

	return results
}

func FuzzyMatch(pattern, text string) bool {
	if pattern == "" {
		return true
	}

	patternLower := strings.ToLower(pattern)
	textLower := strings.ToLower(text)

	if strings.Contains(textLower, patternLower) {
		return true
	}

	matches := fuzzy.Find(pattern, []string{text})
	return len(matches) > 0 && matches[0].Score > 0
}
