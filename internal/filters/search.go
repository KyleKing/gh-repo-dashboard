package filters

import (
	"path/filepath"
	"strings"

	"github.com/sahilm/fuzzy"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

const fuzzyThreshold = 0.6

// searchScope narrows SearchRepos to just the repo name or just the checked-out
// branch, for a query like "main" that would otherwise match both an
// unrelated repo named "main" and every repo sitting on that branch.
type searchScope int

const (
	scopeAny searchScope = iota
	scopeName
	scopeBranch
)

// parseSearchScope reads an optional "r:" or "b:" prefix off searchText, so a
// reader can scope a search the same way the command palette already lets a
// query start with a kind letter.
//
//nolint:gocritic // named returns are banned project-wide, so these two stay unnamed
func parseSearchScope(searchText string) (searchScope, string) {
	switch {
	case strings.HasPrefix(searchText, "r:"):
		return scopeName, searchText[2:]
	case strings.HasPrefix(searchText, "b:"):
		return scopeBranch, searchText[2:]
	default:
		return scopeAny, searchText
	}
}

// matchesScope reports whether name or branch (already lowercased) contains
// query, narrowed to just the field scope names.
func matchesScope(scope searchScope, name, branch, query string) bool {
	switch scope {
	case scopeName:
		return strings.Contains(name, query)
	case scopeBranch:
		return strings.Contains(branch, query)
	default:
		return strings.Contains(name, query) || strings.Contains(branch, query)
	}
}

// SearchRepos filters paths to those whose repo name or checked-out branch
// matches searchText, preferring substring matches and falling back to fuzzy
// matching on the repo name when there are none. See parseSearchScope for
// narrowing the match to just one field.
func SearchRepos(paths []string, summaries map[string]models.RepoSummary, searchText string) []string {
	if searchText == "" {
		return paths
	}

	scope, query := parseSearchScope(searchText)
	queryLower := strings.ToLower(query)

	var substringMatches []string
	var nonMatches []string

	for _, path := range paths {
		name := strings.ToLower(filepath.Base(path))
		branch := strings.ToLower(summaries[path].Branch)

		if matchesScope(scope, name, branch, queryLower) {
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

	matches := fuzzy.Find(query, names)

	var results []string
	for _, match := range matches {
		score := float64(match.Score) / float64(len(query)*len(names[match.Index]))
		if score >= fuzzyThreshold || match.Score > 0 {
			results = append(results, nonMatches[match.Index])
		}
	}

	return results
}

// FuzzyMatch reports whether text matches pattern via substring or fuzzy matching.
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
