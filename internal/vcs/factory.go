package vcs

import (
	"os"
	"path/filepath"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func DetectVCSType(repoPath string) models.VCSType {
	if _, err := os.Stat(filepath.Join(repoPath, ".jj")); err == nil {
		return models.VCSTypeJJ
	}
	return models.VCSTypeGit
}

func GetOperations(repoPath string) Operations {
	vcsType := DetectVCSType(repoPath)
	switch vcsType {
	case models.VCSTypeJJ:
		return NewJJOperations()
	default:
		return NewGitOperations()
	}
}

func GetGitHubEnv(repoPath string) []string {
	vcsType := DetectVCSType(repoPath)
	if vcsType == models.VCSTypeJJ {
		colocatedGit := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(colocatedGit); err == nil {
			return nil
		}
		jjGit := filepath.Join(repoPath, ".jj", "repo", "store", "git")
		return []string{"GIT_DIR=" + jjGit}
	}
	return nil
}

func IsRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}

	jjDir := filepath.Join(path, ".jj")
	if _, err := os.Stat(jjDir); err == nil {
		return true
	}

	return false
}
