package filters_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestSortPathsByName(t *testing.T) {
	t.Parallel()
	paths := []string{"/charlie", "/alice", "/bob"}
	summaries := map[string]models.RepoSummary{
		"/alice":   {RepoSummary: vcs.RepoSummary{Path: "/alice"}},
		"/bob":     {RepoSummary: vcs.RepoSummary{Path: "/bob"}},
		"/charlie": {RepoSummary: vcs.RepoSummary{Path: "/charlie"}},
	}

	result := filters.SortPaths(paths, summaries, models.SortModeName, false)

	expected := []string{"/alice", "/bob", "/charlie"}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsByNameReverse(t *testing.T) {
	t.Parallel()
	paths := []string{"/charlie", "/alice", "/bob"}
	summaries := map[string]models.RepoSummary{
		"/alice":   {RepoSummary: vcs.RepoSummary{Path: "/alice"}},
		"/bob":     {RepoSummary: vcs.RepoSummary{Path: "/bob"}},
		"/charlie": {RepoSummary: vcs.RepoSummary{Path: "/charlie"}},
	}

	result := filters.SortPaths(paths, summaries, models.SortModeName, true)

	expected := []string{"/charlie", "/bob", "/alice"}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsByModified(t *testing.T) {
	t.Parallel()
	now := time.Now()
	paths := []string{"/old", "/new", "/middle"}
	summaries := map[string]models.RepoSummary{
		"/old":    {RepoSummary: vcs.RepoSummary{Path: "/old", LastModified: now.Add(-24 * time.Hour)}},
		"/new":    {RepoSummary: vcs.RepoSummary{Path: "/new", LastModified: now}},
		"/middle": {RepoSummary: vcs.RepoSummary{Path: "/middle", LastModified: now.Add(-12 * time.Hour)}},
	}

	result := filters.SortPaths(paths, summaries, models.SortModeModified, false)

	expected := []string{"/new", "/middle", "/old"}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsByStatus(t *testing.T) {
	t.Parallel()
	paths := []string{"/clean", "/dirty1", "/dirty2"}
	summaries := map[string]models.RepoSummary{
		"/clean":  {RepoSummary: vcs.RepoSummary{Path: "/clean"}},
		"/dirty1": {RepoSummary: vcs.RepoSummary{Path: "/dirty1", Unstaged: 3}},
		"/dirty2": {RepoSummary: vcs.RepoSummary{Path: "/dirty2", Unstaged: 1}},
	}

	result := filters.SortPaths(paths, summaries, models.SortModeStatus, false)

	if result[0] != "/dirty1" {
		t.Errorf("expected /dirty1 first (most dirty), got %s", result[0])
	}
	if result[1] != "/dirty2" {
		t.Errorf("expected /dirty2 second, got %s", result[1])
	}
	if result[2] != "/clean" {
		t.Errorf("expected /clean last, got %s", result[2])
	}
}

func TestSortPathsByBranch(t *testing.T) {
	t.Parallel()
	paths := []string{testRepo1Path, "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		testRepo1Path: {RepoSummary: vcs.RepoSummary{Path: testRepo1Path, Branch: "main"}},
		"/repo2":      {RepoSummary: vcs.RepoSummary{Path: "/repo2", Branch: "develop"}},
		"/repo3":      {RepoSummary: vcs.RepoSummary{Path: "/repo3", Branch: "feature"}},
	}

	result := filters.SortPaths(paths, summaries, models.SortModeBranch, false)

	expected := []string{"/repo2", "/repo3", testRepo1Path}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsEmpty(t *testing.T) {
	t.Parallel()
	var paths []string
	summaries := map[string]models.RepoSummary{}

	result := filters.SortPaths(paths, summaries, models.SortModeName, false)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}
