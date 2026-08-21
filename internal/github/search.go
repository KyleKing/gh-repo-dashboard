package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// SearchLimit caps how many pull requests one saved view reads. A view is a
// working set, not an archive: past this many rows the answer is to narrow the
// query rather than to scroll.
const SearchLimit = 50

// searchFields are what a fleet-wide search can report. It runs against
// GitHub's search index rather than against one repository, so there is no
// check rollup and no head ref in the answer.
const searchFields = "number,title,url,repository,author,state,isDraft,updatedAt"

// PRSearchCacheKey scopes a search result by where it ran. A fleet search has
// no repository of its own, so it keys on the query alone and every repo in
// the fleet shares the one answer.
func PRSearchCacheKey(scope, query string) string {
	return "pr_search\x00" + scope + "\x00" + query
}

// SearchPRsInRepo runs a saved view's query against one repository, returning
// the same shape the repo's own list does, checks included.
func SearchPRsInRepo(ctx context.Context, repoPath, remoteID, query string) ([]forge.PullRequest, error) {
	key := PRSearchCacheKey(cache.RemoteScope(repoPath, remoteID), query)
	if cached, ok := cache.PRSearchCache.Get(key, vcs.Stamp(repoPath)); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	found, err := prListPage(ctx, repoPath, env, "--search", query, "--state", "all")
	if err != nil {
		return nil, err
	}

	cache.PRSearchCache.Set(key, vcs.Stamp(repoPath), found)

	return found, nil
}

// SearchPRsEverywhere runs a saved view's query across every repository the
// search reaches, which is what makes a view like review-requested:@me worth
// having. Rows carry their repository and lack the per-check detail a
// repo-scoped read has, since the search index does not report it.
func SearchPRsEverywhere(ctx context.Context, repoPath, query string) ([]forge.PullRequest, error) {
	key := PRSearchCacheKey("fleet", query)
	if cached, ok := cache.PRSearchCache.Get(key, cache.NoStamp); ok {
		return cached, nil
	}

	args := append([]string{"search", "prs"}, FleetSearchArgs(query)...)
	args = append(args, "--json", searchFields, "--limit", strconv.Itoa(SearchLimit))

	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), args...)
	if err != nil {
		return nil, err
	}

	found, err := parseSearchResults(out)
	if err != nil {
		return nil, err
	}

	cache.PRSearchCache.Set(key, cache.NoStamp, found)

	return found, nil
}

func parseSearchResults(out []byte) ([]forge.PullRequest, error) {
	//nolint:tagliatelle // gh speaks camelCase, and these tags name its fields
	var results []struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		URL        string `json:"url"`
		State      string `json:"state"`
		IsDraft    bool   `json:"isDraft"`
		Repository struct {
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		UpdatedAt time.Time `json:"updatedAt"`
	}

	if err := json.Unmarshal(out, &results); err != nil {
		return nil, fmt.Errorf("parsing gh search prs output: %w", err)
	}

	prs := make([]forge.PullRequest, 0, len(results))
	for i := range results {
		r := &results[i]
		prs = append(prs, forge.PullRequest{
			Number:    r.Number,
			Title:     r.Title,
			URL:       r.URL,
			State:     strings.ToUpper(r.State),
			IsDraft:   r.IsDraft,
			Repo:      r.Repository.NameWithOwner,
			Author:    r.Author.Login,
			UpdatedAt: r.UpdatedAt,
		})
	}

	return prs, nil
}

// FleetSearchArgs turns a saved view's query into the arguments gh search prs
// takes: its terms, a subject if the view named none, and the sort as flags.
func FleetSearchArgs(query string) []string {
	terms, sortArgs := splitSortQualifier(query)

	return append(withSubject(terms), sortArgs...)
}

// scopingQualifiers name a subject a search is about. A query carrying none of
// them is scoped by the repository it runs in, which is exactly what a
// fleet-wide search drops.
var scopingQualifiers = []string{
	"assignee:", "author:", "commenter:", "involves:", "mentions:", "org:",
	"repo:", "review-requested:", "reviewed-by:", "team-review-requested:", "user:",
}

// withSubject keeps a widened search from matching every open pull request on
// GitHub. A view written for one repo says nothing about whose work it is, so
// widening it without a subject would answer a question nobody asked.
func withSubject(terms []string) []string {
	for _, term := range terms {
		for _, qualifier := range scopingQualifiers {
			if strings.HasPrefix(strings.TrimPrefix(term, "-"), qualifier) {
				return terms
			}
		}
	}

	return append(terms, "involves:@me")
}

// splitSortQualifier separates a sort: qualifier from the search terms. The
// search API reads it as part of the query and gh search reads it as two
// flags, so a view written for one works in the other.
//
//nolint:nonamedreturns // the two lists are told apart by name, not by order
func splitSortQualifier(query string) (terms, sortArgs []string) {
	for _, term := range strings.Fields(query) {
		value, isSort := strings.CutPrefix(term, "sort:")
		if !isSort {
			terms = append(terms, term)

			continue
		}

		field, order, hasOrder := strings.Cut(value, "-")
		sortArgs = append(sortArgs, "--sort", field)
		if hasOrder {
			sortArgs = append(sortArgs, "--order", order)
		}
	}

	return terms, sortArgs
}
