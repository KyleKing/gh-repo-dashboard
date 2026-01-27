package vcs

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestExtractRepoPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ssh url",
			input:    "git@github.com:owner/repo.git",
			expected: "owner/repo",
		},
		{
			name:     "ssh url without .git",
			input:    "git@github.com:owner/repo",
			expected: "owner/repo",
		},
		{
			name:     "https url",
			input:    "https://github.com/owner/repo.git",
			expected: "owner/repo",
		},
		{
			name:     "https url without .git",
			input:    "https://github.com/owner/repo",
			expected: "owner/repo",
		},
		{
			name:     "http url",
			input:    "http://github.com/owner/repo.git",
			expected: "owner/repo",
		},
		{
			name:     "gitlab ssh",
			input:    "git@gitlab.com:group/subgroup/repo.git",
			expected: "subgroup/repo",
		},
		{
			name:     "short path returns empty",
			input:    "invalid",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractRepoPath(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestParseStatusCounts(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		staged     int
		unstaged   int
		untracked  int
		conflicted int
	}{
		{
			name:       "empty status",
			input:      "",
			staged:     0,
			unstaged:   0,
			untracked:  0,
			conflicted: 0,
		},
		{
			name:      "staged file",
			input:     "M  file.txt\x00",
			staged:    1,
			unstaged:  0,
			untracked: 0,
		},
		{
			name:      "unstaged file",
			input:     " M file.txt\x00",
			staged:    0,
			unstaged:  1,
			untracked: 0,
		},
		{
			name:      "untracked file",
			input:     "?? file.txt\x00",
			staged:    0,
			unstaged:  0,
			untracked: 1,
		},
		{
			name:       "conflicted UU",
			input:      "UU file.txt\x00",
			conflicted: 1,
		},
		{
			name:       "conflicted DD",
			input:      "DD file.txt\x00",
			conflicted: 1,
		},
		{
			name:       "conflicted AA",
			input:      "AA file.txt\x00",
			conflicted: 1,
		},
		{
			name:      "mixed status",
			input:     "M  staged.txt\x00 M unstaged.txt\x00?? new.txt\x00",
			staged:    1,
			unstaged:  1,
			untracked: 1,
		},
		{
			name:     "added file",
			input:    "A  new.txt\x00",
			staged:   1,
			unstaged: 0,
		},
		{
			name:     "deleted file staged",
			input:    "D  old.txt\x00",
			staged:   1,
			unstaged: 0,
		},
		{
			name:     "renamed file",
			input:    "R  old.txt -> new.txt\x00",
			staged:   1,
			unstaged: 0,
		},
		{
			name:     "modified both staged and unstaged",
			input:    "MM file.txt\x00",
			staged:   1,
			unstaged: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			staged, unstaged, untracked, conflicted := parseStatusOutput(tt.input)
			if staged != tt.staged {
				t.Errorf("staged: expected %d, got %d", tt.staged, staged)
			}
			if unstaged != tt.unstaged {
				t.Errorf("unstaged: expected %d, got %d", tt.unstaged, unstaged)
			}
			if untracked != tt.untracked {
				t.Errorf("untracked: expected %d, got %d", tt.untracked, untracked)
			}
			if conflicted != tt.conflicted {
				t.Errorf("conflicted: expected %d, got %d", tt.conflicted, conflicted)
			}
		})
	}
}

func parseStatusOutput(out string) (staged, unstaged, untracked, conflicted int) {
	if out == "" {
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

func TestParseBranchTrackingInfo(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		ahead  int
		behind int
	}{
		{
			name:   "ahead only",
			input:  "[ahead 3]",
			ahead:  3,
			behind: 0,
		},
		{
			name:   "behind only",
			input:  "[behind 5]",
			ahead:  0,
			behind: 5,
		},
		{
			name:   "ahead and behind",
			input:  "[ahead 2, behind 4]",
			ahead:  2,
			behind: 4,
		},
		{
			name:   "no tracking info",
			input:  "",
			ahead:  0,
			behind: 0,
		},
		{
			name:   "gone tracking",
			input:  "[gone]",
			ahead:  0,
			behind: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ahead, behind := parseBranchTracking(tt.input)
			if ahead != tt.ahead {
				t.Errorf("ahead: expected %d, got %d", tt.ahead, ahead)
			}
			if behind != tt.behind {
				t.Errorf("behind: expected %d, got %d", tt.behind, behind)
			}
		})
	}
}

