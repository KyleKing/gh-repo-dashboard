package filters

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestSearchReposEmpty(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{}

	result := SearchRepos(paths, summaries, "")
	if len(result) != 3 {
		t.Errorf("expected 3 repos with empty search, got %d", len(result))
	}
}

func TestSearchReposSubstring(t *testing.T) {
	paths := []string{"/api-service", "/web-app", "/api-client"}
	summaries := map[string]models.RepoSummary{}

	result := SearchRepos(paths, summaries, "api")
	if len(result) != 2 {
		t.Errorf("expected 2 repos matching 'api', got %d", len(result))
	}

	hasApiService := false
	hasApiClient := false
	for _, p := range result {
		if p == "/api-service" {
			hasApiService = true
		}
		if p == "/api-client" {
			hasApiClient = true
		}
	}
	if !hasApiService || !hasApiClient {
		t.Errorf("expected both api repos, got %v", result)
	}
}

func TestSearchReposCaseInsensitive(t *testing.T) {
	paths := []string{"/MyRepo", "/myrepo", "/MYREPO"}
	summaries := map[string]models.RepoSummary{}

	result := SearchRepos(paths, summaries, "myrepo")
	if len(result) != 3 {
		t.Errorf("expected 3 repos with case-insensitive search, got %d", len(result))
	}
}

func TestSearchReposFuzzy(t *testing.T) {
	paths := []string{"/authentication-service", "/other-app"}
	summaries := map[string]models.RepoSummary{}

	result := SearchRepos(paths, summaries, "auth")
	if len(result) != 1 {
		t.Errorf("expected 1 repo with fuzzy search, got %d", len(result))
	}
}

func TestFuzzyMatchExact(t *testing.T) {
	if !FuzzyMatch("test", "test") {
		t.Error("expected exact match to return true")
	}
}

func TestFuzzyMatchSubstring(t *testing.T) {
	if !FuzzyMatch("api", "api-service") {
		t.Error("expected substring match to return true")
	}
}

func TestFuzzyMatchEmpty(t *testing.T) {
	if !FuzzyMatch("", "anything") {
		t.Error("expected empty pattern to match anything")
	}
}

func TestFuzzyMatchNoMatch(t *testing.T) {
	if FuzzyMatch("xyz123", "abcdef") {
		t.Error("expected no match for unrelated strings")
	}
}
