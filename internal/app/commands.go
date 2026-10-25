package app

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/forge/github"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/copier"
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

// Batch task display names shared by :commands, operator keys, and batch cmds.
const (
	taskCleanupMerged = "Cleanup Merged"
	taskFetchAll      = "Fetch All"
	taskRefreshPRs    = "Refresh PRs"
)

func batchFetchAllCmd(paths []string) tea.Cmd {
	return batch.RunTask(taskFetchAll, paths, batch.FetchAll)
}

func batchPruneRemoteCmd(paths []string) tea.Cmd {
	return batch.RunTask("Prune Remote", paths, batch.PruneRemote)
}

func batchRefreshPRsCmd(paths []string) tea.Cmd {
	return batch.RunTask(taskRefreshPRs, paths, batch.RefreshPRs)
}

func batchCleanupMergedCmd(paths []string) tea.Cmd {
	return batch.RunTask(taskCleanupMerged, paths, batch.CleanupMerged)
}

func batchPreviewCleanupCmd(paths []string) tea.Cmd {
	return batch.RunTask("Cleanup Merged (dry run)", paths, batch.PreviewCleanup)
}

func discoverReposCmd(scanPaths []string, maxDepth int) tea.Cmd {
	return func() tea.Msg {
		paths := discovery.DiscoverRepos(scanPaths, maxDepth)
		return ReposDiscoveredMsg{Paths: paths}
	}
}

// ghAuthTimeout bounds how long checkGHAuthCmd waits on gh's own network call
// before assuming it is unreachable.
const ghAuthTimeout = 2 * time.Second

// checkGHAuthCmd reports what, if anything, is wrong with the gh CLI setup.
// It runs as a normal tea.Cmd rather than blocking startup: gh's own PR and
// workflow reads already degrade to a dash when it cannot authenticate, so
// this is an advisory notice, not worth holding the first frame on a network
// round trip that can take the better part of a second.
func checkGHAuthCmd() tea.Cmd {
	return func() tea.Msg {
		if _, err := exec.LookPath("gh"); err != nil {
			return GHAuthCheckedMsg{
				Message: "gh not found on PATH; PR and workflow columns will be blank." +
					" Install https://cli.github.com and run 'gh auth login'.",
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), ghAuthTimeout)
		defer cancel()

		if err := exec.CommandContext(ctx, "gh", "auth", "status").Run(); err != nil && ctx.Err() == nil {
			return GHAuthCheckedMsg{
				Message: "gh is not authenticated; PR and workflow columns will be blank. Run 'gh auth login'.",
			}
		}

		return GHAuthCheckedMsg{}
	}
}

func loadRepoSummaryCmd(path string) tea.Cmd {
	return func() tea.Msg {
		summary, err := models.ReadSummary(context.Background(), vcs.GetOperations(path), path)

		return RepoSummaryLoadedMsg{
			Path:    path,
			Summary: summary,
			Error:   err,
		}
	}
}

// loadPRCmd reads the pull request open on the repo's checked-out branch.
func loadPRCmd(path, remoteID, branch, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}

	return func() tea.Msg {
		info, err := github.GetPRForBranch(context.Background(), path, remoteID, branch, upstream)
		if err != nil {
			return PRLoadedMsg{Path: path, PRInfo: nil}
		}

		return PRLoadedMsg{Path: path, PRInfo: info}
	}
}

// loadDefaultBranchCICmd reads the CI runs for the repo's default branch head.
// The branch and commit resolve from local refs, so the one gh call is the
// whole API cost.
func loadDefaultBranchCICmd(path, remoteID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		ci, err := github.GetDefaultBranchCI(ctx, path, remoteID)
		if err != nil {
			return WorkflowLoadedMsg{Path: path, Error: err}
		}

		return WorkflowLoadedMsg{Path: path, Branch: ci.Branch, CI: ci}
	}
}

