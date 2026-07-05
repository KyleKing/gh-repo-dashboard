package vcs

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func jjKey(rest string) string {
	return "jj -R " + testRepoPath + " " + rest
}

const jjCommitFormat = `change_id.short() ++ "\t" ++ description.first_line() ++ "\t" ++ ` +
	`author.name() ++ "\t" ++ committer.timestamp().utc().format("%s")`

const jjTimestampFormat = `committer.timestamp().utc().format("%s")`

func TestJJRunJJWrapsExitError(t *testing.T) {
	t.Parallel()
	ctx := stubCommands(t, nil, map[string]error{
		jjKey("status"): &exec.ExitError{Stderr: []byte("Error: no jj repo")},
	})

	j := NewJJOperations()
	_, err := j.runJJ(ctx, testRepoPath, "status")
	if err == nil {
		t.Fatal("expected error")
	}
	expected := "jj status: Error: no jj repo: command failed"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	if !errors.Is(err, ErrCommandFailed) {
		t.Error("expected error to wrap ErrCommandFailed")
	}
}

func TestJJGetCurrentBranch(t *testing.T) {
	t.Parallel()
	key := jjKey("log -r @ -T " + jjCurrentBookmarkFormat + " --no-graph")

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected string
	}{
		{
			name:     "bookmark on working copy",
			canned:   map[string]string{key: "main"},
			expected: "main",
		},
		{
			name:     "multiple bookmarks returns first",
			canned:   map[string]string{key: "feature main"},
			expected: "feature",
		},
		{
			name:     "no bookmark",
			canned:   map[string]string{key: ""},
			expected: "@",
		},
		{
			name:     "command failure falls back to @",
			failures: map[string]error{key: errBoom},
			expected: "@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			branch, err := j.GetCurrentBranch(ctx, testRepoPath)
			if err != nil {
				t.Fatal(err)
			}
			if branch != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, branch)
			}
		})
	}
}

func TestJJGetUpstream(t *testing.T) {
	t.Parallel()
	key := jjKey("bookmark list --all-remotes -T " + jjBookmarkListFormat)

	tests := []struct {
		name     string
		branch   string
		canned   map[string]string
		failures map[string]error
		expected string
		wantErr  bool
	}{
		{
			name:     "tracked bookmark",
			branch:   "main",
			canned:   map[string]string{key: "main\tlocal\nmain\torigin\t0\t1\n"},
			expected: "main@origin",
		},
		{
			name:     "untracked bookmark",
			branch:   "feature",
			canned:   map[string]string{key: "feature\tlocal\n"},
			expected: "",
		},
		{
			name:     "anonymous working copy",
			branch:   "@",
			expected: "",
		},
		{
			name:     "command failure",
			branch:   "main",
			failures: map[string]error{key: errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			upstream, err := j.GetUpstream(ctx, testRepoPath, tt.branch)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if upstream != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, upstream)
			}
		})
	}
}

func TestJJGetAheadBehind(t *testing.T) {
	t.Parallel()
	key := jjKey("bookmark list --all-remotes -T " + jjBookmarkListFormat)

	tests := []struct {
		name     string
		branch   string
		upstream string
		canned   map[string]string
		failures map[string]error
		ahead    int
		behind   int
	}{
		{
			name:     "ahead and behind",
			branch:   "main",
			upstream: "main@origin",
			canned:   map[string]string{key: "main\tlocal\nmain\torigin\t2\t1\n"},
			ahead:    2,
			behind:   1,
		},
		{
			name:     "in sync",
			branch:   "main",
			upstream: "main@origin",
			canned:   map[string]string{key: "main\tlocal\nmain\torigin\t0\t0\n"},
		},
		{
			name:   "anonymous working copy",
			branch: "@",
		},
		{
			name:     "no upstream",
			branch:   "main",
			upstream: "",
		},
		{
			name:     "listing failure returns zeros",
			branch:   "main",
			upstream: "main@origin",
			failures: map[string]error{key: errBoom},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			ahead, behind, err := j.GetAheadBehind(ctx, testRepoPath, tt.branch, tt.upstream)
			if err != nil {
				t.Fatal(err)
			}
			if ahead != tt.ahead || behind != tt.behind {
				t.Errorf("expected %d/%d, got %d/%d", tt.ahead, tt.behind, ahead, behind)
			}
		})
	}
}

func TestJJCountMethods(t *testing.T) {
	t.Parallel()
	ctx := stubCommands(t, map[string]string{
		jjKey("status"): "Working copy changes:\nM changed.txt\nA new.txt\nWorking copy : abc123",
	}, nil)

	j := NewJJOperations()

	tests := []struct {
		name     string
		fn       func(context.Context, string) (int, error)
		expected int
	}{
		{"staged always zero", j.GetStagedCount, 0},
		{"unstaged from status", j.GetUnstagedCount, 2},
		{"untracked always zero", j.GetUntrackedCount, 0},
		{"conflicted always zero", j.GetConflictedCount, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			count, err := tt.fn(ctx, testRepoPath)
			if err != nil {
				t.Fatal(err)
			}
			if count != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, count)
			}
		})
	}
}

func TestJJGetRepoSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected models.RepoSummary
	}{
		{
			name: "tracked bookmark with changes",
			canned: map[string]string{
				jjKey("log -r @ -T " + jjCurrentBookmarkFormat + " --no-graph"): "main",
				jjKey("bookmark list --all-remotes -T " + jjBookmarkListFormat): "main\tlocal\nmain\torigin\t1\t2\n",
				jjKey("status"): "Working copy changes:\nM file.txt",
				jjKey("log -r @ -T " + jjTimestampFormat + " --no-graph"): "1700000000",
			},
			expected: models.RepoSummary{
				Path:         testRepoPath,
				VCSType:      models.VCSTypeJJ,
				Branch:       "main",
				Upstream:     "main@origin",
				Ahead:        1,
				Behind:       2,
				Unstaged:     1,
				LastModified: time.Unix(1700000000, 0),
			},
		},
		{
			name: "anonymous working copy",
			canned: map[string]string{
				jjKey("log -r @ -T " + jjCurrentBookmarkFormat + " --no-graph"): "",
				jjKey("status"): "The working copy is clean",
				jjKey("log -r @ -T " + jjTimestampFormat + " --no-graph"): "1700000000",
			},
			expected: models.RepoSummary{
				Path:         testRepoPath,
				VCSType:      models.VCSTypeJJ,
				Branch:       "@",
				LastModified: time.Unix(1700000000, 0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			summary, err := j.GetRepoSummary(ctx, testRepoPath)
			if err != nil {
				t.Fatal(err)
			}
			if summary != tt.expected {
				t.Errorf("expected %+v, got %+v", tt.expected, summary)
			}
		})
	}
}

func TestJJGetBranchList(t *testing.T) {
	t.Parallel()
	listKey := jjKey("bookmark list --all-remotes -T " + jjBookmarkListFormat)
	currentKey := jjKey("log -r @ -T " + jjCurrentBookmarkFormat + " --no-graph")

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected []models.BranchInfo
		wantErr  bool
	}{
		{
			name: "local bookmarks",
			canned: map[string]string{
				listKey:    "feature\tlocal\nmain\tlocal\n",
				currentKey: "main",
			},
			expected: []models.BranchInfo{
				{Name: "feature"},
				{Name: "main", IsCurrent: true},
			},
		},
		{
			name: "tracked bookmark",
			canned: map[string]string{
				listKey:    "main\tlocal\nmain\torigin\t1\t0\n",
				currentKey: "",
			},
			expected: []models.BranchInfo{
				{Name: "main", Upstream: "main@origin", Ahead: 1},
			},
		},
		{
			name:     "command failure",
			failures: map[string]error{listKey: errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			branches, err := j.GetBranchList(ctx, testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if len(branches) != len(tt.expected) {
				t.Fatalf("expected %d branches, got %d", len(tt.expected), len(branches))
			}
			for i, expected := range tt.expected {
				if branches[i] != expected {
					t.Errorf("branch %d: expected %+v, got %+v", i, expected, branches[i])
				}
			}
		})
	}
}

func TestJJGetStashList(t *testing.T) {
	t.Parallel()
	j := NewJJOperations()
	stashes, err := j.GetStashList(context.Background(), testRepoPath)
	if err != nil {
		t.Fatal(err)
	}
	if stashes != nil {
		t.Errorf("expected nil, got %v", stashes)
	}
}

func TestJJGetWorktreeList(t *testing.T) {
	t.Parallel()
	key := jjKey("workspace list -T " + jjWorkspaceListFormat)

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected []models.WorktreeInfo
		wantErr  bool
	}{
		{
			name: "two workspaces",
			canned: map[string]string{
				key: "default\t/repo\nfeature\t/repo-feature\n",
			},
			expected: []models.WorktreeInfo{
				{Path: "/repo", Branch: "default"},
				{Path: "/repo-feature", Branch: "feature"},
			},
		},
		{
			name:     "unmatched lines skipped",
			canned:   map[string]string{key: "some unexpected output"},
			expected: nil,
		},
		{
			name:     "command failure",
			failures: map[string]error{key: errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			worktrees, err := j.GetWorktreeList(ctx, testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if len(worktrees) != len(tt.expected) {
				t.Fatalf("expected %d worktrees, got %d", len(tt.expected), len(worktrees))
			}
			for i, expected := range tt.expected {
				if worktrees[i] != expected {
					t.Errorf("worktree %d: expected %+v, got %+v", i, expected, worktrees[i])
				}
			}
		})
	}
}