func parseBranchTracking(s string) (ahead, behind int) {
	if s == "" || s == "[gone]" {
		return 0, 0
	}

	trackRe := regexp.MustCompile(`\[ahead (\d+)(?:, behind (\d+))?\]|\[behind (\d+)\]`)
	matches := trackRe.FindStringSubmatch(s)
	if matches == nil {
		return 0, 0
	}

	if matches[1] != "" {
		ahead, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		behind, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		behind, _ = strconv.Atoi(matches[3])
	}
	return
}

func TestParseWorktreePorcelain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []worktreeResult
	}{
		{
			name:     "empty output",
			input:    "",
			expected: nil,
		},
		{
			name: "single worktree",
			input: `worktree /path/to/repo
branch refs/heads/main
`,
			expected: []worktreeResult{
				{Path: "/path/to/repo", Branch: "main"},
			},
		},
		{
			name: "bare worktree",
			input: `worktree /path/to/repo.git
bare
`,
			expected: []worktreeResult{
				{Path: "/path/to/repo.git", IsBare: true},
			},
		},
		{
			name: "locked worktree",
			input: `worktree /path/to/repo
branch refs/heads/feature
locked
`,
			expected: []worktreeResult{
				{Path: "/path/to/repo", Branch: "feature", IsLocked: true},
			},
		},
		{
			name: "multiple worktrees",
			input: `worktree /main
branch refs/heads/main

worktree /feature
branch refs/heads/feature
`,
			expected: []worktreeResult{
				{Path: "/main", Branch: "main"},
				{Path: "/feature", Branch: "feature"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseWorktreePorcelain(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d worktrees, got %d", len(tt.expected), len(result))
				return
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("worktree %d: expected %+v, got %+v", i, expected, result[i])
				}
			}
		})
	}
}

type worktreeResult struct {
	Path     string
	Branch   string
	IsBare   bool
	IsLocked bool
}

func parseWorktreePorcelain(out string) []worktreeResult {
	if out == "" {
		return nil
	}

	var worktrees []worktreeResult
	var current worktreeResult

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				worktrees = append(worktrees, current)
			}
			current = worktreeResult{Path: strings.TrimPrefix(line, "worktree ")}
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

	return worktrees
}

func TestParseStashList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty",
			input:    "",
			expected: 0,
		},
		{
			name:     "one stash",
			input:    "stash@{0}\tWIP on main: abc123\t1234567890",
			expected: 1,
		},
		{
			name:     "multiple stashes",
			input:    "stash@{0}\tWIP\t123\nstash@{1}\tSave\t456\nstash@{2}\tTest\t789",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countStashEntries(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func countStashEntries(out string) int {
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func TestParseRevListOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		ahead  int
		behind int
	}{
		{
			name:   "normal output",
			input:  "3\t2",
			ahead:  3,
			behind: 2,
		},
		{
			name:   "ahead only",
			input:  "5\t0",
			ahead:  5,
			behind: 0,
		},
		{
			name:   "behind only",
			input:  "0\t7",
			ahead:  0,
			behind: 7,
		},
		{
			name:   "no difference",
			input:  "0\t0",
			ahead:  0,
			behind: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ahead, behind := parseRevListOutput(tt.input)
			if ahead != tt.ahead {
				t.Errorf("ahead: expected %d, got %d", tt.ahead, ahead)
			}
			if behind != tt.behind {
				t.Errorf("behind: expected %d, got %d", tt.behind, behind)
			}
		})
	}
}

func parseRevListOutput(out string) (ahead, behind int) {
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0
	}
	ahead, _ = strconv.Atoi(parts[0])
	behind, _ = strconv.Atoi(parts[1])
	return
}
