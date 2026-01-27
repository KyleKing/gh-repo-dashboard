package filters

import (
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestSortPathsByName(t *testing.T) {
	paths := []string{"/charlie", "/alice", "/bob"}
	summaries := map[string]models.RepoSummary{
		"/alice":   {Path: "/alice"},
		"/bob":     {Path: "/bob"},
		"/charlie": {Path: "/charlie"},
	}

	result := SortPaths(paths, summaries, models.SortModeName, false)

	expected := []string{"/alice", "/bob", "/charlie"}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsByNameReverse(t *testing.T) {
	paths := []string{"/charlie", "/alice", "/bob"}
	summaries := map[string]models.RepoSummary{
		"/alice":   {Path: "/alice"},
		"/bob":     {Path: "/bob"},
		"/charlie": {Path: "/charlie"},
	}

	result := SortPaths(paths, summaries, models.SortModeName, true)

	expected := []string{"/charlie", "/bob", "/alice"}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsByModified(t *testing.T) {
	now := time.Now()
	paths := []string{"/old", "/new", "/middle"}
	summaries := map[string]models.RepoSummary{
		"/old":    {Path: "/old", LastModified: now.Add(-24 * time.Hour)},
		"/new":    {Path: "/new", LastModified: now},
		"/middle": {Path: "/middle", LastModified: now.Add(-12 * time.Hour)},
	}

	result := SortPaths(paths, summaries, models.SortModeModified, false)

	expected := []string{"/new", "/middle", "/old"}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsByStatus(t *testing.T) {
	paths := []string{"/clean", "/dirty1", "/dirty2"}
	summaries := map[string]models.RepoSummary{
		"/clean":  {Path: "/clean"},
		"/dirty1": {Path: "/dirty1", Unstaged: 3},
		"/dirty2": {Path: "/dirty2", Unstaged: 1},
	}

	result := SortPaths(paths, summaries, models.SortModeStatus, false)

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
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Branch: "main"},
		"/repo2": {Path: "/repo2", Branch: "develop"},
		"/repo3": {Path: "/repo3", Branch: "feature"},
	}

	result := SortPaths(paths, summaries, models.SortModeBranch, false)

	expected := []string{"/repo2", "/repo3", "/repo1"}
	for i, p := range result {
		if p != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], p)
		}
	}
}

func TestSortPathsEmpty(t *testing.T) {
	var paths []string
	summaries := map[string]models.RepoSummary{}

	result := SortPaths(paths, summaries, models.SortModeName, false)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}
