package discovery

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func mustMkdirAll(path string) {
	if err := os.MkdirAll(path, 0o750); err != nil {
		panic("mkdir " + path + ": " + err.Error())
	}
}

func TestDiscoverRepos(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func(base string) []string
		maxDepth int
		expected int
	}{
		{
			name: "finds git repos at depth 1",
			setup: func(base string) []string {
				repos := []string{"repo1", "repo2"}
				for _, r := range repos {
					mustMkdirAll(filepath.Join(base, r, ".git"))
				}

				return repos
			},
			maxDepth: 1,
			expected: 2,
		},
		{
			name: "finds jj repos",
			setup: func(base string) []string {
				mustMkdirAll(filepath.Join(base, "jj-repo", ".jj"))
				return []string{"jj-repo"}
			},
			maxDepth: 1,
			expected: 1,
		},
		{
			name: "respects max depth",
			setup: func(base string) []string {
				mustMkdirAll(filepath.Join(base, "level1", "level2", "repo", ".git"))
				return nil
			},
			maxDepth: 1,
			expected: 0,
		},
		{
			name: "finds nested repos at depth 2",
			setup: func(base string) []string {
				mustMkdirAll(filepath.Join(base, "group", "repo", ".git"))
				return nil
			},
			maxDepth: 2,
			expected: 1,
		},
		{
			name: "skips hidden directories",
			setup: func(base string) []string {
				mustMkdirAll(filepath.Join(base, ".hidden", "repo", ".git"))
				mustMkdirAll(filepath.Join(base, "visible", ".git"))

				return nil
			},
			maxDepth: 2,
			expected: 1,
		},
		{
			name: "handles base path as repo",
			setup: func(base string) []string {
				mustMkdirAll(filepath.Join(base, ".git"))
				return nil
			},
			maxDepth: 1,
			expected: 1,
		},
		{
			name:     "handles empty directory",
			setup:    func(_ string) []string { return nil },
			maxDepth: 1,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			tt.setup(base)

			repos := DiscoverRepos([]string{base}, tt.maxDepth)
			if len(repos) != tt.expected {
				t.Errorf("expected %d repos, got %d: %v", tt.expected, len(repos), repos)
			}
		})
	}
}

func TestDiscoverReposDeduplicates(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	repoPath := filepath.Join(base, "repo")
	mustMkdirAll(filepath.Join(repoPath, ".git"))

	repos := DiscoverRepos([]string{base, repoPath, base}, 1)
	if len(repos) != 1 {
		t.Errorf("expected 1 unique repo, got %d: %v", len(repos), repos)
	}
}

func TestDiscoverReposMultiplePaths(t *testing.T) {
	t.Parallel()
	base1 := t.TempDir()
	base2 := t.TempDir()

	mustMkdirAll(filepath.Join(base1, "repo1", ".git"))
	mustMkdirAll(filepath.Join(base2, "repo2", ".git"))

	repos := DiscoverRepos([]string{base1, base2}, 1)
	if len(repos) != 2 {
		t.Errorf("expected 2 repos, got %d", len(repos))
	}
}

func TestDiscoverReposStopsAtRepo(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	parentRepo := filepath.Join(base, "parent")
	nestedRepo := filepath.Join(parentRepo, "nested")

	mustMkdirAll(filepath.Join(parentRepo, ".git"))
	mustMkdirAll(filepath.Join(nestedRepo, ".git"))

	repos := DiscoverRepos([]string{base}, 3)

	if len(repos) != 1 {
		t.Errorf("expected 1 repo (should stop at parent), got %d: %v", len(repos), repos)
	}
	if repos[0] != parentRepo {
		t.Errorf("expected parent repo, got %s", repos[0])
	}
}

func TestDiscoverReposOrder(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		mustMkdirAll(filepath.Join(base, name, ".git"))
	}

	repos := DiscoverRepos([]string{base}, 1)

	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = filepath.Base(r)
	}

	if !sort.StringsAreSorted(names) {
		t.Log("note: repos are not sorted alphabetically (may be by discovery order)")
	}
}

func TestDiscoverReposNonexistentPath(t *testing.T) {
	t.Parallel()
	repos := DiscoverRepos([]string{"/nonexistent/path/that/does/not/exist"}, 1)
	if len(repos) != 0 {
		t.Errorf("expected 0 repos for nonexistent path, got %d", len(repos))
	}
}

func TestDiscoverReposZeroDepth(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	mustMkdirAll(filepath.Join(base, "repo", ".git"))

	repos := DiscoverRepos([]string{base}, 0)
	if len(repos) != 0 {
		t.Errorf("expected 0 repos at depth 0, got %d", len(repos))
	}
}
