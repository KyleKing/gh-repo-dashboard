package vcs

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type GitOperations struct{}

func NewGitOperations() *GitOperations {
	return &GitOperations{}
}

func (g *GitOperations) VCSType() models.VCSType {
	return models.VCSTypeGit
}

func (g *GitOperations) runGit(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *GitOperations) GetRepoSummary(ctx context.Context, repoPath string) (models.RepoSummary, error) {
	summary := models.RepoSummary{
		Path:    repoPath,
		VCSType: models.VCSTypeGit,
	}

	branch, err := g.GetCurrentBranch(ctx, repoPath)
	if err != nil {
		return summary, err
	}
	summary.Branch = branch

	upstream, _ := g.GetUpstream(ctx, repoPath, branch)
	summary.Upstream = upstream

	if upstream != "" {
		ahead, behind, _ := g.GetAheadBehind(ctx, repoPath, branch, upstream)
		summary.Ahead = ahead
		summary.Behind = behind
	}

	staged, unstaged, untracked, conflicted := g.getStatusCounts(ctx, repoPath)
	summary.Staged = staged
	summary.Unstaged = unstaged
	summary.Untracked = untracked
	summary.Conflicted = conflicted

	stashCount, _ := g.getStashCount(ctx, repoPath)
	summary.StashCount = stashCount

	lastMod, _ := g.GetLastModified(ctx, repoPath)
	if lastMod > 0 {
		summary.LastModified = time.Unix(lastMod, 0)
	}

	return summary, nil
}

func (g *GitOperations) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	out, err := g.runGit(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if out == "HEAD" {
		hash, err := g.runGit(ctx, repoPath, "rev-parse", "--short", "HEAD")
		if err != nil {
			return "HEAD", nil
		}
		return fmt.Sprintf("(%s)", hash), nil
	}
	return out, nil
}

func (g *GitOperations) GetUpstream(ctx context.Context, repoPath string, branch string) (string, error) {
	out, err := g.runGit(ctx, repoPath, "rev-parse", "--abbrev-ref", fmt.Sprintf("%s@{upstream}", branch))
	if err != nil {
		return "", err
	}
	return out, nil
}

func (g *GitOperations) GetAheadBehind(ctx context.Context, repoPath string, branch string, upstream string) (int, int, error) {
	out, err := g.runGit(ctx, repoPath, "rev-list", "--left-right", "--count", fmt.Sprintf("%s...%s", branch, upstream))
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %s", out)
	}

	ahead, _ := strconv.Atoi(parts[0])
	behind, _ := strconv.Atoi(parts[1])
	return ahead, behind, nil
}

func (g *GitOperations) getStatusCounts(ctx context.Context, repoPath string) (staged, unstaged, untracked, conflicted int) {
	out, err := g.runGit(ctx, repoPath, "status", "--porcelain", "-z")
	if err != nil {
		return
	}

	entries := strings.Split(out, "\x00")
	for _, entry := range entries {
		if len(entry) < 2 {
			continue
		}
		x := entry[0]
		y := entry[1]

		switch {
		case x == 'U' || y == 'U' || (x == 'D' && y == 'D') || (x == 'A' && y == 'A'):
			conflicted++
		case x == '?':
			untracked++
		default:
			if x != ' ' && x != '?' {
				staged++
			}
			if y != ' ' && y != '?' {
				unstaged++
			}
		}
	}
	return
}

func (g *GitOperations) GetStagedCount(ctx context.Context, repoPath string) (int, error) {
	staged, _, _, _ := g.getStatusCounts(ctx, repoPath)
	return staged, nil
}

func (g *GitOperations) GetUnstagedCount(ctx context.Context, repoPath string) (int, error) {
	_, unstaged, _, _ := g.getStatusCounts(ctx, repoPath)
	return unstaged, nil
}

func (g *GitOperations) GetUntrackedCount(ctx context.Context, repoPath string) (int, error) {
	_, _, untracked, _ := g.getStatusCounts(ctx, repoPath)
	return untracked, nil
}

func (g *GitOperations) GetConflictedCount(ctx context.Context, repoPath string) (int, error) {
	_, _, _, conflicted := g.getStatusCounts(ctx, repoPath)
	return conflicted, nil
}

func (g *GitOperations) getStashCount(ctx context.Context, repoPath string) (int, error) {
	out, err := g.runGit(ctx, repoPath, "stash", "list")
	if err != nil {
		return 0, err
	}
	if out == "" {
		return 0, nil
	}
	return len(strings.Split(out, "\n")), nil
}

func (g *GitOperations) GetBranchList(ctx context.Context, repoPath string) ([]models.BranchInfo, error) {
	format := "%(refname:short)\t%(upstream:short)\t%(upstream:track)\t%(committerdate:unix)\t%(HEAD)"
	out, err := g.runGit(ctx, repoPath, "for-each-ref", "--format="+format, "refs/heads/")
	if err != nil {
		return nil, err
	}

	var branches []models.BranchInfo
	scanner := bufio.NewScanner(strings.NewReader(out))
	trackRe := regexp.MustCompile(`\[ahead (\d+)(?:, behind (\d+))?\]|\[behind (\d+)\]`)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}

		var ahead, behind int
		if matches := trackRe.FindStringSubmatch(parts[2]); matches != nil {
			if matches[1] != "" {
				ahead, _ = strconv.Atoi(matches[1])
			}
			if matches[2] != "" {
				behind, _ = strconv.Atoi(matches[2])
			}
			if matches[3] != "" {
				behind, _ = strconv.Atoi(matches[3])
			}
		}

		ts, _ := strconv.ParseInt(parts[3], 10, 64)

		branches = append(branches, models.BranchInfo{
			Name:       parts[0],
			Upstream:   parts[1],
			Ahead:      ahead,
			Behind:     behind,
			LastCommit: time.Unix(ts, 0),
			IsCurrent:  parts[4] == "*",
		})
	}

	return branches, nil
}

