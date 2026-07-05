package models

import "time"

// Display values shared by PRInfo/ChecksStatus/WorkflowSummary and the views
// that render them, so both sides compare against the same constant.
const (
	PRStatusMerged         = "MERGED"
	PRStatusClosed         = "CLOSED"
	ReviewApproved         = "approved"
	ReviewChangesRequested = "changes requested"
	StatusFailing          = "failing"
)

// PRInfo summarizes a pull request for the repo list and detail views.
type PRInfo struct {
	Number          int
	Title           string
	State           string
	URL             string
	IsDraft         bool
	Mergeable       string
	HeadRef         string
	BaseRef         string
	Checks          ChecksStatus
	ReviewDecision  string
	ApprovedBy      []string
	ChangesRequests int
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

		return "—"
	}
}

// ChecksStatus tallies a pull request's CI check outcomes.
type ChecksStatus struct {
	Total   int
	Passing int
	Failing int
	Pending int
	Skipped int
}

// Summary returns a one-word overall status for the checks.
func (c ChecksStatus) Summary() string {
	if c.Total == 0 {
		return "—"
	}
	if c.Failing > 0 {
		return StatusFailing
	}
	if c.Pending > 0 {
		return "pending"
	}
	if c.Passing == c.Total {
		return "passing"
	}

	return "mixed"
}

// PRDetail holds the full detail view state for a single pull request.
type PRDetail struct {
	PRInfo
	Body       string
	Author     string
	Assignees  []string
	Reviewers  []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Additions  int
	Deletions  int
	Comments   int
	ReviewsURL string
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
		return "—"
	}
	if w.Failing > 0 {
		return StatusFailing
	}
	if w.InProgress > 0 {
		return "running"
	}
	if w.Passing == w.Total {
		return "passing"
	}

	return "mixed"
}