func TestJJGetCommitLog(t *testing.T) {
	t.Parallel()
	key := jjKey("log -r @~2.. -T " + jjCommitFormat + " --no-graph")

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected []models.CommitInfo
		wantErr  bool
	}{
		{
			name: "two commits",
			canned: map[string]string{
				key: "abcdef12\tfeat: add thing\tKyle King\t1700000000\n" +
					"12345678\tfix: bug\tOther Dev\t1690000000",
			},
			expected: []models.CommitInfo{
				{
					Hash: "abcdef12", ShortHash: "abcdef12", Subject: "feat: add thing",
					Author: "Kyle King", Date: time.Unix(1700000000, 0),
				},
				{
					Hash: "12345678", ShortHash: "12345678", Subject: "fix: bug",
					Author: "Other Dev", Date: time.Unix(1690000000, 0),
				},
			},
		},
		{
			name:     "malformed lines skipped",
			canned:   map[string]string{key: "not-enough-fields"},
			expected: nil,
		},
		{
			name:     "command failure",
			failures: map[string]error{key: errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			commits, err := j.GetCommitLog(ctx, testRepoPath, 2)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if len(commits) != len(tt.expected) {
				t.Fatalf("expected %d commits, got %d", len(tt.expected), len(commits))
			}
			for i, expected := range tt.expected {
				if commits[i] != expected {
					t.Errorf("commit %d: expected %+v, got %+v", i, expected, commits[i])
				}
			}
		})
	}
}

func TestJJGetLastModified(t *testing.T) {
	t.Parallel()
	key := jjKey("log -r @ -T " + jjTimestampFormat + " --no-graph")

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected int64
		wantErr  bool
	}{
		{
			name:     "valid timestamp",
			canned:   map[string]string{key: "1700000000"},
			expected: 1700000000,
		},
		{
			name:     "command failure",
			failures: map[string]error{key: errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			ts, err := j.GetLastModified(ctx, testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if ts != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, ts)
			}
		})
	}
}

func TestJJGetRemoteURL(t *testing.T) {
	t.Parallel()
	key := jjKey("git remote list")

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected string
		wantErr  bool
	}{
		{
			name: "origin present",
			canned: map[string]string{
				key: "origin git@github.com:owner/repo.git\nupstream https://example.com/x.git",
			},
			expected: "git@github.com:owner/repo.git",
		},
		{
			name:     "no origin",
			canned:   map[string]string{key: "upstream https://example.com/x.git"},
			expected: "",
		},
		{
			name:     "command failure",
			failures: map[string]error{key: errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			url, err := j.GetRemoteURL(ctx, testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if url != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, url)
			}
		})
	}
}

func TestJJFetchAllAndPruneRemote(t *testing.T) {
	t.Parallel()
	fetchKey := jjKey("git fetch --all-remotes")

	tests := []struct {
		name       string
		canned     map[string]string
		failures   map[string]error
		run        func(context.Context, *JJOperations) (bool, string, error)
		expectedOK bool
		expected   string
	}{
		{
			name:   "fetch success",
			canned: map[string]string{fetchKey: ""},
			run: func(ctx context.Context, j *JJOperations) (bool, string, error) {
				return j.FetchAll(ctx, testRepoPath)
			},
			expectedOK: true,
			expected:   "Fetched from all remotes",
		},
		{
			name:     "fetch failure",
			failures: map[string]error{fetchKey: errNetworkDown},
			run: func(ctx context.Context, j *JJOperations) (bool, string, error) {
				return j.FetchAll(ctx, testRepoPath)
			},
			expectedOK: false,
			expected:   "network down",
		},
		{
			name: "prune is a no-op",
			run: func(ctx context.Context, j *JJOperations) (bool, string, error) {
				return j.PruneRemote(ctx, testRepoPath)
			},
			expectedOK: true,
			expected:   "JJ doesn't require explicit pruning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			ok, msg, err := tt.run(ctx, NewJJOperations())
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.expectedOK {
				t.Errorf("expected ok=%v, got %v", tt.expectedOK, ok)
			}
			if msg != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, msg)
			}
		})
	}
}

func TestJJCleanupMergedBranches(t *testing.T) {
	t.Parallel()
	listKey := jjKey("bookmark list --all-remotes -T " + jjBookmarkListFormat)

	tests := []struct {
		name       string
		canned     map[string]string
		failures   map[string]error
		expectedOK bool
		expected   string
	}{
		{
			name: "deletes merged bookmark",
			canned: map[string]string{
				listKey: "feature\tlocal\nmain\tlocal\n",
				jjKey("log -r feature@origin..main@origin -T change_id --no-graph"): "",
				jjKey("bookmark delete feature"):                                    "",
			},
			expectedOK: true,
			expected:   "Deleted 1 bookmarks: feature",
		},
		{
			name: "unmerged bookmark kept",
			canned: map[string]string{
				listKey: "feature\tlocal\nmain\tlocal\n",
				jjKey("log -r feature@origin..main@origin -T change_id --no-graph"): "changeid1",
			},
			expectedOK: true,
			expected:   "No merged bookmarks to delete",
		},
		{
			name:       "listing failure",
			failures:   map[string]error{listKey: errBoom},
			expectedOK: false,
			expected:   "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := stubCommands(t, tt.canned, tt.failures)

			j := NewJJOperations()
			ok, msg, err := j.CleanupMergedBranches(ctx, testRepoPath)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.expectedOK {
				t.Errorf("expected ok=%v, got %v", tt.expectedOK, ok)
			}
			if msg != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, msg)
			}
		})
	}
}
