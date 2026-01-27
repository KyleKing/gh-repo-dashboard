package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func DiscoverRepos(basePaths []string, maxDepth int) []string {
	var repos []string
	seen := make(map[string]bool)

	for _, basePath := range basePaths {
		discovered := discoverInPath(basePath, maxDepth)
		for _, repo := range discovered {
			if !seen[repo] {
				seen[repo] = true
				repos = append(repos, repo)
			}
		}
	}

	return repos
}

func discoverInPath(basePath string, maxDepth int) []string {
	var repos []string

	if vcs.IsRepo(basePath) {
		return []string{basePath}
	}

	scanDir(basePath, 0, maxDepth, &repos)
	return repos
}

func scanDir(dir string, depth int, maxDepth int, repos *[]string) {
	if depth >= maxDepth {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(dir, name)

		if vcs.IsRepo(fullPath) {
			*repos = append(*repos, fullPath)
			continue
		}

		scanDir(fullPath, depth+1, maxDepth, repos)
	}
}