func (g *GitOperations) GetStashList(ctx context.Context, repoPath string) ([]models.StashDetail, error) {
	format := "%(reflog:short)\t%(reflog:subject)\t%(committerdate:unix)"
	out, err := g.runGit(ctx, repoPath, "stash", "list", "--format="+format)
	if err != nil {
		return nil, err
	}

	if out == "" {
		return nil, nil
	}

	var stashes []models.StashDetail
	scanner := bufio.NewScanner(strings.NewReader(out))
	stashRe := regexp.MustCompile(`stash@\{(\d+)\}`)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		var index int
		if matches := stashRe.FindStringSubmatch(parts[0]); matches != nil {
			index, _ = strconv.Atoi(matches[1])
		}

		ts, _ := strconv.ParseInt(parts[2], 10, 64)

		stashes = append(stashes, models.StashDetail{
			Index:   index,
			Message: parts[1],
			Date:    time.Unix(ts, 0),
		})
	}

	return stashes, nil
}

func (g *GitOperations) GetWorktreeList(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error) {
	out, err := g.runGit(ctx, repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var worktrees []models.WorktreeInfo
	var current models.WorktreeInfo

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = models.WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "bare":
			current.IsBare = true
		case line == "locked":
			current.IsLocked = true
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

func (g *GitOperations) GetCommitLog(ctx context.Context, repoPath string, count int) ([]models.CommitInfo, error) {
	format := "%H\t%h\t%s\t%an\t%ct"
	out, err := g.runGit(ctx, repoPath, "log", fmt.Sprintf("-n%d", count), "--format="+format)
	if err != nil {
		return nil, err
	}

	var commits []models.CommitInfo
	scanner := bufio.NewScanner(strings.NewReader(out))

	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 5 {
			continue
		}

		ts, _ := strconv.ParseInt(parts[4], 10, 64)

		commits = append(commits, models.CommitInfo{
			Hash:      parts[0],
			ShortHash: parts[1],
			Subject:   parts[2],
			Author:    parts[3],
			Date:      time.Unix(ts, 0),
		})
	}

	return commits, nil
}

func (g *GitOperations) GetLastModified(ctx context.Context, repoPath string) (int64, error) {
	out, err := g.runGit(ctx, repoPath, "log", "-1", "--format=%ct")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(out, 10, 64)
}

func (g *GitOperations) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	out, err := g.runGit(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return out, nil
}

func (g *GitOperations) FetchAll(ctx context.Context, repoPath string) (bool, string, error) {
	_, err := g.runGit(ctx, repoPath, "fetch", "--all", "--prune")
	if err != nil {
		return false, err.Error(), nil
	}
	return true, "Fetched from all remotes", nil
}

func (g *GitOperations) PruneRemote(ctx context.Context, repoPath string) (bool, string, error) {
	_, err := g.runGit(ctx, repoPath, "remote", "prune", "origin")
	if err != nil {
		return false, err.Error(), nil
	}
	return true, "Pruned stale remote branches", nil
}

func (g *GitOperations) CleanupMergedBranches(ctx context.Context, repoPath string) (bool, string, error) {
	mainBranch := "main"
	if _, err := g.runGit(ctx, repoPath, "rev-parse", "--verify", "main"); err != nil {
		if _, err := g.runGit(ctx, repoPath, "rev-parse", "--verify", "master"); err == nil {
			mainBranch = "master"
		} else {
			return false, "Could not find main or master branch", nil
		}
	}

	out, err := g.runGit(ctx, repoPath, "branch", "--merged", mainBranch)
	if err != nil {
		return false, err.Error(), nil
	}

	var deleted []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		branch := strings.TrimSpace(scanner.Text())
		branch = strings.TrimPrefix(branch, "* ")

		if branch == mainBranch || branch == "master" || branch == "main" || branch == "" {
			continue
		}

		if _, err := g.runGit(ctx, repoPath, "branch", "-d", branch); err == nil {
			deleted = append(deleted, branch)
		}
	}

	if len(deleted) == 0 {
		return true, "No merged branches to delete", nil
	}
	return true, fmt.Sprintf("Deleted %d branches: %s", len(deleted), strings.Join(deleted, ", ")), nil
}

func ExtractRepoPath(remoteURL string) string {
	url := strings.TrimSuffix(remoteURL, ".git")

	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
	} else if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
	} else if strings.HasPrefix(url, "http://") {
		url = strings.TrimPrefix(url, "http://")
	}

	parts := strings.Split(url, "/")
	if len(parts) >= 3 {
		return filepath.Join(parts[len(parts)-2], parts[len(parts)-1])
	}
	return ""
}
