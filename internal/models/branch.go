// Package models defines the domain types shared across the repo dashboard: repos,
// branches, pull requests, and the filter/sort state used to display them.
package models

import (
	"fmt"
	"strings"
	"time"
)

// emDash is the placeholder rendered for empty/unknown values.
const emDash = "—"

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

	return RelativeTime(b.LastCommit)
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
	PRInfo        *PRInfo
	WorkflowInfo  *WorkflowSummary
	ChangeID      string
	Description   string
}

// UncommittedCount returns the total number of staged, unstaged, untracked, and conflicted files.
func (b BranchDetail) UncommittedCount() int {
	return b.Staged + b.Unstaged + b.Untracked + b.Conflicted
}

// FileChangesSummary renders a human-readable summary of uncommitted file changes.
func (b BranchDetail) FileChangesSummary() string {
	parts := []string{}
	if b.Staged > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", b.Staged))
	}
	if b.Unstaged > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", b.Unstaged))
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
	return RelativeTime(c.Date)
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
	return RelativeTime(s.Date)
}

const (
	hoursPerDay  = 24
	daysPerWeek  = 7
	daysPerMonth = 30
	daysPerYear  = 365
)

// RelativeTime renders t as a human-readable duration relative to now (e.g. "3 days ago").
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return emDash
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}

		return fmt.Sprintf("%d mins ago", mins)
	case diff < hoursPerDay*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}

		return fmt.Sprintf("%d hours ago", hours)
	case diff < daysPerWeek*hoursPerDay*time.Hour:
		days := int(diff.Hours() / hoursPerDay)
		if days == 1 {
			return "1 day ago"
		}

		return fmt.Sprintf("%d days ago", days)
	case diff < daysPerMonth*hoursPerDay*time.Hour:
		weeks := int(diff.Hours() / hoursPerDay / daysPerWeek)
		if weeks == 1 {
			return "1 week ago"
		}

		return fmt.Sprintf("%d weeks ago", weeks)
	case diff < daysPerYear*hoursPerDay*time.Hour:
		months := int(diff.Hours() / hoursPerDay / daysPerMonth)
		if months == 1 {
			return "1 month ago"
		}

		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff.Hours() / hoursPerDay / daysPerYear)
		if years == 1 {
			return "1 year ago"
		}

		return fmt.Sprintf("%d years ago", years)
	}
}
