package vcs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type JJOperations struct{}

func NewJJOperations() *JJOperations {
	return &JJOperations{}
}

// jjCurrentBookmarkFormat renders only local bookmarks, avoiding the "*" sync
// marker that the built-in "bookmarks" keyword appends for unsynced remotes.
const jjCurrentBookmarkFormat = `self.local_bookmarks().map(|b| b.name()).join(" ")`

// jjBookmarkListFormat emits one line per local bookmark and, when tracked,
// one additional line with its origin ahead/behind counts. Real `jj bookmark
// list` output puts remote-tracking info on an indented "@origin: ..."
// continuation line rather than inline with the bookmark name, so this
// template sidesteps that text format entirely.
const jjBookmarkListFormat = `if(self.remote() == "origin", ` +
	`self.name() ++ "\torigin\t" ++ self.tracking_ahead_count().lower() ++ "\t" ++ self.tracking_behind_count().lower() ++ "\n", ` +
	`if(self.remote(), "", self.name() ++ "\tlocal\n"))`

// jjWorkspaceListFormat emits "name\tabsolute-path" per workspace. The default
// `jj workspace list` output has no path at all, so a template is required.
const jjWorkspaceListFormat = `self.name() ++ "\t" ++ self.root() ++ "\n"`

type jjBookmark struct {
	name     string
	upstream string
	ahead    int
	behind   int
}

func parseJJBookmarkList(out string) []jjBookmark {
	byName := make(map[string]*jjBookmark)
	var order []string

	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		name := fields[0]

		bookmark, ok := byName[name]
		if !ok {
			bookmark = &jjBookmark{name: name}
			byName[name] = bookmark
			order = append(order, name)
		}

		if len(fields) == 4 && fields[1] == "origin" {
			bookmark.upstream = name + "@origin"
			bookmark.ahead, _ = strconv.Atoi(fields[2])
			bookmark.behind, _ = strconv.Atoi(fields[3])
		}
	}

	bookmarks := make([]jjBookmark, 0, len(order))
	for _, name := range order {
		bookmarks = append(bookmarks, *byName[name])
	}

	return bookmarks
}

func (j *JJOperations) VCSType() models.VCSType {
	return models.VCSTypeJJ
}

func (j *JJOperations) runJJ(ctx context.Context, repoPath string, args ...string) (string, error) {
	fullArgs := append([]string{"-R", repoPath}, args...)
	out, err := runCommand(ctx, "", "jj", fullArgs...)
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("jj %s: %s", strings.Join(args, " "), string(exitErr.Stderr))
		}

		return "", err
	}

	return out, nil
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
	out, err := j.runJJ(ctx, repoPath, "log", "-r", "@", "-T", jjCurrentBookmarkFormat, "--no-graph")
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

func (j *JJOperations) GetUpstream(ctx context.Context, repoPath, branch string) (string, error) {
	if branch == "@" || branch == "" {
		return "", nil
	}

	out, err := j.runJJ(ctx, repoPath, "bookmark", "list", "--all-remotes", "-T", jjBookmarkListFormat)
	if err != nil {
		return "", err
	}

	for _, bookmark := range parseJJBookmarkList(out) {
		if bookmark.name == branch {
			return bookmark.upstream, nil
		}
	}

	return "", nil
}

func (j *JJOperations) GetAheadBehind(ctx context.Context, repoPath, branch, upstream string) (int, int, error) {
	if branch == "@" || branch == "" || upstream == "" {
		return 0, 0, nil
	}

	out, err := j.runJJ(ctx, repoPath, "bookmark", "list", "--all-remotes", "-T", jjBookmarkListFormat)
	if err != nil {
		return 0, 0, nil
	}

	for _, bookmark := range parseJJBookmarkList(out) {
		if bookmark.name == branch && bookmark.upstream == upstream {
			return bookmark.ahead, bookmark.behind, nil
		}
	}

	return 0, 0, nil
}

func (j *JJOperations) getStatusCounts(ctx context.Context, repoPath string) (staged, unstaged, untracked, conflicted int) {
	out, err := j.runJJ(ctx, repoPath, "status")
	if err != nil {
		return staged, unstaged, untracked, conflicted
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
	out, err := j.runJJ(ctx, repoPath, "bookmark", "list", "--all-remotes", "-T", jjBookmarkListFormat)
	if err != nil {
		return nil, err
	}

	currentBookmark, _ := j.GetCurrentBranch(ctx, repoPath)

	var branches []models.BranchInfo
	for _, bookmark := range parseJJBookmarkList(out) {
		branches = append(branches, models.BranchInfo{
			Name:      bookmark.name,
			Upstream:  bookmark.upstream,
			Ahead:     bookmark.ahead,
			Behind:    bookmark.behind,
			IsCurrent: bookmark.name == currentBookmark,
		})
	}

	return branches, nil
}

func (j *JJOperations) GetStashList(ctx context.Context, repoPath string) ([]models.StashDetail, error) {
	return nil, nil
}

func (j *JJOperations) GetWorktreeList(ctx context.Context, repoPath string) ([]models.WorktreeInfo, error) {
	out, err := j.runJJ(ctx, repoPath, "workspace", "list", "-T", jjWorkspaceListFormat)
	if err != nil {
		return nil, err
	}

	var worktrees []models.WorktreeInfo
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		name, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}

		worktrees = append(worktrees, models.WorktreeInfo{
			Path:   path,
			Branch: name,
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
	out, err := j.runJJ(ctx, repoPath, "bookmark", "list", "--all-remotes", "-T", jjBookmarkListFormat)
	if err != nil {
		return false, err.Error(), nil
	}

	var deleted []string
	for _, bookmark := range parseJJBookmarkList(out) {
		if bookmark.name == "main" || bookmark.name == "master" || bookmark.name == "trunk" {
			continue
		}

		isMerged, err := j.runJJ(ctx, repoPath, "log", "-r",
			bookmark.name+"@origin..main@origin", "-T", "change_id", "--no-graph")
		if err != nil {
			continue
		}

		if strings.TrimSpace(isMerged) == "" {
			if _, err := j.runJJ(ctx, repoPath, "bookmark", "delete", bookmark.name); err == nil {
				deleted = append(deleted, bookmark.name)
			}
		}
	}

	if len(deleted) == 0 {
		return true, "No merged bookmarks to delete", nil
	}

	return true, fmt.Sprintf("Deleted %d bookmarks: %s", len(deleted), strings.Join(deleted, ", ")), nil
}
