package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// CreatePR opens a pull request for the repo's current branch, filling title
// and body from its commits, and returns the new pull request's URL. It
// invalidates the repo's cached pull request lists so the next read sees it.
func CreatePR(ctx context.Context, repoPath, branch, base string) (string, error) {
	env := vcs.GetGitHubEnv(repoPath)

	args := []string{"pr", "create", "--fill", "--head", branch}
	if base != "" {
		args = append(args, "--base", base)
	}

	out, err := runGH(ctx, repoPath, env, args...)
	if err != nil {
		return "", err
	}

	InvalidatePRCaches(repoPath)

	return lastLine(string(out)), nil
}

// SquashMergePR squash-merges a pull request and deletes its head branch, both
// on the remote and locally. It invalidates the repo's cached pull request
// lists so the next read reflects the merge.
func SquashMergePR(ctx context.Context, repoPath string, prNumber int) error {
	env := vcs.GetGitHubEnv(repoPath)

	_, err := runGH(ctx, repoPath, env,
		"pr", "merge", strconv.Itoa(prNumber), "--squash", "--delete-branch")
	if err != nil {
		return err
	}

	InvalidatePRCaches(repoPath)

	return nil
}

// CheckoutPR checks the pull request out into repoPath's working directory,
// fetching its head ref first. Returns the local branch name gh landed on.
func CheckoutPR(ctx context.Context, repoPath string, prNumber int) (string, error) {
	env := vcs.GetGitHubEnv(repoPath)

	if _, err := runGH(ctx, repoPath, env, "pr", "checkout", strconv.Itoa(prNumber)); err != nil {
		return "", fmt.Errorf("checking out PR #%d: %w", prNumber, err)
	}

	InvalidatePRCaches(repoPath)

	branch, err := vcs.GetOperations(repoPath).GetCurrentBranch(ctx, repoPath)
	if err != nil {
		return strconv.Itoa(prNumber), nil //nolint:nilerr // the checkout worked; only the name is unknown
	}

	return branch, nil
}

// InvalidatePRCaches drops every cached pull request view of a repo. The
// per-branch and per-repo caches are keyed by upstream as well as path, so
// they are cleared wholesale rather than by key.
func InvalidatePRCaches(repoPath string) {
	cache.PRCache.Clear()
	cache.PRListCache.Clear()
	cache.PRDetailCache.Clear()
	cache.MergedPRHeadsCache.Delete(repoPath)
}

func lastLine(out string) string {
	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) == 0 {
		return ""
	}

	return lines[len(lines)-1]
}
