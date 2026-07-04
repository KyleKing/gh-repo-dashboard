package models

import (
	"fmt"
	"strings"
	"time"
)

type BranchInfo struct {
	Name       string
	Upstream   string
	Ahead      int
	Behind     int
	LastCommit time.Time
	IsCurrent  bool
	IsRemote   bool
}

func (b BranchInfo) RelativeLastCommit() string {
	if b.LastCommit.IsZero() {
		return "—"
	}

	return RelativeTime(b.LastCommit)
}

type BranchDetail struct {
	Branch       BranchInfo
	Commits      []CommitInfo
	Staged       int
	Unstaged     int
	Untracked    int
	Conflicted   int
	PRInfo       *PRInfo
	WorkflowInfo *WorkflowSummary
	ChangeID     string
	Description  string
}

func (b BranchDetail) UncommittedCount() int {
	return b.Staged + b.Unstaged + b.Untracked + b.Conflicted
}

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

type CommitInfo struct {
	Hash      string
	ShortHash string
	Subject   string
	Author    string
	Date      time.Time
}

func (c CommitInfo) RelativeDate() string {
	return RelativeTime(c.Date)
}

type StashDetail struct {
	Index   int
	Message string
	Branch  string
	Date    time.Time
}

func (s StashDetail) RelativeDate() string {
	return RelativeTime(s.Date)
}

const (
	hoursPerDay  = 24
	daysPerWeek  = 7
	daysPerMonth = 30
	daysPerYear  = 365
)

func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "—"
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
