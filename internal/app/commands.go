package app

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/batch"
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func batchFetchAllCmd(paths []string) tea.Cmd {
	return batch.RunTask("Fetch All", paths, batch.FetchAll)
}

func batchPruneRemoteCmd(paths []string) tea.Cmd {
	return batch.RunTask("Prune Remote", paths, batch.PruneRemote)
}

func batchCleanupMergedCmd(paths []string) tea.Cmd {
	return batch.RunTask("Cleanup Merged", paths, batch.CleanupMerged)
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

func loadRepoSummaryCmd(path string) tea.Cmd {
	return func() tea.Msg {
		ops := vcs.GetOperations(path)
		summary, err := ops.GetRepoSummary(context.Background(), path)
		if err == nil {
			summary.NotesFile, summary.NotesFirstLine = models.DetectNotes(path)
		}

		return RepoSummaryLoadedMsg{
			Path:    path,
			Summary: summary,
			Error:   err,
		}
	}
}

func loadPRCmd(path, _, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}

	return func() tea.Msg {
		return PRLoadedMsg{Path: path, PRInfo: nil}
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
		var prs []models.PRInfo
		if summary.Upstream != "" {
			//nolint:errcheck // best-effort, see comment above
			prs, _ = github.GetPRsForRepo(ctx, path, summary.Upstream)
		}

		notesFile, _ := models.DetectNotes(path)
		notesContent := models.ReadNotesFile(path, notesFile)

		return DetailLoadedMsg{
			Path:              path,
			Branches:          branches,
			Stashes:           stashes,
			Worktrees:         worktrees,
			PRs:               prs,
			NotesFile:         notesFile,
			NotesContent:      notesContent,
			DeletableBranches: deletableBranches(ctx, path, branches),
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
		var selectedBranch models.BranchInfo
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
			Branch:       selectedBranch,
			Commits:      commits,
			Staged:       summary.Staged,
			Unstaged:     summary.Unstaged,
			Untracked:    summary.Untracked,
			Conflicted:   summary.Conflicted,
			PRInfo:       summary.PRInfo,
			WorkflowInfo: summary.WorkflowInfo,
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

func loadPRCountCmd(path, upstream string) tea.Cmd {
	if upstream == "" {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		count, err := github.GetPRCount(ctx, path, upstream)
		if err != nil {
			return PRCountLoadedMsg{Path: path, Count: 0}
		}

		return PRCountLoadedMsg{Path: path, Count: count}
	}
}

func loadPRDetailCmd(repoPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		detail, err := github.GetPRDetail(ctx, repoPath, prNumber)
		if err != nil {
			return PRDetailLoadedMsg{
				Path:     repoPath,
				PRNumber: prNumber,
				Error:    err,
			}
		}

		return PRDetailLoadedMsg{
			Path:     repoPath,
			PRNumber: prNumber,
			Detail:   *detail,
		}
	}
}

func prefetchPRDetailCmd(repoPath string, prNumber int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		// Prefetch runs in background and populates cache
		// No message sent to avoid UI updates during prefetch
		//nolint:errcheck // prefetch only warms the cache, no message is sent
		_, _ = github.GetPRDetail(ctx, repoPath, prNumber)

		return nil
	}
}

func openOrCreatePRCmd(_, _ string) tea.Cmd {
	return func() tea.Msg {
		return PRCreatedMsg{
			URL:   "",
			Error: nil,
		}
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
			cmd = exec.CommandContext(ctx, "open", url)
		case "linux":
			cmd = exec.CommandContext(ctx, "xdg-open", url)
		case "windows":
			cmd = exec.CommandContext(ctx, "cmd", "/c", "start", url)
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
