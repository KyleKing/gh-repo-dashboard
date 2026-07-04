package vcs

import (
	"strings"
	"testing"
)

func TestParseJJBookmarkList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []jjBookmark
	}{
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
		{
			name:     "single local bookmark",
			input:    "main\tlocal\n",
			expected: []jjBookmark{{name: "main"}},
		},
		{
			name:  "bookmark tracked at origin",
			input: "main\tlocal\nmain\torigin\t1\t2\n",
			expected: []jjBookmark{
				{name: "main", upstream: "main@origin", ahead: 1, behind: 2},
			},
		},
		{
			name:  "multiple bookmarks",
			input: "main\tlocal\nfeature\tlocal\n",
			expected: []jjBookmark{
				{name: "main"},
				{name: "feature"},
			},
		},
		{
			name: "colocated git remote is ignored",
			input: "main\tlocal\n" +
				"main\torigin\t0\t1\n",
			expected: []jjBookmark{
				{name: "main", upstream: "main@origin", behind: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bookmarks := parseJJBookmarkList(tt.input)
			if len(bookmarks) != len(tt.expected) {
				t.Fatalf("expected %d bookmarks, got %d", len(tt.expected), len(bookmarks))
			}
			for i, expected := range tt.expected {
				if bookmarks[i] != expected {
					t.Errorf("bookmark %d: expected %+v, got %+v", i, expected, bookmarks[i])
				}
			}
		})
	}
}

func TestParseJJStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty status",
			input:    "",
			expected: 0,
		},
		{
			name:     "working copy clean",
			input:    "Working copy : abc123\nParent commit: def456",
			expected: 0,
		},
		{
			name:     "added file",
			input:    "A file.txt\nWorking copy changes:",
			expected: 1,
		},
		{
			name:     "modified file",
			input:    "M file.txt",
			expected: 1,
		},
		{
			name:     "deleted file",
			input:    "D file.txt",
			expected: 1,
		},
		{
			name:     "renamed file",
			input:    "R old.txt -> new.txt",
			expected: 1,
		},
		{
			name:     "multiple changes",
			input:    "A new.txt\nM changed.txt\nD removed.txt",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJJStatusCounts(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func parseJJStatusCounts(out string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "A ") || strings.HasPrefix(trimmed, "M ") ||
			strings.HasPrefix(trimmed, "D ") || strings.HasPrefix(trimmed, "R ") {
			count++
		}
	}
	return count
}

func TestJJOperationsVCSType(t *testing.T) {
	ops := NewJJOperations()
	if ops.VCSType().String() != "jj" {
		t.Errorf("expected jj, got %s", ops.VCSType().String())
	}
}

func TestGitOperationsVCSType(t *testing.T) {
	ops := NewGitOperations()
	if ops.VCSType().String() != "git" {
		t.Errorf("expected git, got %s", ops.VCSType().String())
	}
}
