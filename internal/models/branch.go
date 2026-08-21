// Package models defines the domain types shared across the repo dashboard: repos,
// branches, pull requests, and the filter/sort state used to display them.
package models

import (
	"fmt"
	"strings"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

// BranchDetail holds the full detail view state for a single branch.
// DefaultBranch is empty when no default-branch comparison is available;
// DefaultAhead/DefaultBehind are only meaningful when it is set.
type BranchDetail struct {
	Branch        vcs.BranchInfo
	Commits       []vcs.CommitInfo
	DefaultBranch string
	DefaultAhead  int
	DefaultBehind int
	Staged        int
	Unstaged      int
	Untracked     int
	Conflicted    int
	PRInfo        *forge.PullRequest
	WorkflowInfo  *forge.WorkflowSummary
	ChangeID      string
	Description   string
}

// UncommittedCount returns the total number of staged, unstaged, untracked, and conflicted files.
func (b BranchDetail) UncommittedCount() int {
	return b.Staged + b.Unstaged + b.Untracked + b.Conflicted
}

// ModifiedFilesLabel names tracked files changed but not committed. Since jj
// has no staging area, calling them "unstaged" there describes a distinction
// the VCS does not make.
func ModifiedFilesLabel(vcsType vcs.Type) string {
	if vcsType == vcs.TypeJJ {
		return "changed"
	}

	return "unstaged"
}

// FileChangesSummary renders a human-readable summary of uncommitted file
// changes, wording them for vcsType.
func (b BranchDetail) FileChangesSummary(vcsType vcs.Type) string {
	parts := []string{}
	if b.Staged > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", b.Staged))
	}
	if b.Unstaged > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", b.Unstaged, ModifiedFilesLabel(vcsType)))
	}
	if b.Untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", b.Untracked))
	}
	if b.Conflicted > 0 {
		parts = append(parts, fmt.Sprintf("%d conflicted", b.Conflicted))
	}
	if len(parts) == 0 {
		return "No uncommitted changes"
	}

	return strings.Join(parts, ", ")
}
