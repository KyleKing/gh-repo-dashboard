package vcs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

const testRepoPath = "/repo"

var (
	errUnexpectedCommand = errors.New("unexpected command")
	errBoom              = errors.New("boom")
	errNoUpstream        = errors.New("no upstream")
	errNoSuchRemote      = errors.New("no such remote")
	errNetworkDown       = errors.New("network down")
	errNoRemote          = errors.New("no remote")
	errUnknownRevision   = errors.New("unknown revision")
)

func stubCommands(t *testing.T, canned map[string]string, failures map[string]error) {
	t.Helper()
	orig := runCommand
	runCommand = func(ctx context.Context, dir, name string, args ...string) (string, error) {
		key := name + " " + strings.Join(args, " ")
		if err, ok := failures[key]; ok {
			return "", err
		}
		if out, ok := canned[key]; ok {
			return out, nil
		}

		return "", fmt.Errorf("%s: %w", key, errUnexpectedCommand)
	}
	t.Cleanup(func() { runCommand = orig })
}

func TestRunCommandReal(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	t.Run("success trims output", func(t *testing.T) {
		out, err := runCommand(context.Background(), t.TempDir(), "git", "--version")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(out, "git version") {
			t.Errorf("unexpected output: %q", out)
		}
		if strings.HasSuffix(out, "\n") {
			t.Error("output not trimmed")
		}
	})

	t.Run("missing binary returns error", func(t *testing.T) {
		if _, err := runCommand(context.Background(), t.TempDir(), "definitely-missing-binary-xyz"); err == nil {
			t.Error("expected error")
		}
	})

	t.Run("exit error is wrapped by runGit", func(t *testing.T) {
		g := NewGitOperations()
		_, err := g.runGit(context.Background(), t.TempDir(), "rev-parse", "--verify", "HEAD")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.HasPrefix(err.Error(), "git rev-parse --verify HEAD:") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestGitRunGitWrapsExitError(t *testing.T) {
	stubCommands(t, nil, map[string]error{
		"git status --porcelain -z": &exec.ExitError{Stderr: []byte("fatal: not a repo")},
	})

	g := NewGitOperations()
	_, err := g.runGit(context.Background(), testRepoPath, "status", "--porcelain", "-z")
	if err == nil {
		t.Fatal("expected error")
	}
	expected := "git status --porcelain -z: fatal: not a repo: command failed"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
	if !errors.Is(err, ErrCommandFailed) {
		t.Error("expected error to wrap ErrCommandFailed")
	}
}

func TestGitGetCurrentBranch(t *testing.T) {
	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected string
		wantErr  bool
	}{
		{
			name:     "on branch",
			canned:   map[string]string{"git rev-parse --abbrev-ref HEAD": "main"},
			expected: "main",
		},
		{
			name: "detached head uses short hash",
			canned: map[string]string{
				"git rev-parse --abbrev-ref HEAD": "HEAD",
				"git rev-parse --short HEAD":      "abc1234",
			},
			expected: "(abc1234)",
		},
		{
			name:     "detached head with short hash failure",
			canned:   map[string]string{"git rev-parse --abbrev-ref HEAD": "HEAD"},
			failures: map[string]error{"git rev-parse --short HEAD": errBoom},
			expected: "HEAD",
		},
		{
			name:     "command failure",
			failures: map[string]error{"git rev-parse --abbrev-ref HEAD": errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			branch, err := g.GetCurrentBranch(context.Background(), testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if branch != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, branch)
			}
		})
	}
}

func TestGitGetUpstream(t *testing.T) {
	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected string
		wantErr  bool
	}{
		{
			name:     "has upstream",
			canned:   map[string]string{"git rev-parse --abbrev-ref main@{upstream}": "origin/main"},
			expected: "origin/main",
		},
		{
			name:     "no upstream configured",
			failures: map[string]error{"git rev-parse --abbrev-ref main@{upstream}": errNoUpstream},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			upstream, err := g.GetUpstream(context.Background(), testRepoPath, "main")
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if upstream != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, upstream)
			}
		})
	}
}

