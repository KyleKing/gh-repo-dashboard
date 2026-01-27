package models

import (
	"fmt"
	"path/filepath"
	"time"
)

type RepoSummary struct {
	Path          string
	VCSType       VCSType
	Branch        string
	Upstream      string
	Ahead         int
	Behind        int
	Staged        int
	Unstaged      int
	Untracked     int
	Conflicted    int
	StashCount    int
	LastModified  time.Time
	PRInfo        *PRInfo
	WorkflowInfo  *WorkflowSummary
	Loading       bool
	Error         error
}

func (r RepoSummary) Name() string {
	return filepath.Base(r.Path)
}

func (r RepoSummary) UncommittedCount() int {
	return r.Staged + r.Unstaged + r.Untracked + r.Conflicted
}

func (r RepoSummary) IsDirty() bool {
	return r.UncommittedCount() > 0 || r.Ahead > 0
}

func (r RepoSummary) Status() RepoStatus {
	if r.Ahead > 0 && r.Behind > 0 {
		return RepoStatusDiverged
	}
	if r.Ahead > 0 {
		return RepoStatusAhead
	}
	if r.Behind > 0 {
		return RepoStatusBehind
	}
	if r.UncommittedCount() > 0 {
		return RepoStatusDirty
	}
	return RepoStatusClean
}

func (r RepoSummary) StatusSummary() string {
	parts := []string{}

	if r.Staged > 0 {
		parts = append(parts, fmt.Sprintf("+%d", r.Staged))
	}
	if r.Unstaged > 0 {
		parts = append(parts, fmt.Sprintf("~%d", r.Unstaged))
	}
	if r.Untracked > 0 {
		parts = append(parts, fmt.Sprintf("?%d", r.Untracked))
	}
	if r.Conflicted > 0 {
		parts = append(parts, fmt.Sprintf("!%d", r.Conflicted))
	}
	if r.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", r.Ahead))
	}
	if r.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", r.Behind))
	}

	if len(parts) == 0 {
		return "✓"
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

func (r RepoSummary) RelativeModified() string {
	if r.LastModified.IsZero() {
		return "—"
	}
	return RelativeTime(r.LastModified)
}

type WorktreeInfo struct {
	Path     string
	Branch   string
	IsBare   bool
	IsLocked bool
}