func staleDefaultBranchCICmd(path, remoteID string) tea.Cmd {
	return func() tea.Msg {
		ci, _, ok := github.CachedDefaultBranchCIStale(context.Background(), path, remoteID)
		if !ok {
			return WorkflowLoadedMsg{Path: path, Stale: true, Miss: true}
		}

		return WorkflowLoadedMsg{Path: path, Branch: ci.Branch, CI: ci, Stale: true}
	}
}

func loadCopierInfoCmd(path string) tea.Cmd {
	return func() tea.Msg {
		//nolint:errcheck // best-effort, absence just renders emDash
		info, _ := copier.GetTemplateInfo(context.Background(), path)

		return CopierInfoLoadedMsg{Path: path, Info: info}
	}
}

func loadNotesContentCmd(path string, files []models.NoteFile) tea.Cmd {
	return func() tea.Msg {
		return NotesContentLoadedMsg{Path: path, Files: models.ReadNotesFiles(path, files)}
	}
}

func loadDetailCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		ops := vcs.GetOperations(path)

		// DetailLoadedMsg has no error field: a failed section just renders
		// empty rather than blocking the rest of the detail view.
		branches, _ := ops.GetBranchList(ctx, path)    //nolint:errcheck // best-effort, see comment above
		stashes, _ := ops.GetStashList(ctx, path)      //nolint:errcheck // best-effort, see comment above
		worktrees, _ := ops.GetWorktreeList(ctx, path) //nolint:errcheck // best-effort, see comment above

		summary, _ := ops.GetRepoSummary(ctx, path) //nolint:errcheck // best-effort, see comment above
		var prs []forge.PullRequest
		if summary.Upstream != "" {
			//nolint:errcheck // best-effort, see comment above
			prs, _ = github.GetPRsForRepo(ctx, path, summary.RemoteID, summary.Upstream)
		}

		notesFiles := models.DetectNotes(path)
		notesContents := models.ReadNotesFiles(path, notesFiles)

		//nolint:errcheck // best-effort, see comment above
		newestFile, newestFileTime, _ := ops.GetNewestModifiedFile(ctx, path)

		return DetailLoadedMsg{
			Path:              path,
			Branches:          branches,
			Stashes:           stashes,
			Worktrees:         worktrees,
			PRs:               prs,
			NotesFiles:        notesContents,
			DeletableBranches: deletableBranches(ctx, path, summary.RemoteID, branches),
			NewestFile:        newestFile,
			NewestFileTime:    newestFileTime,
		}
	}
}

func loadBranchDetailCmd(repoPath, branchName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		ops := vcs.GetOperations(repoPath)

		// BranchDetailLoadedMsg has no error field: a failed section just
		// renders empty rather than blocking the rest of the detail view.
		branches, _ := ops.GetBranchList(ctx, repoPath) //nolint:errcheck // best-effort, see comment above
		var selectedBranch vcs.BranchInfo
		for _, b := range branches {
			if b.Name == branchName {
				selectedBranch = b
				break
			}
		}

		//nolint:errcheck // best-effort, see comment above
		commits, _ := ops.GetCommitLog(ctx, repoPath, branchDetailLogLimit)

		//nolint:errcheck // best-effort, see comment above
		summary, _ := ops.GetRepoSummary(ctx, repoPath)

		detail := models.BranchDetail{
			Branch:     selectedBranch,
			Commits:    commits,
			Staged:     summary.Staged,
			Unstaged:   summary.Unstaged,
			Untracked:  summary.Untracked,
			Conflicted: summary.Conflicted,
		}

		if defaultBranch := findDefaultBranch(branches); defaultBranch != "" && defaultBranch != branchName {
			if ahead, behind, err := ops.CompareBranches(ctx, repoPath, branchName, defaultBranch); err == nil {
				detail.DefaultBranch = defaultBranch
				detail.DefaultAhead = ahead
				detail.DefaultBehind = behind
			}
		}

		return BranchDetailLoadedMsg{
			Path:   repoPath,
			Detail: detail,
		}
	}
}

func loadOpenPRsCmd(path, remoteID, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}

	return func() tea.Msg {
		prs, err := github.GetPRsForRepo(context.Background(), path, remoteID, upstream)

		return OpenPRsLoadedMsg{Path: path, PRs: prs, Error: err}
	}
}

