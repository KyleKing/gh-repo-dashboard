package vcs

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type JJOperations struct{}

func NewJJOperations() *JJOperations {
	return &JJOperations{}
}

func (j *JJOperations) VCSType() models.VCSType {
	return models.VCSTypeJJ
}

func (j *JJOperations) runJJ(ctx context.Context, repoPath string, args ...string) (string, error) {
	fullArgs := append([]string{"-R", repoPath}, args...)
	cmd := exec.CommandContext(ctx, "jj", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("jj %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (j *JJOperations) GetRepoSummary(ctx context.Context, repoPath string) (models.RepoSummary, error) {
	summary := models.RepoSummary{
		Path:    repoPath,
		VCSType: models.VCSTypeJJ,
	}

	bookmark, err := j.GetCurrentBranch(ctx, repoPath)
	if err != nil {
		summary.Branch = "@"
	} else {
		summary.Branch = bookmark
	}

	if bookmark != "@" && bookmark != "" {
		upstream, _ := j.GetUpstream(ctx, repoPath, bookmark)
		summary.Upstream = upstream

		if upstream != "" {
			ahead, behind, _ := j.GetAheadBehind(ctx, repoPath, bookmark, upstream)
			summary.Ahead = ahead
			summary.Behind = behind
		}
	}

	_, unstaged, _, _ := j.getStatusCounts(ctx, repoPath)
	summary.Unstaged = unstaged

	lastMod, _ := j.GetLastModified(ctx, repoPath)
	if lastMod > 0 {
		summary.LastModified = time.Unix(lastMod, 0)
	}

	return summary, nil
}

func (j *JJOperations) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	out, err := j.runJJ(ctx, repoPath, "log", "-r", "@", "-T", "bookmarks", "--no-graph")
	if err != nil {
		return "@", nil
	}
	bookmarks := strings.TrimSpace(out)
	if bookmarks != "" {
		parts := strings.Fields(bookmarks)
		if len(parts) > 0 {
			return parts[0], nil
		}
	}
	return "@", nil
}

func (j *JJOperations) GetUpstream(ctx context.Context, repoPath string, branch string) (string, error) {
	if branch == "@" || branch == "" {
		return "", nil
	}

	out, err := j.runJJ(ctx, repoPath, "bookmark", "list")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), branch) {
			if strings.Contains(line, "@origin") {
				return fmt.Sprintf("%s@origin", branch), nil
			}
		}
	}
	return "", nil
}

func (j *JJOperations) GetAheadBehind(ctx context.Context, repoPath string, branch string, upstream string) (int, int, error) {
	if branch == "@" || branch == "" {
		return 0, 0, nil
	}

	aheadOut, err := j.runJJ(ctx, repoPath, "log", "-r", fmt.Sprintf("%s@origin..", branch), "-T", "change_id", "--no-graph")
	if err != nil {
		return 0, 0, nil
	}
	ahead := countNonEmptyLines(aheadOut)

	behindOut, err := j.runJJ(ctx, repoPath, "log", "-r", fmt.Sprintf("..%s@origin", branch), "-T", "change_id", "--no-graph")
	if err != nil {
		return ahead, 0, nil
	}
	behind := countNonEmptyLines(behindOut)

	return ahead, behind, nil
}

func countNonEmptyLines(s string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func (j *JJOperations) getStatusCounts(ctx context.Context, repoPath string) (staged, unstaged, untracked, conflicted int) {
	out, err := j.runJJ(ctx, repoPath, "status")
	if err != nil {
		return
	}

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "A ") || strings.HasPrefix(trimmed, "M ") ||
			strings.HasPrefix(trimmed, "D ") || strings.HasPrefix(trimmed, "R ") {
			unstaged++
		}
	}
	return 0, unstaged, 0, 0
}

func (j *JJOperations) GetStagedCount(ctx context.Context, repoPath string) (int, error) {
	return 0, nil
}

func (j *JJOperations) GetUnstagedCount(ctx context.Context, repoPath string) (int, error) {
	_, unstaged, _, _ := j.getStatusCounts(ctx, repoPath)
	return unstaged, nil
}

func (j *JJOperations) GetUntrackedCount(ctx context.Context, repoPath string) (int, error) {
	return 0, nil
}

func (j *JJOperations) GetConflictedCount(ctx context.Context, repoPath string) (int, error) {
	return 0, nil
}

