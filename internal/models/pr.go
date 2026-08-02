package models

import (
	"strings"
	"time"
)

// Display values shared by PRInfo/ChecksStatus/WorkflowSummary and the views
// that render them, so both sides compare against the same constant.
const (
	PRStatusMerged         = "MERGED"
	PRStatusClosed         = "CLOSED"
	ReviewApproved         = "approved"
	ReviewChangesRequested = "changes requested"
	StatusFailing          = "failing"
	StatusPassing          = "passing"
)

// PRInfo summarizes a pull request for the repo list and detail views.
type PRInfo struct {
	Number          int          `json:"number"`
	Title           string       `json:"title"`
	State           string       `json:"state"`
	URL             string       `json:"url"`
	IsDraft         bool         `json:"is_draft"`
	Mergeable       string       `json:"mergeable,omitempty"`
	HeadRef         string       `json:"head_ref"`
	BaseRef         string       `json:"base_ref"`
	Checks          ChecksStatus `json:"checks"`
	ReviewDecision  string       `json:"review_decision,omitempty"`
	ApprovedBy      []string     `json:"approved_by,omitempty"`
	ChangesRequests int          `json:"changes_requests,omitempty"`
}

// StatusDisplay returns the pull request's display status label.
func (p PRInfo) StatusDisplay() string {
	if p.IsDraft {
		return "DRAFT"
	}
	switch p.State {
	case "OPEN":
		return "OPEN"
	case PRStatusMerged:
		return PRStatusMerged
	case PRStatusClosed:
		return PRStatusClosed
	default:
		return p.State
	}
}

// ReviewStatus returns a human-readable summary of the pull request's review decision.
func (p PRInfo) ReviewStatus() string {
	switch p.ReviewDecision {
	case "APPROVED":
		return ReviewApproved
	case "CHANGES_REQUESTED":
		return ReviewChangesRequested
	case "REVIEW_REQUIRED":
		return "review required"
	default:
		if len(p.ApprovedBy) > 0 {
			return ReviewApproved
		}

		return emDash
	}
}

// ChecksStatus tallies a pull request's CI check outcomes.
type ChecksStatus struct {
	Total   int `json:"total"`
	Passing int `json:"passing"`
	Failing int `json:"failing"`
	Pending int `json:"pending"`
	Skipped int `json:"skipped"`
}

// Summary returns a one-word overall status for the checks.
func (c ChecksStatus) Summary() string {
	if c.Total == 0 {
		return emDash
	}
	if c.Failing > 0 {
		return StatusFailing
	}
	if c.Pending > 0 {
		return "pending"
	}
	if c.Passing == c.Total {
		return StatusPassing
	}

	return "mixed"
}

// CheckDetail is a single CI check on a pull request.
type CheckDetail struct {
	Name        string
	Workflow    string
	Status      string
	Conclusion  string
	StartedAt   time.Time
	CompletedAt time.Time
}

// StatusDisplay returns the check's lowercased conclusion once it has
// completed, or its in-flight status ("queued", "in progress") before then.
func (c CheckDetail) StatusDisplay() string {
	status := strings.ToLower(c.Status)
	if status == "completed" && c.Conclusion != "" {
		return strings.ToLower(c.Conclusion)
	}
	if status == "" {
		return emDash
	}

	return strings.ReplaceAll(status, "_", " ")
}

// Duration renders how long the check ran, or emDash while it's still running.
func (c CheckDetail) Duration() string {
	if c.StartedAt.IsZero() || c.CompletedAt.IsZero() {
		return emDash
	}

	elapsed := c.CompletedAt.Sub(c.StartedAt).Round(time.Second)
	if elapsed < 0 {
		return emDash
	}

	return elapsed.String()
}

// PRComment is a single issue comment on a pull request.
type PRComment struct {
	Author    string
	Body      string
	CreatedAt time.Time
}

// RelativeCreated returns a human-readable relative time for the comment.
func (c PRComment) RelativeCreated() string {
	return RelativeTime(c.CreatedAt)
}

// PRDetail holds the full detail view state for a single pull request.
type PRDetail struct {
	PRInfo
	Body      string
	Author    string
	Assignees []string
	Reviewers []string
	CreatedAt time.Time
	UpdatedAt time.Time
	Additions int
	Deletions int
	// Comments counts issue comments on the pull request; LatestComment is
	// the most recent of them, nil when there are none.
	Comments      int
	LatestComment *PRComment
	CheckDetails  []CheckDetail
	ReviewsURL    string
}

// RelativeCreated returns a human-readable relative time for the pull request's creation.
func (p PRDetail) RelativeCreated() string {
	return RelativeTime(p.CreatedAt)
}

// RelativeUpdated returns a human-readable relative time for the pull request's last update.
func (p PRDetail) RelativeUpdated() string {
	return RelativeTime(p.UpdatedAt)
}

// WorkflowRun summarizes a single CI workflow run.
type WorkflowRun struct {
	ID         int64
	Name       string
	Status     string
	Conclusion string
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// StatusDisplay returns the workflow run's display status label.
func (w WorkflowRun) StatusDisplay() string {
	if w.Status == "completed" {
		return w.Conclusion
	}

	return w.Status
}

// WorkflowSummary aggregates the CI workflow runs for a commit.
type WorkflowSummary struct {
	Runs       []WorkflowRun
	Total      int
	Passing    int
	Failing    int
	InProgress int
}

// StatusDisplay returns a one-word overall status for the workflow runs.
func (w WorkflowSummary) StatusDisplay() string {
	if w.Total == 0 {
		return emDash
	}
	if w.Failing > 0 {
		return StatusFailing
	}
	if w.InProgress > 0 {
		return "running"
	}
	if w.Passing == w.Total {
		return StatusPassing
	}

	return "mixed"
}
