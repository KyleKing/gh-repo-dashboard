package vcs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

const (
	porcelainStatusCodeLen = 2
	minRemoteURLPathParts  = 3
)

// GitOperations implements Operations for git repositories.
type GitOperations struct{}

// NewGitOperations returns a GitOperations.
func NewGitOperations() *GitOperations {
	return &GitOperations{}
}

// VCSType implements Operations.
func (*GitOperations) VCSType() models.VCSType {
	return models.VCSTypeGit
}

func (*GitOperations) runGit(ctx context.Context, repoPath string, args ...string) (string, error) {
	out, err := runCommand(ctx, repoPath, "git", args...)
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), string(exitErr.Stderr), ErrCommandFailed)
		}

		return "", err
	}

	return out, nil
}

// GetRepoSummary implements Operations.
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

	// The remaining fields are best-effort: a failure on any one of them
	// shouldn't blank out an otherwise-populated summary.
	upstream, _ := g.GetUpstream(ctx, repoPath, branch) //nolint:errcheck // best-effort, see comment above
	summary.Upstream = upstream

	if upstream != "" {
		//nolint:errcheck // best-effort, see comment above
		ahead, behind, _ := g.GetAheadBehind(ctx, repoPath, branch, upstream)
		summary.Ahead = ahead
		summary.Behind = behind
	}

	counts := g.getStatusCounts(ctx, repoPath)
	summary.Staged = counts.staged
	summary.Unstaged = counts.unstaged
	summary.Untracked = counts.untracked
	summary.Conflicted = counts.conflicted

	stashCount, _ := g.getStashCount(ctx, repoPath) //nolint:errcheck // best-effort, see comment above
	summary.StashCount = stashCount

	lastMod, _ := g.GetLastModified(ctx, repoPath) //nolint:errcheck // best-effort, see comment above
	if lastMod > 0 {
		summary.LastModified = time.Unix(lastMod, 0)
	}

	return summary, nil
}

// GetCurrentBranch implements Operations.
func (g *GitOperations) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	out, err := g.runGit(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if out == "HEAD" {
		hash, err := g.runGit(ctx, repoPath, "rev-parse", "--short", "HEAD")
		if err != nil {
			//nolint:nilerr // degrade to plain "HEAD" label rather than failing the whole summary
			return "HEAD", nil
		}

		return fmt.Sprintf("(%s)", hash), nil
	}

	return out, nil
}

// GetUpstream implements Operations.
func (g *GitOperations) GetUpstream(ctx context.Context, repoPath, branch string) (string, error) {
	out, err := g.runGit(ctx, repoPath, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return "", err
	}

	return out, nil
}

// GetAheadBehind implements Operations.
//
//nolint:gocritic // matches the Operations interface's (ahead, behind int, err error)
func (g *GitOperations) GetAheadBehind(ctx context.Context, repoPath, branch, upstream string) (int, int, error) {
	out, err := g.runGit(ctx, repoPath, "rev-list", "--left-right", "--count", fmt.Sprintf("%s...%s", branch, upstream))
	if err != nil {
		return 0, 0, err
	}

	const revListFieldCount = 2 // ahead, behind

	parts := strings.Fields(out)
	if len(parts) != revListFieldCount {
		return 0, 0, fmt.Errorf("rev-list output %q: %w", out, ErrUnexpectedOutput)
	}

	ahead, _ := strconv.Atoi(parts[0])  //nolint:errcheck // regex guarantees digits
	behind, _ := strconv.Atoi(parts[1]) //nolint:errcheck // regex guarantees digits

	return ahead, behind, nil
}

type statusCounts struct {
	staged     int
	unstaged   int
	untracked  int
	conflicted int
}

// classifyPorcelainEntry categorizes one `git status --porcelain` entry by its
// two-character XY status code.
func classifyPorcelainEntry(x, y byte) statusCounts {
	switch {
	case x == 'U' || y == 'U' || (x == 'D' && y == 'D') || (x == 'A' && y == 'A'):
		return statusCounts{conflicted: 1}
	case x == '?':
		return statusCounts{untracked: 1}
	default:
		var counts statusCounts
		if x != ' ' && x != '?' {
			counts.staged = 1
		}
		if y != ' ' && y != '?' {
			counts.unstaged = 1
		}

		return counts
	}
}

func (g *GitOperations) getStatusCounts(ctx context.Context, repoPath string) statusCounts {
	var counts statusCounts

	out, err := g.runGit(ctx, repoPath, "status", "--porcelain", "-z")
	if err != nil {
		return counts
	}

	entries := strings.Split(out, "\x00")
	for _, entry := range entries {
		if len(entry) < porcelainStatusCodeLen {
			continue
		}

		entryCounts := classifyPorcelainEntry(entry[0], entry[1])
		counts.staged += entryCounts.staged
		counts.unstaged += entryCounts.unstaged
		counts.untracked += entryCounts.untracked
		counts.conflicted += entryCounts.conflicted
	}

	return counts
}

// GetStagedCount implements Operations.
func (g *GitOperations) GetStagedCount(ctx context.Context, repoPath string) (int, error) {
	return g.getStatusCounts(ctx, repoPath).staged, nil
}

// GetUnstagedCount implements Operations.
func (g *GitOperations) GetUnstagedCount(ctx context.Context, repoPath string) (int, error) {
	return g.getStatusCounts(ctx, repoPath).unstaged, nil
}

// GetUntrackedCount implements Operations.
func (g *GitOperations) GetUntrackedCount(ctx context.Context, repoPath string) (int, error) {
	return g.getStatusCounts(ctx, repoPath).untracked, nil
}