func (j *JJOperations) GetBranchList(ctx context.Context, repoPath string) ([]models.BranchInfo, error) {
	out, err := j.runJJ(ctx, repoPath, "bookmark", "list")
	if err != nil {
		return nil, err
	}

	currentBookmark, _ := j.GetCurrentBranch(ctx, repoPath)

	var branches []models.BranchInfo
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 1 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		hasTracking := strings.Contains(line, "@origin")

		var upstream string
		var ahead, behind int
		if hasTracking {
			upstream = fmt.Sprintf("%s@origin", name)
			ahead, behind, _ = j.GetAheadBehind(ctx, repoPath, name, upstream)
		}

		branches = append(branches, models.BranchInfo{
			Name:      name,
			Upstream:  upstream,
			Ahead:     ahead,
			Behind:    behind,
			IsCurrent: name == currentBookmark,
		})
	}

	return branches, nil
}

func (j *JJOperations) GetStashList(ctx context.Context, repoPath string) ([]models.StashDetail, error) {
	return nil, nil
}

func (j *JJOperations) GetWorktreeList(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error) {
	out, err := j.runJJ(ctx, repoPath, "workspace", "list")
	if err != nil {
		return nil, err
	}

	workspaceRe := regexp.MustCompile(`^(\S+)@(\S+):\s+(\S+)`)

	var worktrees []models.WorktreeInfo
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		matches := workspaceRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		workspaceName := matches[1]
		path := matches[3]

		worktrees = append(worktrees, models.WorktreeInfo{
			Path:   path,
			Branch: workspaceName,
		})
	}

	return worktrees, nil
}

func (j *JJOperations) GetCommitLog(ctx context.Context, repoPath string, count int) ([]models.CommitInfo, error) {
	format := `change_id.short() ++ "\t" ++ description.first_line() ++ "\t" ++ author.name() ++ "\t" ++ committer.timestamp().utc().format("%s")`
	out, err := j.runJJ(ctx, repoPath, "log", "-r", fmt.Sprintf("@~%d..", count), "-T", format, "--no-graph")
	if err != nil {
		return nil, err
	}

	var commits []models.CommitInfo
	scanner := bufio.NewScanner(strings.NewReader(out))

	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 4 {
			continue
		}

		ts, _ := strconv.ParseInt(parts[3], 10, 64)

		commits = append(commits, models.CommitInfo{
			Hash:      parts[0],
			ShortHash: parts[0],
			Subject:   parts[1],
			Author:    parts[2],
			Date:      time.Unix(ts, 0),
		})
	}

	return commits, nil
}

func (j *JJOperations) GetLastModified(ctx context.Context, repoPath string) (int64, error) {
	format := `committer.timestamp().utc().format("%s")`
	out, err := j.runJJ(ctx, repoPath, "log", "-r", "@", "-T", format, "--no-graph")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}

func (j *JJOperations) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	out, err := j.runJJ(ctx, repoPath, "git", "remote", "list")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "origin") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", nil
}

func (j *JJOperations) FetchAll(ctx context.Context, repoPath string) (bool, string, error) {
	_, err := j.runJJ(ctx, repoPath, "git", "fetch", "--all-remotes")
	if err != nil {
		return false, err.Error(), nil
	}
	return true, "Fetched from all remotes", nil
}

func (j *JJOperations) PruneRemote(ctx context.Context, repoPath string) (bool, string, error) {
	return true, "JJ doesn't require explicit pruning", nil
}

func (j *JJOperations) CleanupMergedBranches(ctx context.Context, repoPath string) (bool, string, error) {
	out, err := j.runJJ(ctx, repoPath, "bookmark", "list")
	if err != nil {
		return false, err.Error(), nil
	}

	var deleted []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 1 {
			continue
		}

		bookmark := strings.TrimSpace(parts[0])
		if bookmark == "main" || bookmark == "master" || bookmark == "trunk" {
			continue
		}

		isMerged, err := j.runJJ(ctx, repoPath, "log", "-r",
			fmt.Sprintf("%s@origin..main@origin", bookmark), "-T", "change_id", "--no-graph")
		if err != nil {
			continue
		}

		if strings.TrimSpace(isMerged) == "" {
			if _, err := j.runJJ(ctx, repoPath, "bookmark", "delete", bookmark); err == nil {
				deleted = append(deleted, bookmark)
			}
		}
	}

	if len(deleted) == 0 {
		return true, "No merged bookmarks to delete", nil
	}
	return true, fmt.Sprintf("Deleted %d bookmarks: %s", len(deleted), strings.Join(deleted, ", ")), nil
}
