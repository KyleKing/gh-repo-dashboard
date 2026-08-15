package filters

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sahilm/fuzzy"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

const fuzzyThreshold = 0.6

// searchScope narrows SearchRepos to just one field, for a query like "main"
// that would otherwise match both an unrelated repo named "main" and every
// repo sitting on that branch.
type searchScope int

const (
	scopeAny searchScope = iota
	scopeName
	scopeBranch
	scopePR
	scopeTemplate
	scopeCommit
)

// parseSearchScope reads an optional scope prefix off searchText, so a reader
// can scope a search the same way the command palette already lets a query
// start with a kind letter: "r:" name, "b:" branch, "p:" PR number/title,
// "t:" copier template, "c:" commit recency.
//
//nolint:gocritic // named returns are banned project-wide, so these two stay unnamed
func parseSearchScope(searchText string) (searchScope, string) {
	switch {
	case strings.HasPrefix(searchText, "r:"):
		return scopeName, searchText[2:]
	case strings.HasPrefix(searchText, "b:"):
		return scopeBranch, searchText[2:]
	case strings.HasPrefix(searchText, "p:"):
		return scopePR, searchText[2:]
	case strings.HasPrefix(searchText, "t:"):
		return scopeTemplate, searchText[2:]
	case strings.HasPrefix(searchText, "c:"):
		return scopeCommit, searchText[2:]
	default:
		return scopeAny, searchText
	}
}

// matchesScope reports whether name or branch (already lowercased) matches
// query via GlobMatch, narrowed to just the field scope names.
func matchesScope(scope searchScope, name, branch, query string) bool {
	switch scope {
	case scopeName:
		return GlobMatch(query, name)
	case scopeBranch:
		return GlobMatch(query, branch)
	default:
		return GlobMatch(query, name) || GlobMatch(query, branch)
	}
}

// GlobMatch reports whether target matches a search query using shell-glob
// syntax: "*" matches any run of characters (including "/", unlike
// path.Match, since branch names and template sources both use it) and "?"
// matches any single character; both are literal when escaped ("\*", "\?").
// The query is auto-wrapped with "*" on whichever end isn't pinned by a
// leading "^" or trailing "$" anchor, so a bare query keeps the "found
// anywhere" convenience of a plain substring search while "^"/"$" (or both,
// for an exact match) opt into pinning an edge. A literal "^" or "$" is
// written escaped too: "\^", "\$". Actual matching is delegated to Go's
// regexp engine once the glob syntax is translated.
func GlobMatch(query, target string) bool {
	re, err := regexp.Compile(globRegexp(query))

	return err == nil && re.MatchString(target)
}

