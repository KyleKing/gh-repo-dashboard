// Package models defines the domain types shared across the repo dashboard: repos,
// branches, pull requests, and the filter/sort state used to display them.
package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/gh-repo-dashboard/internal/ui"
)

const emDash = "—"

// emDash is the placeholder rendered for empty/unknown values.

// DefaultBranchNames are the conventional primary branch names, assumed
// wherever a repo's real default branch has not been resolved from its remote.
var DefaultBranchNames = []string{"main", "master", "trunk"}

// DetachedBranchLabel formats a detached HEAD's short commit as the branch
// label shown wherever a branch name would go.
func DetachedBranchLabel(shortHash string) string {
	return "(" + shortHash + ")"
}

// IsDetachedBranch reports whether label came from DetachedBranchLabel, so
// views can say "detached" rather than treating the commit as a branch name.
func IsDetachedBranch(label string) bool {
	return len(label) > 2 && strings.HasPrefix(label, "(") && strings.HasSuffix(label, ")")
}

// IsDefaultBranchName reports whether name is one of DefaultBranchNames.
func IsDefaultBranchName(name string) bool {
	for _, candidate := range DefaultBranchNames {
		if name == candidate {
			return true
		}
	}

	return false
}

// BranchInfo summarizes a single branch's tracking state.
type BranchInfo struct {
	Name       string
	Upstream   string
	Ahead      int
	Behind     int
	LastCommit time.Time
	IsCurrent  bool
	IsRemote   bool
	// Head is the branch tip's commit OID (git) or change target commit id
	// (jj), used to detect squash-merged branches whose tip matches a merged
	// pull request's head OID even though the branch itself was never merged.
	Head string
}

// RelativeLastCommit returns a human-readable relative time for the branch's last commit.
func (b BranchInfo) RelativeLastCommit() string {
	if b.LastCommit.IsZero() {
		return emDash
	}

	return ui.RelativeTime(b.LastCommit)
}

// BranchDetail holds the full detail view state for a single branch.
// DefaultBranch is empty when no default-branch comparison is available;
// DefaultAhead/DefaultBehind are only meaningful when it is set.
type BranchDetail struct {
	Branch        BranchInfo
	Commits       []CommitInfo
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
func ModifiedFilesLabel(vcsType VCSType) string {
	if vcsType == VCSTypeJJ {
		return "changed"
	}

	return "unstaged"
}

// FileChangesSummary renders a human-readable summary of uncommitted file
// changes, wording them for vcsType.
func (b BranchDetail) FileChangesSummary(vcsType VCSType) string {
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

// CommitInfo summarizes a single commit for display.
type CommitInfo struct {
	Hash      string
	ShortHash string
	Subject   string
	Author    string
	Date      time.Time
}

// RelativeDate returns a human-readable relative time for the commit's date.
func (c CommitInfo) RelativeDate() string {
	return ui.RelativeTime(c.Date)
}

// StashDetail summarizes a single stash entry.
type StashDetail struct {
	Index   int
	Message string
	Branch  string
	Date    time.Time
}

// RelativeDate returns a human-readable relative time for the stash's date.
func (s StashDetail) RelativeDate() string {
	return ui.RelativeTime(s.Date)
}
