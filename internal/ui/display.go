// Package ui renders shared models as terminal and CLI text. Anything that
// emits a glyph, a placeholder, or a human-readable duration belongs here
// rather than in the data packages it reads.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

const (
	emDash = "—"

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

// ActivitySummary renders the latest activity as an age and an author, or
// emDash when the pull request has neither comments nor reviews.
func PRActivitySummary(p *forge.PullRequest) string {
	if p.Activity == nil || p.Activity.At.IsZero() {
		return emDash
	}

	return RelativeTime(p.Activity.At) + " " + p.Activity.Author
}

// ReviewGlyph marks an approval or a change request, or is empty otherwise.
func PRReviewGlyph(p *forge.PullRequest) string {
	switch p.ReviewDecision {
	case "APPROVED":
		return "✓"
	case "CHANGES_REQUESTED":
		return "✗"
	default:
		return ""
	}
}

// StatusDisplay returns the pull request's display status label.
func PRStatusDisplay(p forge.PullRequest) string {
	if p.IsDraft {
		return "DRAFT"
	}
	switch p.State {
	case "OPEN":
		return "OPEN"
	case forge.PRStatusMerged:
		return forge.PRStatusMerged
	case forge.PRStatusClosed:
		return forge.PRStatusClosed
	default:
		return p.State
	}
}

// ReviewStatus returns a human-readable summary of the pull request's review decision.
func PRReviewStatus(p forge.PullRequest) string {
	return reviewStatus(p.ReviewDecision, p.ApprovedBy)
}

// Summary returns a one-word overall status for the checks.
func ChecksSummary(c forge.ChecksStatus) string {
	if c.Total == 0 {
		return emDash
	}
	if c.Failing > 0 {
		return forge.StatusFailing
	}
	if c.Pending > 0 {
		return "pending"
	}
	if c.Passing == c.Total {
		return forge.StatusPassing
	}

	return "mixed"
}

// StatusDisplay returns the check's lowercased conclusion once it has
// completed, or its in-flight status ("queued", "in progress") before then.
func CheckStatusDisplay(c forge.CheckDetail) string {
	status := strings.ToLower(c.Status)
	if status == forge.StatusCompleted && c.Conclusion != "" {
		return strings.ToLower(c.Conclusion)
	}
	if status == "" {
		return emDash
	}

	return strings.ReplaceAll(status, "_", " ")
}

// Duration renders how long the check ran, or emDash while it's still running.
func CheckDuration(c forge.CheckDetail) string {
	if c.StartedAt.IsZero() || c.CompletedAt.IsZero() {
		return emDash
	}

	elapsed := c.CompletedAt.Sub(c.StartedAt).Round(time.Second)
	if elapsed < 0 {
		return emDash
	}

	return elapsed.String()
}

// RelativeCreated returns a human-readable relative time for the comment.
func CommentRelativeCreated(c forge.PRComment) string {
	return RelativeTime(c.CreatedAt)
}

// RelativeCreated returns a human-readable relative time for the pull request's creation.
func PRDetailRelativeCreated(p forge.PRDetail) string {
	return RelativeTime(p.CreatedAt)
}

// RelativeUpdated returns a human-readable relative time for the pull request's last update.
func PRDetailRelativeUpdated(p forge.PRDetail) string {
	return RelativeTime(p.UpdatedAt)
}

// StatusDisplay returns the workflow run's display status label.
func WorkflowRunStatusDisplay(w forge.WorkflowRun) string {
	if w.Status == forge.StatusCompleted {
		return w.Conclusion
	}

	return w.Status
}

// Conclusion rolls the workflows up into one word: failing if any failed,
// pending while any is still going, passing when all succeeded, and emDash
// when the commit has no runs at all.
func CIConclusion(c *forge.DefaultBranchCI) string {
	if len(c.Workflows) == 0 {
		return emDash
	}

	pending := false
	for i := range c.Workflows {
		switch {
		case c.Workflows[i].Conclusion == "failure":
			return forge.StatusFailing
		case c.Workflows[i].Status != forge.StatusCompleted:
			pending = true
		}
	}

	if pending {
		return "pending"
	}

	return forge.StatusPassing
}

// StatusDisplay returns a one-word overall status for the workflow runs.
func WorkflowSummaryStatusDisplay(w forge.WorkflowSummary) string {
	if w.Total == 0 {
		return emDash
	}
	if w.Failing > 0 {
		return forge.StatusFailing
	}
	if w.InProgress > 0 {
		return "running"
	}
	if w.Passing == w.Total {
		return forge.StatusPassing
	}

	return "mixed"
}

// ReviewStatus reports the preview's review state in the same vocabulary the
// full detail uses.
func PreviewReviewStatus(p forge.PRPreview) string {
	return reviewStatus(p.ReviewDecision, nil)
}

func reviewStatus(decision string, approvedBy []string) string {
	switch decision {
	case "APPROVED":
		return forge.ReviewApproved
	case "CHANGES_REQUESTED":
		return forge.ReviewChangesRequested
	case "REVIEW_REQUIRED":
		return "review required"
	default:
		if len(approvedBy) > 0 {
			return forge.ReviewApproved
		}

		return emDash
	}
}

// RepoStatusSummary renders a compact symbol-based summary of the repo's
// working tree state.
func RepoStatusSummary(r vcs.RepoSummary) string {
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

	return strings.Join(parts, " ")
}

// RepoRelativeModified returns a human-readable relative time for the repo's
// last modification.
func RepoRelativeModified(r vcs.RepoSummary) string {
	if r.LastModified.IsZero() {
		return emDash
	}

	return RelativeTime(r.LastModified)
}

// BranchRelativeLastCommit returns a human-readable relative time for the
// branch's last commit.
func BranchRelativeLastCommit(b vcs.BranchInfo) string {
	if b.LastCommit.IsZero() {
		return emDash
	}

	return RelativeTime(b.LastCommit)
}

// CommitRelativeDate returns a human-readable relative time for the commit's date.
func CommitRelativeDate(c vcs.CommitInfo) string {
	return RelativeTime(c.Date)
}

// StashRelativeDate returns a human-readable relative time for the stash's date.
func StashRelativeDate(s vcs.StashDetail) string {
	return RelativeTime(s.Date)
}