// GetConflictedCount implements Operations.
func (g *GitOperations) GetConflictedCount(ctx context.Context, repoPath string) (int, error) {
	return g.getStatusCounts(ctx, repoPath).conflicted, nil
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

// branchListFieldCount is the number of tab-separated fields in the
// for-each-ref format below (refname, upstream, track, date, HEAD marker).
// runCommand trims trailing whitespace from the output, so the final line can
// lose empty trailing fields (e.g. a last branch with no upstream); the parser
// pads missing fields back to this count.
const branchListFieldCount = 5

// GetBranchList implements Operations.
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
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		for len(parts) < branchListFieldCount {
			parts = append(parts, "")
		}

		var ahead, behind int
		if matches := trackRe.FindStringSubmatch(parts[2]); matches != nil {
			if matches[1] != "" {
				ahead, _ = strconv.Atoi(matches[1]) //nolint:errcheck // regex guarantees digits
			}
			if matches[2] != "" {
				behind, _ = strconv.Atoi(matches[2]) //nolint:errcheck // regex guarantees digits
			}
			if matches[3] != "" {
				behind, _ = strconv.Atoi(matches[3]) //nolint:errcheck // regex guarantees digits
			}
		}

		ts, _ := strconv.ParseInt(parts[3], 10, 64) //nolint:errcheck // git emits a unix timestamp here

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

// stashListFieldCount is the number of tab-separated fields in the
// stash-list format below (reflog short name, subject, date).
const stashListFieldCount = 3

// GetStashList implements Operations.
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
		if len(parts) < stashListFieldCount {
			continue
		}

		var index int
		if matches := stashRe.FindStringSubmatch(parts[0]); matches != nil {
			index, _ = strconv.Atoi(matches[1]) //nolint:errcheck // regex guarantees digits
		}

		ts, _ := strconv.ParseInt(parts[2], 10, 64) //nolint:errcheck // git emits a unix timestamp here

		stashes = append(stashes, models.StashDetail{
			Index:   index,
			Message: parts[1],
			Date:    time.Unix(ts, 0),
		})
	}

	return stashes, nil
}

// GetWorktreeList implements Operations.
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

// commitLogFieldCount is the number of tab-separated fields in the log
// format below (hash, short hash, subject, author, date).
const commitLogFieldCount = 5

// GetCommitLog implements Operations.
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
		if len(parts) < commitLogFieldCount {
			continue
		}

		ts, _ := strconv.ParseInt(parts[4], 10, 64) //nolint:errcheck // git emits a unix timestamp here

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

// GetLastModified implements Operations.
func (g *GitOperations) GetLastModified(ctx context.Context, repoPath string) (int64, error) {
	out, err := g.runGit(ctx, repoPath, "log", "-1", "--format=%ct")
	if err != nil {
		return 0, err
	}

	ts, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing commit timestamp: %w", err)
	}

	return ts, nil
}

// GetRemoteURL implements Operations.
func (g *GitOperations) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	out, err := g.runGit(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}

	return out, nil
}

// FetchAll implements Operations.
//
//nolint:gocritic // matches the Operations interface's (ok bool, msg string, err error)
func (g *GitOperations) FetchAll(ctx context.Context, repoPath string) (bool, string, error) {
	_, err := g.runGit(ctx, repoPath, "fetch", "--all", "--prune")
	if err != nil {
		//nolint:nilerr // failure is reported through the message, not the error field
		return false, err.Error(), nil
	}

	return true, "Fetched from all remotes", nil
}

// PruneRemote implements Operations.
//
//nolint:gocritic // matches the Operations interface's (ok bool, msg string, err error)
func (g *GitOperations) PruneRemote(ctx context.Context, repoPath string) (bool, string, error) {
	_, err := g.runGit(ctx, repoPath, "remote", "prune", "origin")
	if err != nil {
		//nolint:nilerr // failure is reported through the message, not the error field
		return false, err.Error(), nil
	}

	return true, "Pruned stale remote branches", nil
}

// CleanupMergedBranches implements Operations.
//
//nolint:gocritic // matches the Operations interface's (ok bool, msg string, err error)
func (g *GitOperations) CleanupMergedBranches(ctx context.Context, repoPath string) (bool, string, error) {
	mainBranch := defaultMainBranch
	if _, err := g.runGit(ctx, repoPath, "rev-parse", "--verify", defaultMainBranch); err != nil {
		_, err := g.runGit(ctx, repoPath, "rev-parse", "--verify", masterBranch)
		if err != nil {
			//nolint:nilerr // failure is reported through the message, not the error field
			return false, "Could not find main or master branch", nil
		}
		mainBranch = masterBranch
	}

	out, err := g.runGit(ctx, repoPath, "branch", "--merged", mainBranch)
	if err != nil {
		//nolint:nilerr // failure is reported through the message, not the error field
		return false, err.Error(), nil
	}

	var deleted []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		branch := strings.TrimSpace(scanner.Text())
		branch = strings.TrimPrefix(branch, "* ")

		if branch == mainBranch || branch == masterBranch || branch == defaultMainBranch || branch == "" {
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

// ExtractRepoPath derives an "owner/repo" style path from a git remote URL.
func ExtractRepoPath(remoteURL string) string {
	url := strings.TrimSuffix(remoteURL, ".git")

	switch {
	case strings.HasPrefix(url, "git@"):
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
	case strings.HasPrefix(url, "https://"):
		url = strings.TrimPrefix(url, "https://")
	case strings.HasPrefix(url, "http://"):
		url = strings.TrimPrefix(url, "http://")
	}

	parts := strings.Split(url, "/")
	if len(parts) >= minRemoteURLPathParts {
		return filepath.Join(parts[len(parts)-2], parts[len(parts)-1])
	}

	return ""
}
