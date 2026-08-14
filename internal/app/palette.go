package app

import (
	"strconv"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
)

// findKind is the object type a palette query targets. A query with no prefix
// searches every kind at once.
type findKind int

// Find kinds, in the order results are grouped.
const (
	findAny findKind = iota
	findRepo
	findBranch
	findPR
	findStash
	findNote
)

// findPrefixes map a query's leading letter to the kind it narrows to. The
// letters are the first letter of each object's name, so the grammar is
// guessable rather than memorized.
var findPrefixes = map[string]findKind{
	"b": findBranch,
	"n": findNote,
	"r": findRepo,
	"s": findStash,
}

// fleetPrefix widens a focused-view query to every repo, and nameFind labels
// the palette wherever it is named.
const (
	fleetPrefix = "*"
	nameFind    = "find"
)

// findQuery is a parsed palette query.
type findQuery struct {
	kind findKind
	text string
	// number is the PR number a "#12" query asks for, or zero.
	number int
	// fleet widens a repo-scoped query to the whole fleet.
	fleet bool
}

// parseFindQuery reads the palette's typed-prefix grammar: "#12" is a PR
// number, a one-letter prefix followed by a space narrows to that type, a
// leading "*" widens the scope, and anything else searches everything.
func parseFindQuery(raw string) findQuery {
	q := findQuery{}

	text := strings.TrimSpace(raw)
	if rest, ok := strings.CutPrefix(text, fleetPrefix); ok {
		q.fleet = true
		text = strings.TrimSpace(rest)
	}

	if rest, ok := strings.CutPrefix(text, "#"); ok {
		q.kind = findPR
		q.text = strings.TrimSpace(rest)
		q.number, _ = strconv.Atoi(q.text) //nolint:errcheck // a non-numeric tail just matches PR titles

		return q
	}

	if prefix, rest, found := strings.Cut(text, " "); found {
		if kind, ok := findPrefixes[prefix]; ok {
			q.kind = kind
			q.text = strings.TrimSpace(rest)

			return q
		}
	}

	q.text = text

	return q
}

// wants reports whether the query is asking for results of this kind.
func (q findQuery) wants(kind findKind) bool {
	return q.kind == findAny || q.kind == kind
}

// matches reports whether any field matches the query text, fuzzily: the
// same substring-then-fuzzy matching the Repos list's own search already
// uses, so typing fragments of a branch or PR title out of order still finds
// it here the way it would there.
func (q findQuery) matches(fields ...string) bool {
	if q.text == "" {
		return true
	}

	for _, field := range fields {
		if filters.FuzzyMatch(q.text, field) {
			return true
		}
	}

	return false
}

// findResult is one palette row: what it is, where it lives, and enough state
// for the default action to act on it.
type findResult struct {
	kind   findKind
	repo   string
	label  string
	detail string
	branch string
	number int
	index  int
}

// key identifies a result across re-queries, so a mark survives the user
// typing another character.
func (r *findResult) key() string {
	return strconv.Itoa(int(r.kind)) + "\x00" + r.repo + "\x00" + r.label
}

func (r *findResult) kindName() string {
	switch r.kind {
	case findRepo:
		return "repo"
	case findBranch:
		return nameBranch
	case findPR:
		return "PR"
	case findStash:
		return "stash"
	case findNote:
		return "note"
	case findAny:
		return ""
	}

	return ""
}

// homogeneousKind returns the one kind every result shares, or findAny when
// the set is mixed. The action menu offers type-specific verbs only when the
// set has a type.
func homogeneousKind(results []findResult) findKind {
	if len(results) == 0 {
		return findAny
	}

	kind := results[0].kind
	for _, r := range results[1:] {
		if r.kind != kind {
			return findAny
		}
	}

	return kind
}

// resultRepos returns the distinct repo paths a result set touches, in the
// order they first appear.
func resultRepos(results []findResult) []string {
	seen := make(map[string]bool, len(results))

	paths := make([]string, 0, len(results))
	for _, r := range results {
		if r.repo == "" || seen[r.repo] {
			continue
		}
		seen[r.repo] = true
		paths = append(paths, r.repo)
	}

	return paths
}