// globRegexp turns a search query into the anchored regular expression
// GlobMatch runs: the "^"/"$" auto-wrap rule described on GlobMatch, then
// "*"/"?"/escapes translated into regexp syntax via QuoteMeta for every
// literal run.
func globRegexp(query string) string {
	prefixAnchor := false

	switch {
	case strings.HasPrefix(query, `\^`):
		query = "^" + query[2:]
	case strings.HasPrefix(query, "^"):
		prefixAnchor = true
		query = query[1:]
	}

	suffixAnchor := false

	switch {
	case strings.HasSuffix(query, `\$`):
		query = query[:len(query)-2] + "$"
	case strings.HasSuffix(query, "$"):
		suffixAnchor = true
		query = query[:len(query)-1]
	}

	var b strings.Builder

	b.WriteByte('^')

	if !prefixAnchor {
		b.WriteString(".*")
	}

	escaped := false

	for i := range len(query) {
		c := query[i]

		switch {
		case escaped:
			b.WriteString(regexp.QuoteMeta(string(c)))
			escaped = false
		case c == '\\':
			escaped = true
		case c == '*':
			b.WriteString(".*")
		case c == '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}

	if escaped {
		b.WriteString(regexp.QuoteMeta(`\`))
	}

	if !suffixAnchor {
		b.WriteString(".*")
	}

	b.WriteByte('$')

	return b.String()
}

// SearchRepos filters paths by searchText. With no scope prefix (or "r:"/
// "b:"), it matches the repo name or checked-out branch, preferring
// substring matches and falling back to fuzzy matching on the repo name when
// there are none. "p:", "t:", and "c:" scope to a PR number/title, a copier
// template, or commit recency instead, with no fuzzy fallback (an exact
// field either matches or it doesn't).
func SearchRepos(paths []string, summaries map[string]models.RepoSummary, searchText string) []string {
	if searchText == "" {
		return paths
	}

	scope, query := parseSearchScope(searchText)

	switch scope {
	case scopePR:
		return filterByPR(paths, summaries, query)
	case scopeTemplate:
		return filterByTemplate(paths, summaries, query)
	case scopeCommit:
		return filterByCommitAge(paths, summaries, query)
	case scopeAny, scopeName, scopeBranch:
		// Handled by the substring/fuzzy path below.
	}

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

// filterByPR matches a repo's associated pull request: an exact number for
// "p:123", otherwise a case-insensitive title substring.
func filterByPR(paths []string, summaries map[string]models.RepoSummary, query string) []string {
	if num, err := strconv.Atoi(query); err == nil {
		var results []string

		for _, path := range paths {
			if pr := summaries[path].PRInfo; pr != nil && pr.Number == num {
				results = append(results, path)
			}
		}

		return results
	}

	queryLower := strings.ToLower(query)

	var results []string

	for _, path := range paths {
		if pr := summaries[path].PRInfo; pr != nil && GlobMatch(queryLower, strings.ToLower(pr.Title)) {
			results = append(results, path)
		}
	}

	return results
}

// filterByTemplate matches a repo's copier template source, "t:my_go_template".
func filterByTemplate(paths []string, summaries map[string]models.RepoSummary, query string) []string {
	queryLower := strings.ToLower(query)

	var results []string

	for _, path := range paths {
		info := summaries[path].TemplateInfo
		if info != nil && GlobMatch(queryLower, strings.ToLower(info.SrcPath)) {
			results = append(results, path)
		}
	}

	return results
}

// recencyCmp is the comparison a "c:" search performs against a repo's last
// commit age: recencyWithin matches at or under the duration ("c:<7d", and
// the bare "c:7d" default), recencyOlderThan matches over it ("c:>30d").
type recencyCmp int

const (
	recencyWithin recencyCmp = iota
	recencyOlderThan
)

// hoursPerDay and daysPerWeek build the "d"/"w" duration units below, which
// time.ParseDuration doesn't accept on its own.
const (
	hoursPerDay = 24
	daysPerWeek = 7
)

// durationUnits maps a "c:" suffix to its duration.
var durationUnits = map[byte]time.Duration{
	'm': time.Minute,
	'h': time.Hour,
	'd': hoursPerDay * time.Hour,
	'w': daysPerWeek * hoursPerDay * time.Hour,
}

// errInvalidDuration reports a "c:" query whose duration can't be parsed.
var errInvalidDuration = errors.New("invalid duration")

// minDurationLen is the shortest a duration string can be: one digit plus its unit suffix.
const minDurationLen = 2

// parseRecencyDuration parses a bare "7d"/"30m"-style relative duration.
func parseRecencyDuration(s string) (time.Duration, error) {
	if len(s) < minDurationLen {
		return 0, fmt.Errorf("%w: %q (want e.g. 7d, 30m, 2w)", errInvalidDuration, s)
	}

	unit, ok := durationUnits[s[len(s)-1]]
	if !ok {
		return 0, fmt.Errorf("%w: unit in %q (want m/h/d/w)", errInvalidDuration, s)
	}

	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: %q (want e.g. 7d, 30m, 2w)", errInvalidDuration, s)
	}

	return time.Duration(n) * unit, nil
}

// parseRecencyQuery reads a "c:" query's optional "<"/">" comparison (bare
// defaults to "<", "within") and its duration.
func parseRecencyQuery(query string) (recencyCmp, time.Duration, error) {
	cmp := recencyWithin
	rest := query

	switch {
	case strings.HasPrefix(query, "<"):
		rest = query[1:]
	case strings.HasPrefix(query, ">"):
		cmp = recencyOlderThan
		rest = query[1:]
	}

	dur, err := parseRecencyDuration(rest)

	return cmp, dur, err
}

// filterByCommitAge matches repos by how long ago their last commit landed.
// An unparsable query matches nothing rather than erroring, the same way a
// live search box degrades for any other bad pattern.
func filterByCommitAge(paths []string, summaries map[string]models.RepoSummary, query string) []string {
	cmp, dur, err := parseRecencyQuery(query)
	if err != nil {
		return nil
	}

	var results []string

	for _, path := range paths {
		summary := summaries[path]
		if summary.LastModified.IsZero() {
			continue
		}

		age := time.Since(summary.LastModified)

		matches := age <= dur
		if cmp == recencyOlderThan {
			matches = age > dur
		}

		if matches {
			results = append(results, path)
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
