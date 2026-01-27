package models

import (
	"testing"
	"time"
)

func TestRepoSummaryName(t *testing.T) {
	s := RepoSummary{Path: "/home/user/projects/my-repo"}
	if s.Name() != "my-repo" {
		t.Errorf("expected 'my-repo', got '%s'", s.Name())
	}
}

func TestRepoSummaryUncommittedCount(t *testing.T) {
	s := RepoSummary{
		Staged:     2,
		Unstaged:   3,
		Untracked:  1,
		Conflicted: 0,
	}
	if s.UncommittedCount() != 6 {
		t.Errorf("expected 6, got %d", s.UncommittedCount())
	}
}

func TestRepoSummaryIsDirty(t *testing.T) {
	tests := []struct {
		name     string
		summary  RepoSummary
		expected bool
	}{
		{
			name:     "clean repo",
			summary:  RepoSummary{},
			expected: false,
		},
		{
			name:     "has staged",
			summary:  RepoSummary{Staged: 1},
			expected: true,
		},
		{
			name:     "has unstaged",
			summary:  RepoSummary{Unstaged: 1},
			expected: true,
		},
		{
			name:     "has untracked",
			summary:  RepoSummary{Untracked: 1},
			expected: true,
		},
		{
			name:     "has ahead",
			summary:  RepoSummary{Ahead: 1},
			expected: true,
		},
		{
			name:     "only behind is not dirty",
			summary:  RepoSummary{Behind: 1},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.summary.IsDirty() != tt.expected {
				t.Errorf("expected IsDirty() = %v, got %v", tt.expected, tt.summary.IsDirty())
			}
		})
	}
}

func TestRepoSummaryStatus(t *testing.T) {
	tests := []struct {
		name     string
		summary  RepoSummary
		expected RepoStatus
	}{
		{
			name:     "clean",
			summary:  RepoSummary{},
			expected: RepoStatusClean,
		},
		{
			name:     "dirty",
			summary:  RepoSummary{Unstaged: 1},
			expected: RepoStatusDirty,
		},
		{
			name:     "ahead",
			summary:  RepoSummary{Ahead: 1},
			expected: RepoStatusAhead,
		},
		{
			name:     "behind",
			summary:  RepoSummary{Behind: 1},
			expected: RepoStatusBehind,
		},
		{
			name:     "diverged",
			summary:  RepoSummary{Ahead: 1, Behind: 1},
			expected: RepoStatusDiverged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.summary.Status() != tt.expected {
				t.Errorf("expected Status() = %v, got %v", tt.expected, tt.summary.Status())
			}
		})
	}
}

func TestRepoSummaryStatusSummary(t *testing.T) {
	tests := []struct {
		name     string
		summary  RepoSummary
		expected string
	}{
		{
			name:     "clean",
			summary:  RepoSummary{},
			expected: "✓",
		},
		{
			name:     "staged only",
			summary:  RepoSummary{Staged: 2},
			expected: "+2",
		},
		{
			name:     "unstaged only",
			summary:  RepoSummary{Unstaged: 3},
			expected: "~3",
		},
		{
			name:     "untracked only",
			summary:  RepoSummary{Untracked: 1},
			expected: "?1",
		},
		{
			name:     "ahead only",
			summary:  RepoSummary{Ahead: 5},
			expected: "↑5",
		},
		{
			name:     "behind only",
			summary:  RepoSummary{Behind: 3},
			expected: "↓3",
		},
		{
			name:     "mixed",
			summary:  RepoSummary{Staged: 1, Unstaged: 2, Ahead: 3},
			expected: "+1 ~2 ↑3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.summary.StatusSummary() != tt.expected {
				t.Errorf("expected StatusSummary() = '%s', got '%s'", tt.expected, tt.summary.StatusSummary())
			}
		})
	}
}

func TestRepoSummaryRelativeModified(t *testing.T) {
	s := RepoSummary{}
	if s.RelativeModified() != "—" {
		t.Errorf("expected '—' for zero time, got '%s'", s.RelativeModified())
	}

	s.LastModified = time.Now()
	if s.RelativeModified() == "—" {
		t.Error("expected non-empty relative time")
	}
}