func TestGitGetAheadBehind(t *testing.T) {
	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		ahead    int
		behind   int
		wantErr  bool
	}{
		{
			name:   "ahead and behind",
			canned: map[string]string{"git rev-list --left-right --count main...origin/main": "3\t2"},
			ahead:  3,
			behind: 2,
		},
		{
			name:    "malformed output",
			canned:  map[string]string{"git rev-list --left-right --count main...origin/main": "garbage"},
			wantErr: true,
		},
		{
			name:     "command failure",
			failures: map[string]error{"git rev-list --left-right --count main...origin/main": errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			ahead, behind, err := g.GetAheadBehind(context.Background(), testRepoPath, "main", "origin/main")
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if ahead != tt.ahead || behind != tt.behind {
				t.Errorf("expected %d/%d, got %d/%d", tt.ahead, tt.behind, ahead, behind)
			}
		})
	}
}

func TestGitStatusCountMethods(t *testing.T) {
	stubCommands(t, map[string]string{
		"git status --porcelain -z": "M  staged.txt\x00 M unstaged.txt\x00?? new.txt\x00UU conflict.txt\x00",
	}, nil)

	g := NewGitOperations()
	ctx := context.Background()

	tests := []struct {
		name     string
		fn       func(context.Context, string) (int, error)
		expected int
	}{
		{"staged", g.GetStagedCount, 1},
		{"unstaged", g.GetUnstagedCount, 1},
		{"untracked", g.GetUntrackedCount, 1},
		{"conflicted", g.GetConflictedCount, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestGitGetRepoSummary(t *testing.T) {
	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected models.RepoSummary
		wantErr  bool
	}{
		{
			name: "clean repo with upstream",
			canned: map[string]string{
				"git rev-parse --abbrev-ref HEAD":                      "main",
				"git rev-parse --abbrev-ref main@{upstream}":           "origin/main",
				"git rev-list --left-right --count main...origin/main": "0\t0",
				"git status --porcelain -z":                            "",
				"git stash list":                                       "",
				"git log -1 --format=%ct":                              "1700000000",
			},
			expected: models.RepoSummary{
				Path:         testRepoPath,
				VCSType:      models.VCSTypeGit,
				Branch:       "main",
				Upstream:     "origin/main",
				LastModified: time.Unix(1700000000, 0),
			},
		},
		{
			name: "dirty repo ahead and behind",
			canned: map[string]string{
				"git rev-parse --abbrev-ref HEAD":                      "main",
				"git rev-parse --abbrev-ref main@{upstream}":           "origin/main",
				"git rev-list --left-right --count main...origin/main": "3\t2",
				"git status --porcelain -z":                            "M  a.txt\x00 M b.txt\x00?? c.txt\x00",
				"git stash list":                                       "stash@{0}: WIP on main\nstash@{1}: save",
				"git log -1 --format=%ct":                              "1700000000",
			},
			expected: models.RepoSummary{
				Path:         testRepoPath,
				VCSType:      models.VCSTypeGit,
				Branch:       "main",
				Upstream:     "origin/main",
				Ahead:        3,
				Behind:       2,
				Staged:       1,
				Unstaged:     1,
				Untracked:    1,
				StashCount:   2,
				LastModified: time.Unix(1700000000, 0),
			},
		},
		{
			name: "no upstream skips ahead behind",
			canned: map[string]string{
				"git rev-parse --abbrev-ref HEAD": "feature",
				"git status --porcelain -z":       "",
				"git stash list":                  "",
				"git log -1 --format=%ct":         "1700000000",
			},
			failures: map[string]error{
				"git rev-parse --abbrev-ref feature@{upstream}": errNoUpstream,
			},
			expected: models.RepoSummary{
				Path:         testRepoPath,
				VCSType:      models.VCSTypeGit,
				Branch:       "feature",
				LastModified: time.Unix(1700000000, 0),
			},
		},
		{
			name: "branch failure returns error",
			failures: map[string]error{
				"git rev-parse --abbrev-ref HEAD": errBoom,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			summary, err := g.GetRepoSummary(context.Background(), testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if tt.wantErr {
				return
			}
			if summary != tt.expected {
				t.Errorf("expected %+v, got %+v", tt.expected, summary)
			}
		})
	}
}

func TestGitGetBranchList(t *testing.T) {
	key := "git for-each-ref --format=%(refname:short)\t%(upstream:short)\t%(upstream:track)\t%(committerdate:unix)\t%(HEAD) refs/heads/"

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected []models.BranchInfo
		wantErr  bool
	}{
		{
			name: "mixed branches",
			canned: map[string]string{
				key: "dev\t\t\t1680000000\t \n" +
					"feature\torigin/feature\t[behind 3]\t1690000000\t \n" +
					"main\torigin/main\t[ahead 2, behind 1]\t1700000000\t*",
			},
			expected: []models.BranchInfo{
				{Name: "dev", LastCommit: time.Unix(1680000000, 0)},
				{Name: "feature", Upstream: "origin/feature", Behind: 3, LastCommit: time.Unix(1690000000, 0)},
				{Name: "main", Upstream: "origin/main", Ahead: 2, Behind: 1, LastCommit: time.Unix(1700000000, 0), IsCurrent: true},
			},
		},
		{
			name:     "empty output",
			canned:   map[string]string{key: ""},
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
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			branches, err := g.GetBranchList(context.Background(), testRepoPath)
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

func TestGitGetStashList(t *testing.T) {
	key := "git stash list --format=%(reflog:short)\t%(reflog:subject)\t%(committerdate:unix)"

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected []models.StashDetail
		wantErr  bool
	}{
		{
			name: "two stashes",
			canned: map[string]string{
				key: "stash@{0}\tWIP on main: abc msg\t1700000000\n" +
					"stash@{1}\tsaved work\t1690000000",
			},
			expected: []models.StashDetail{
				{Index: 0, Message: "WIP on main: abc msg", Date: time.Unix(1700000000, 0)},
				{Index: 1, Message: "saved work", Date: time.Unix(1690000000, 0)},
			},
		},
		{
			name:     "no stashes",
			canned:   map[string]string{key: ""},
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
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			stashes, err := g.GetStashList(context.Background(), testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if len(stashes) != len(tt.expected) {
				t.Fatalf("expected %d stashes, got %d", len(tt.expected), len(stashes))
			}
			for i, expected := range tt.expected {
				if stashes[i] != expected {
					t.Errorf("stash %d: expected %+v, got %+v", i, expected, stashes[i])
				}
			}
		})
	}
}

func TestGitGetWorktreeList(t *testing.T) {
	key := "git worktree list --porcelain"

	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected []models.WorktreeInfo
		wantErr  bool
	}{
		{
			name: "main and locked feature worktrees",
			canned: map[string]string{
				key: "worktree /repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
					"worktree /repo-feature\nHEAD def456\nbranch refs/heads/feature\nlocked",
			},
			expected: []models.WorktreeInfo{
				{Path: "/repo", Branch: "main"},
				{Path: "/repo-feature", Branch: "feature", IsLocked: true},
			},
		},
		{
			name:   "bare worktree",
			canned: map[string]string{key: "worktree /repo.git\nbare"},
			expected: []models.WorktreeInfo{
				{Path: "/repo.git", IsBare: true},
			},
		},
		{
			name:     "command failure",
			failures: map[string]error{key: errBoom},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			worktrees, err := g.GetWorktreeList(context.Background(), testRepoPath)
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

func TestGitGetCommitLog(t *testing.T) {
	key := "git log -n2 --format=%H\t%h\t%s\t%an\t%ct"

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
				key: "aaaa1111\taaaa\tfeat: add thing\tKyle King\t1700000000\n" +
					"bbbb2222\tbbbb\tfix: bug\tOther Dev\t1690000000",
			},
			expected: []models.CommitInfo{
				{Hash: "aaaa1111", ShortHash: "aaaa", Subject: "feat: add thing", Author: "Kyle King", Date: time.Unix(1700000000, 0)},
				{Hash: "bbbb2222", ShortHash: "bbbb", Subject: "fix: bug", Author: "Other Dev", Date: time.Unix(1690000000, 0)},
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
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			commits, err := g.GetCommitLog(context.Background(), testRepoPath, 2)
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

func TestGitGetLastModified(t *testing.T) {
	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected int64
		wantErr  bool
	}{
		{
			name:     "valid timestamp",
			canned:   map[string]string{"git log -1 --format=%ct": "1700000000"},
			expected: 1700000000,
		},
		{
			name:     "command failure",
			failures: map[string]error{"git log -1 --format=%ct": errBoom},
			wantErr:  true,
		},
		{
			name:    "non-numeric output",
			canned:  map[string]string{"git log -1 --format=%ct": "garbage"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			ts, err := g.GetLastModified(context.Background(), testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if ts != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, ts)
			}
		})
	}
}

func TestGitGetRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		canned   map[string]string
		failures map[string]error
		expected string
		wantErr  bool
	}{
		{
			name:     "has origin",
			canned:   map[string]string{"git remote get-url origin": "git@github.com:owner/repo.git"},
			expected: "git@github.com:owner/repo.git",
		},
		{
			name:     "no origin",
			failures: map[string]error{"git remote get-url origin": errNoSuchRemote},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			url, err := g.GetRemoteURL(context.Background(), testRepoPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if url != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, url)
			}
		})
	}
}

func TestGitFetchAllAndPruneRemote(t *testing.T) {
	tests := []struct {
		name       string
		canned     map[string]string
		failures   map[string]error
		run        func(*GitOperations) (bool, string, error)
		expectedOK bool
		expected   string
	}{
		{
			name:   "fetch success",
			canned: map[string]string{"git fetch --all --prune": ""},
			run: func(g *GitOperations) (bool, string, error) {
				return g.FetchAll(context.Background(), testRepoPath)
			},
			expectedOK: true,
			expected:   "Fetched from all remotes",
		},
		{
			name:     "fetch failure",
			failures: map[string]error{"git fetch --all --prune": errNetworkDown},
			run: func(g *GitOperations) (bool, string, error) {
				return g.FetchAll(context.Background(), testRepoPath)
			},
			expectedOK: false,
			expected:   "network down",
		},
		{
			name:   "prune success",
			canned: map[string]string{"git remote prune origin": ""},
			run: func(g *GitOperations) (bool, string, error) {
				return g.PruneRemote(context.Background(), testRepoPath)
			},
			expectedOK: true,
			expected:   "Pruned stale remote branches",
		},
		{
			name:     "prune failure",
			failures: map[string]error{"git remote prune origin": errNoRemote},
			run: func(g *GitOperations) (bool, string, error) {
				return g.PruneRemote(context.Background(), testRepoPath)
			},
			expectedOK: false,
			expected:   "no remote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			ok, msg, err := tt.run(NewGitOperations())
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

func TestGitCleanupMergedBranches(t *testing.T) {
	tests := []struct {
		name       string
		canned     map[string]string
		failures   map[string]error
		expectedOK bool
		expected   string
	}{
		{
			name: "deletes merged branches",
			canned: map[string]string{
				"git rev-parse --verify main": "abc123",
				"git branch --merged main":    "  feature\n* main\n  old-fix",
				"git branch -d feature":       "",
				"git branch -d old-fix":       "",
			},
			expectedOK: true,
			expected:   "Deleted 2 branches: feature, old-fix",
		},
		{
			name: "falls back to master",
			canned: map[string]string{
				"git rev-parse --verify master": "abc123",
				"git branch --merged master":    "* master",
			},
			failures: map[string]error{
				"git rev-parse --verify main": errUnknownRevision,
			},
			expectedOK: true,
			expected:   "No merged branches to delete",
		},
		{
			name: "neither main nor master",
			failures: map[string]error{
				"git rev-parse --verify main":   errUnknownRevision,
				"git rev-parse --verify master": errUnknownRevision,
			},
			expectedOK: false,
			expected:   "Could not find main or master branch",
		},
		{
			name: "merged listing failure",
			canned: map[string]string{
				"git rev-parse --verify main": "abc123",
			},
			failures: map[string]error{
				"git branch --merged main": errBoom,
			},
			expectedOK: false,
			expected:   "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.canned, tt.failures)

			g := NewGitOperations()
			ok, msg, err := g.CleanupMergedBranches(context.Background(), testRepoPath)
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
