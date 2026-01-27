package vcs

import (
	"strings"
	"testing"
)

func TestCountNonEmptyLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "single line",
			input:    "abc123",
			expected: 1,
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: 3,
		},
		{
			name:     "lines with whitespace only",
			input:    "line1\n   \nline2\n\t\n",
			expected: 2,
		},
		{
			name:     "empty lines",
			input:    "line1\n\n\nline2\n",
			expected: 2,
		},
		{
			name:     "only whitespace",
			input:    "   \n\t\n  \n",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countNonEmptyLines(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestParseJJBookmarkList(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedCount   int
		hasOriginRemote bool
	}{
		{
			name:          "empty",
			input:         "",
			expectedCount: 0,
		},
		{
			name:          "single local bookmark",
			input:         "main: abcd1234",
			expectedCount: 1,
		},
		{
			name:            "bookmark with tracking",
			input:           "main@origin: abcd1234",
			expectedCount:   1,
			hasOriginRemote: true,
		},
		{
			name:          "multiple bookmarks",
			input:         "main: abc\nfeature: def\ndevelop: ghi",
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bookmarks := parseJJBookmarkOutput(tt.input)
			if len(bookmarks) != tt.expectedCount {
				t.Errorf("expected %d bookmarks, got %d", tt.expectedCount, len(bookmarks))
			}
			if tt.hasOriginRemote && len(bookmarks) > 0 {
				found := false
				for _, b := range bookmarks {
					if strings.Contains(b, "@origin") {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected to find @origin tracking")
				}
			}
		})
	}
}

func parseJJBookmarkOutput(out string) []string {
	if out == "" {
		return nil
	}

	var bookmarks []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		bookmarks = append(bookmarks, line)
	}
	return bookmarks
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

func TestParseJJWorkspaceList(t *testing.T) {
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
			name:     "single workspace",
			input:    "default@abc123: /path/to/repo",
			expected: 1,
		},
		{
			name:     "multiple workspaces",
			input:    "default@abc: /main\nfeature@def: /feature",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJJWorkspaceOutput(tt.input)
			if len(result) != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, len(result))
			}
		})
	}
}

func parseJJWorkspaceOutput(out string) []string {
	if out == "" {
		return nil
	}

	var workspaces []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		workspaces = append(workspaces, line)
	}
	return workspaces
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