func staleOpenPRsCmd(path, remoteID, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}

	return func() tea.Msg {
		prs, _, ok := github.CachedPRsStale(path, remoteID, upstream)
		if !ok {
			return OpenPRsLoadedMsg{Path: path, Stale: true, Miss: true}
		}

		return OpenPRsLoadedMsg{Path: path, PRs: prs, Stale: true}
	}
}

const viewerTimeout = 5 * time.Second

// loadViewerCmd names the signed-in GitHub user. An unreadable login leaves no pull request waiting on anyone.
func loadViewerCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), viewerTimeout)
		defer cancel()

		out, err := exec.CommandContext(ctx, "gh", "api", "user", "--jq", ".login").Output()
		if err != nil {
			return ViewerLoadedMsg{}
		}

		return ViewerLoadedMsg{Login: strings.TrimSpace(string(out))}
	}
}

// loadDueTasksCmd runs off the UI thread because a task's where may walk the repo's git directory.
func loadDueTasksCmd(sel tasks.Selector, summary models.RepoSummary) tea.Cmd {
	return func() tea.Msg {
		var due []string

		for _, task := range sel.Tasks {
			if task.Every == "" {
				continue
			}

			if isDue, err := sel.Due(task, summary); err == nil && isDue {
				due = append(due, task.Name)
			}
		}

		return DueTasksLoadedMsg{Path: summary.Path, Tasks: due}
	}
}

func loadChangedPathsCmd(path string, vcsType vcs.Type) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), changedPathsTimeout)
		defer cancel()

		args := []string{"git", "status", "--porcelain"}
		if vcsType == vcs.TypeJJ {
			args = []string{"jj", diffWord, "--summary", "--no-pager"}
		}

		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = path

		out, err := cmd.Output()
		if err != nil {
			return ChangedPathsLoadedMsg{Path: path}
		}

		return ChangedPathsLoadedMsg{Path: path, Paths: parseChangedPaths(string(out))}
	}
}

const (
	changedPathsTimeout = 5 * time.Second
	diffWord            = "diff"
)

func parseChangedPaths(out string) []string {
	var paths []string

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if _, rest, found := strings.Cut(line, " "); found && strings.TrimSpace(rest) != "" {
			paths = append(paths, strings.TrimSpace(rest))
		}
	}

	return paths
}

// loadBranchCountCmd reads a repo's local branch count for the Repos list.
// The list's BRs column used to piggyback on the branch list the expanded
// region loads, which only ever runs for whichever repo has that region open,
// so every other row's count sat empty forever. This is its own read, local
// and cheap, that runs for every repo the way the PR count does.
func loadBranchCountCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		branches, err := vcs.GetOperations(path).GetBranchList(ctx, path)
		if err != nil {
			return BranchCountLoadedMsg{Path: path, Count: 0}
		}

		return BranchCountLoadedMsg{Path: path, Count: len(branches)}
	}
}

// loadStashDiffCmd reads one stash's full patch, rendered by the configured
// external diff viewer where there is one. Since jj has no stashes, the call is
// git-only and a non-git repo simply reports nothing.
func loadStashDiffCmd(repoPath string, index, width int) tea.Cmd {
	return func() tea.Msg {
		msg := StashDiffLoadedMsg{Path: repoPath, Index: index}

		git, ok := vcs.GetOperations(repoPath).(*vcs.GitOperations)
		if !ok {
			return msg
		}

		ctx := context.Background()
		if viewer := git.ExternalDiffCommand(ctx, repoPath); viewer != "" {
			if diff, err := git.StashDiffExternal(ctx, repoPath, index, width, viewer); err == nil {
				msg.Diff = diff

				return msg
			}
		}

		//nolint:errcheck // a stash we cannot read reports an empty diff
		msg.Diff, _ = git.StashDiff(ctx, repoPath, index)

		return msg
	}
}

