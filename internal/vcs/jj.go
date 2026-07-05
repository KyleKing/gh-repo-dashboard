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

// JJOperations implements Operations for jj (Jujutsu) repositories.
type JJOperations struct{}

// NewJJOperations returns a JJOperations.
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
			bookmark.ahead, _ = strconv.Atoi(fields[2])  //nolint:errcheck // jj's template emits digits here
			bookmark.behind, _ = strconv.Atoi(fields[3]) //nolint:errcheck // jj's template emits digits here
		}
	}

	bookmarks := make([]jjBookmark, 0, len(order))
	for _, name := range order {
		bookmarks = append(bookmarks, *byName[name])
	}

	return bookmarks
}

// VCSType implements Operations.
func (*JJOperations) VCSType() models.VCSType {
	return models.VCSTypeJJ
}

func (*JJOperations) runJJ(ctx context.Context, repoPath string, args ...string) (string, error) {
	fullArgs := append([]string{"-R", repoPath}, args...)
	out, err := runCommand(ctx, "", "jj", fullArgs...)
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("jj %s: %s: %w", strings.Join(args, " "), string(exitErr.Stderr), ErrCommandFailed)
		}

		return "", err
	}

	return out, nil
}

// GetRepoSummary implements Operations.
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

	// The remaining fields are best-effort: a failure on any one of them
	// shouldn't blank out an otherwise-populated summary.
	if bookmark != "@" && bookmark != "" {
		upstream, _ := j.GetUpstream(ctx, repoPath, bookmark) //nolint:errcheck // best-effort, see comment above
		summary.Upstream = upstream

		if upstream != "" {
			ahead, behind, _ := j.GetAheadBehind(ctx, repoPath, bookmark, upstream) //nolint:errcheck // best-effort, see comment above
			summary.Ahead = ahead
			summary.Behind = behind
		}
	}

	summary.Unstaged = j.countUnstaged(ctx, repoPath)

	lastMod, _ := j.GetLastModified(ctx, repoPath) //nolint:errcheck // best-effort, see comment above
	if lastMod > 0 {
		summary.LastModified = time.Unix(lastMod, 0)
	}

	return summary, nil
}

// GetCurrentBranch implements Operations.
func (j *JJOperations) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	out, err := j.runJJ(ctx, repoPath, "log", "-r", "@", "-T", jjCurrentBookmarkFormat, "--no-graph")
	if err != nil {
		//nolint:nilerr // "@" is the anonymous-working-copy label, a valid degraded state
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

// GetUpstream implements Operations.
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

// GetAheadBehind implements Operations.
func (j *JJOperations) GetAheadBehind(ctx context.Context, repoPath, branch, upstream string) (int, int, error) {
	if branch == "@" || branch == "" || upstream == "" {
		return 0, 0, nil
	}

	out, err := j.runJJ(ctx, repoPath, "bookmark", "list", "--all-remotes", "-T", jjBookmarkListFormat)
	if err != nil {
		//nolint:nilerr // fall back to "unknown" ahead/behind rather than failing the summary
		return 0, 0, nil
	}

	for _, bookmark := range parseJJBookmarkList(out) {
		if bookmark.name == branch && bookmark.upstream == upstream {
			return bookmark.ahead, bookmark.behind, nil
		}
	}

	return 0, 0, nil
}

// countUnstaged returns jj's uncommitted-change count. There is no
// separate staged/untracked/conflicted state, so those always report zero.
func (j *JJOperations) countUnstaged(ctx context.Context, repoPath string) int {
	out, err := j.runJJ(ctx, repoPath, "status")
	if err != nil {
		return 0
	}

	unstaged := 0
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "A ") || strings.HasPrefix(trimmed, "M ") ||
			strings.HasPrefix(trimmed, "D ") || strings.HasPrefix(trimmed, "R ") {
			unstaged++
		}
	}

	return unstaged
}

// GetStagedCount implements Operations.
func (*JJOperations) GetStagedCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// GetUnstagedCount implements Operations.
func (j *JJOperations) GetUnstagedCount(ctx context.Context, repoPath string) (int, error) {
	return j.countUnstaged(ctx, repoPath), nil
}

// GetUntrackedCount implements Operations.
func (*JJOperations) GetUntrackedCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// GetConflictedCount implements Operations.
func (*JJOperations) GetConflictedCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// GetBranchList implements Operations.
func (j *JJOperations) GetBranchList(ctx context.Context, repoPath string) ([]models.BranchInfo, error) {
	out, err := j.runJJ(ctx, repoPath, "bookmark", "list", "--all-remotes", "-T", jjBookmarkListFormat)
	if err != nil {
		return nil, err
	}

	currentBookmark, _ := j.GetCurrentBranch(ctx, repoPath) //nolint:errcheck // never actually returns an error

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

// GetStashList implements Operations.
func (*JJOperations) GetStashList(_ context.Context, _ string) ([]models.StashDetail, error) {
	return nil, nil
}

// GetWorktreeList implements Operations.
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

// jjCommitLogFieldCount is the number of tab-separated fields in the log
// template below (change id, subject, author, date).
const jjCommitLogFieldCount = 4

// jjRemoteListMinFields is the minimum whitespace-separated fields in a
// `jj git remote list` line (remote name, URL).
const jjRemoteListMinFields = 2

// GetCommitLog implements Operations.
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
		if len(parts) < jjCommitLogFieldCount {
			continue
		}

		ts, _ := strconv.ParseInt(parts[3], 10, 64) //nolint:errcheck // jj's template emits a unix timestamp here

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

// GetLastModified implements Operations.
func (j *JJOperations) GetLastModified(ctx context.Context, repoPath string) (int64, error) {
	format := `committer.timestamp().utc().format("%s")`
	out, err := j.runJJ(ctx, repoPath, "log", "-r", "@", "-T", format, "--no-graph")
	if err != nil {
		return 0, err
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing commit timestamp: %w", err)
	}

	return ts, nil
}

// GetRemoteURL implements Operations.
func (j *JJOperations) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	out, err := j.runJJ(ctx, repoPath, "git", "remote", "list")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "origin") {
			parts := strings.Fields(line)
			if len(parts) >= jjRemoteListMinFields {
				return parts[1], nil
			}
		}
	}

	return "", nil
}

// FetchAll implements Operations.
func (j *JJOperations) FetchAll(ctx context.Context, repoPath string) (bool, string, error) {
	_, err := j.runJJ(ctx, repoPath, "git", "fetch", "--all-remotes")
	if err != nil {
		//nolint:nilerr // failure is reported through the message, not the error field
		return false, err.Error(), nil
	}

	return true, "Fetched from all remotes", nil
}

// PruneRemote implements Operations.
func (*JJOperations) PruneRemote(_ context.Context, _ string) (bool, string, error) {
	return true, "JJ doesn't require explicit pruning", nil
}

// CleanupMergedBranches implements Operations.
func (j *JJOperations) CleanupMergedBranches(ctx context.Context, repoPath string) (bool, string, error) {
	out, err := j.runJJ(ctx, repoPath, "bookmark", "list", "--all-remotes", "-T", jjBookmarkListFormat)
	if err != nil {
		//nolint:nilerr // failure is reported through the message, not the error field
		return false, err.Error(), nil
	}

	var deleted []string
	for _, bookmark := range parseJJBookmarkList(out) {
		if bookmark.name == defaultMainBranch || bookmark.name == masterBranch || bookmark.name == "trunk" {
			continue
		}

		isMerged, err := j.runJJ(ctx, repoPath, "log", "-r",
			bookmark.name+"@origin.."+defaultMainBranch+"@origin", "-T", "change_id", "--no-graph")
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
