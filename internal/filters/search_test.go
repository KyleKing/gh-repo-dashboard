package filters_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestSearchReposEmpty(t *testing.T) {
	t.Parallel()
	paths := []string{testRepo1Path, "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{}

	result := filters.SearchRepos(paths, summaries, "")
	if len(result) != 3 {
		t.Errorf("expected 3 repos with empty search, got %d", len(result))
	}
}

func TestSearchReposSubstring(t *testing.T) {
	t.Parallel()
	paths := []string{"/api-service", "/web-app", "/api-client"}
	summaries := map[string]models.RepoSummary{}

	result := filters.SearchRepos(paths, summaries, "api")
	if len(result) != 2 {
		t.Errorf("expected 2 repos matching 'api', got %d", len(result))
	}

	hasAPIService := false
	hasAPIClient := false
	for _, p := range result {
		if p == "/api-service" {
			hasAPIService = true
		}
		if p == "/api-client" {
			hasAPIClient = true
		}
	}
	if !hasAPIService || !hasAPIClient {
		t.Errorf("expected both api repos, got %v", result)
	}
}

func TestSearchReposCaseInsensitive(t *testing.T) {
	t.Parallel()
	paths := []string{"/MyRepo", "/myrepo", "/MYREPO"}
	summaries := map[string]models.RepoSummary{}

	result := filters.SearchRepos(paths, summaries, "myrepo")
	if len(result) != 3 {
		t.Errorf("expected 3 repos with case-insensitive search, got %d", len(result))
	}
}

func TestSearchReposMatchesBranch(t *testing.T) {
	t.Parallel()
	paths := []string{"/repo-a", "/repo-b"}
	summaries := map[string]models.RepoSummary{
		"/repo-a": {Branch: "kyle/dev-1234-fix-thing"},
		"/repo-b": {Branch: "main"},
	}

	result := filters.SearchRepos(paths, summaries, "dev-1234")
	if len(result) != 1 || result[0] != "/repo-a" {
		t.Errorf("expected only the repo on the matching branch, got %v", result)
	}
}

func TestSearchReposScopePrefix(t *testing.T) {
	t.Parallel()
	paths := []string{"/main-service", "/other-repo"}
	summaries := map[string]models.RepoSummary{
		"/main-service": {Branch: "feature/x"},
		"/other-repo":   {Branch: "main"},
	}

	byName := filters.SearchRepos(paths, summaries, "r:main")
	if len(byName) != 1 || byName[0] != "/main-service" {
		t.Errorf("r: scope should match the repo name only, got %v", byName)
	}

	byBranch := filters.SearchRepos(paths, summaries, "b:main")
	if len(byBranch) != 1 || byBranch[0] != "/other-repo" {
		t.Errorf("b: scope should match the branch only, got %v", byBranch)
	}
}

func TestSearchReposPRScope(t *testing.T) {
	t.Parallel()
	paths := []string{"/repo-a", "/repo-b", "/repo-c"}
	summaries := map[string]models.RepoSummary{
		"/repo-a": {PRInfo: &forge.PullRequest{Number: 123, Title: "Bump the deps"}},
		"/repo-b": {PRInfo: &forge.PullRequest{Number: 456, Title: "Fix the thing"}},
		"/repo-c": {},
	}

	byNumber := filters.SearchRepos(paths, summaries, "p:123")
	if len(byNumber) != 1 || byNumber[0] != "/repo-a" {
		t.Errorf("p: with a number should match the PR number exactly, got %v", byNumber)
	}

	byTitle := filters.SearchRepos(paths, summaries, "p:bump")
	if len(byTitle) != 1 || byTitle[0] != "/repo-a" {
		t.Errorf("p: with text should match the PR title, got %v", byTitle)
	}
}

func TestSearchReposTemplateScope(t *testing.T) {
	t.Parallel()
	paths := []string{"/repo-a", "/repo-b"}
	summaries := map[string]models.RepoSummary{
		"/repo-a": {TemplateInfo: &models.CopierTemplateInfo{SrcPath: "gh:kyleking/my_go_template"}},
		"/repo-b": {},
	}

	result := filters.SearchRepos(paths, summaries, "t:my_go_template")
	if len(result) != 1 || result[0] != "/repo-a" {
		t.Errorf("t: should match the copier template source, got %v", result)
	}
}

func TestSearchReposCommitRecencyScope(t *testing.T) {
	t.Parallel()
	paths := []string{"/recent", "/stale", "/unknown"}
	summaries := map[string]models.RepoSummary{
		"/recent":  {LastModified: time.Now().Add(-2 * 24 * time.Hour)},
		"/stale":   {LastModified: time.Now().Add(-60 * 24 * time.Hour)},
		"/unknown": {},
	}

	within := filters.SearchRepos(paths, summaries, "c:<7d")
	if len(within) != 1 || within[0] != "/recent" {
		t.Errorf("c:<7d should match only the recent repo, got %v", within)
	}

	older := filters.SearchRepos(paths, summaries, "c:>30d")
	if len(older) != 1 || older[0] != "/stale" {
		t.Errorf("c:>30d should match only the stale repo, got %v", older)
	}

	bareDefaultsToWithin := filters.SearchRepos(paths, summaries, "c:7d")
	if len(bareDefaultsToWithin) != 1 || bareDefaultsToWithin[0] != "/recent" {
		t.Errorf("bare c:7d should default to within, got %v", bareDefaultsToWithin)
	}

	invalid := filters.SearchRepos(paths, summaries, "c:bogus")
	if len(invalid) != 0 {
		t.Errorf("an invalid duration should match nothing, got %v", invalid)
	}
}

func TestGlobMatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		query    string
		target   string
		expected bool
	}{
		{"bare query found anywhere", "main", "kyle/dev-main-fix", true},
		{"bare query not found", "main", "kyle/dev-trunk-fix", false},
		{"star wildcard spans separators", "dev-*-fix", "kyle/dev-1234-fix", true},
		{"prefix anchor matches start", "^kyle", "kyle/dev-main-fix", true},
		{"prefix anchor rejects non-start", "^dev", "kyle/dev-main-fix", false},
		{"suffix anchor matches end", "fix$", "kyle/dev-main-fix", true},
		{"suffix anchor rejects non-end", "main$", "kyle/dev-main-fix", false},
		{"both anchors require exact match", "^main$", "main", true},
		{"both anchors reject partial", "^main$", "mainline", false},
		{"escaped literal asterisk", `v1\*`, "v1*", true},
		{"escaped asterisk is not a wildcard", `v1\*`, "v1beta", false},
		{"escaped leading caret is literal", `\^weird`, "^weird-name", true},
		{"question mark matches one char", "v?.0", "v2.0", true},
		{"question mark rejects extra char", "v?.0", "v20.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := filters.GlobMatch(tt.query, tt.target); got != tt.expected {
				t.Errorf("GlobMatch(%q, %q) = %v; want %v", tt.query, tt.target, got, tt.expected)
			}
		})
	}
}

func TestSearchReposFuzzy(t *testing.T) {
	t.Parallel()
	paths := []string{"/authentication-service", "/other-app"}
	summaries := map[string]models.RepoSummary{}

	result := filters.SearchRepos(paths, summaries, "auth")
	if len(result) != 1 {
		t.Errorf("expected 1 repo with fuzzy search, got %d", len(result))
	}
}

func TestFuzzyMatchExact(t *testing.T) {
	t.Parallel()
	if !filters.FuzzyMatch("test", "test") {
		t.Error("expected exact match to return true")
	}
}

func TestFuzzyMatchSubstring(t *testing.T) {
	t.Parallel()
	if !filters.FuzzyMatch("api", "api-service") {
		t.Error("expected substring match to return true")
	}
}

func TestFuzzyMatchEmpty(t *testing.T) {
	t.Parallel()
	if !filters.FuzzyMatch("", "anything") {
		t.Error("expected empty pattern to match anything")
	}
}

func TestFuzzyMatchNoMatch(t *testing.T) {
	t.Parallel()
	if filters.FuzzyMatch("xyz123", "abcdef") {
		t.Error("expected no match for unrelated strings")
	}
}