// loadStashDiffstatCmd reads one stash's diffstat. Since jj has no stashes,
// the call is git-only and a non-git repo simply reports nothing.
func loadStashDiffstatCmd(repoPath string, index int) tea.Cmd {
	return func() tea.Msg {
		msg := StashDiffstatLoadedMsg{Path: repoPath, Index: index}

		git, ok := vcs.GetOperations(repoPath).(*vcs.GitOperations)
		if !ok {
			return msg
		}

		//nolint:errcheck // a stash we cannot read reports an empty diffstat
		msg.Diffstat, _ = git.StashDiffstat(context.Background(), repoPath, index)

		return msg
	}
}

// loadUncommittedDiffCmd reads the working tree's full patch against HEAD for
// repoPath, rendered by the configured external diff viewer where there is
// one. A non-git repo simply reports nothing.
func loadUncommittedDiffCmd(repoPath string, width int) tea.Cmd {
	return func() tea.Msg {
		msg := UncommittedDiffLoadedMsg{Path: repoPath}

		git, ok := vcs.GetOperations(repoPath).(*vcs.GitOperations)
		if !ok {
			return msg
		}

		ctx := context.Background()
		if viewer := git.ExternalDiffCommand(ctx, repoPath); viewer != "" {
			if diff, err := git.UncommittedDiffExternal(ctx, repoPath, width, viewer); err == nil {
				msg.Diff = diff

				return msg
			}
		}

		//nolint:errcheck // a repo we cannot read reports an empty diff
		msg.Diff, _ = git.UncommittedDiff(ctx, repoPath)

		return msg
	}
}

// loadUncommittedDiffstatCmd reads the working tree's diffstat against HEAD
// for repoPath. A non-git repo simply reports nothing.
func loadUncommittedDiffstatCmd(repoPath string) tea.Cmd {
	return func() tea.Msg {
		msg := UncommittedDiffstatLoadedMsg{Path: repoPath}

		git, ok := vcs.GetOperations(repoPath).(*vcs.GitOperations)
		if !ok {
			return msg
		}

		//nolint:errcheck // a repo we cannot read reports an empty diffstat
		msg.Diffstat, _ = git.UncommittedDiffstat(context.Background(), repoPath)

		return msg
	}
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.CommandContext(ctx, "pbcopy")
		case "linux":
			linuxClipboardCmd := "type xclip >/dev/null 2>&1 && xclip -selection clipboard || " +
				"type xsel >/dev/null 2>&1 && xsel --clipboard --input || " +
				"type wl-copy >/dev/null 2>&1 && wl-copy"
			cmd = exec.CommandContext(ctx, "sh", "-c", linuxClipboardCmd)
		case "windows":
			cmd = exec.CommandContext(ctx, "clip")
		default:
			return StatusMsg{Message: "Clipboard not supported on this platform"}
		}

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
		}

		if err := cmd.Start(); err != nil {
			return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
		}

		if _, err := stdin.Write([]byte(text)); err != nil {
			return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
		}

		if err := stdin.Close(); err != nil {
			return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
		}

		if err := cmd.Wait(); err != nil {
			return StatusMsg{Message: fmt.Sprintf("Failed to copy: %v", err)}
		}

		return CopySuccessMsg{Text: text}
	}
}

func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.CommandContext(ctx, "open", url) // #nosec G204 -- fixed opener, url from gh output
		case "linux":
			cmd = exec.CommandContext(ctx, "xdg-open", url) // #nosec G204 -- fixed opener, url from gh output
		case "windows":
			cmd = exec.CommandContext(ctx, "cmd", "/c", "start", url) // #nosec G204 -- fixed opener, url from gh output
		default:
			return StatusMsg{Message: "URL opening not supported on this platform"}
		}

		if err := cmd.Start(); err != nil {
			return StatusMsg{Message: fmt.Sprintf("Failed to open URL: %v", err)}
		}

		return URLOpenedMsg{URL: url}
	}
}

func clearStatusAfterDelay() tea.Cmd {
	return tea.Tick(statusClearDelay, func(_ time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}
