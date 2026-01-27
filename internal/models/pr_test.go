package models

import "testing"

func TestPRInfoStatusDisplay(t *testing.T) {
	tests := []struct {
		name     string
		pr       PRInfo
		expected string
	}{
		{
			name:     "draft pr",
			pr:       PRInfo{IsDraft: true, State: "OPEN"},
			expected: "DRAFT",
		},
		{
			name:     "open pr",
			pr:       PRInfo{State: "OPEN"},
			expected: "OPEN",
		},
		{
			name:     "merged pr",
			pr:       PRInfo{State: "MERGED"},
			expected: "MERGED",
		},
		{
			name:     "closed pr",
			pr:       PRInfo{State: "CLOSED"},
			expected: "CLOSED",
		},
		{
			name:     "unknown state passed through",
			pr:       PRInfo{State: "UNKNOWN"},
			expected: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pr.StatusDisplay()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPRInfoReviewStatus(t *testing.T) {
	tests := []struct {
		name     string
		pr       PRInfo
		expected string
	}{
		{
			name:     "approved via decision",
			pr:       PRInfo{ReviewDecision: "APPROVED"},
			expected: "approved",
		},
		{
			name:     "changes requested",
			pr:       PRInfo{ReviewDecision: "CHANGES_REQUESTED"},
			expected: "changes requested",
		},
		{
			name:     "review required",
			pr:       PRInfo{ReviewDecision: "REVIEW_REQUIRED"},
			expected: "review required",
		},
		{
			name:     "approved via approvers list",
			pr:       PRInfo{ApprovedBy: []string{"user1"}},
			expected: "approved",
		},
		{
			name:     "no review info",
			pr:       PRInfo{},
			expected: "—",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pr.ReviewStatus()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestChecksStatusSummary(t *testing.T) {
	tests := []struct {
		name     string
		checks   ChecksStatus
		expected string
	}{
		{
			name:     "no checks",
			checks:   ChecksStatus{Total: 0},
			expected: "—",
		},
		{
			name:     "all passing",
			checks:   ChecksStatus{Total: 3, Passing: 3},
			expected: "passing",
		},
		{
			name:     "has failures",
			checks:   ChecksStatus{Total: 3, Passing: 2, Failing: 1},
			expected: "failing",
		},
		{
			name:     "has pending",
			checks:   ChecksStatus{Total: 3, Passing: 2, Pending: 1},
			expected: "pending",
		},
		{
			name:     "mixed (skipped)",
			checks:   ChecksStatus{Total: 3, Passing: 2, Skipped: 1},
			expected: "mixed",
		},
		{
			name:     "failing takes priority over pending",
			checks:   ChecksStatus{Total: 3, Failing: 1, Pending: 2},
			expected: "failing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checks.Summary()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWorkflowRunStatusDisplay(t *testing.T) {
	tests := []struct {
		name     string
		run      WorkflowRun
		expected string
	}{
		{
			name:     "completed shows conclusion",
			run:      WorkflowRun{Status: "completed", Conclusion: "success"},
			expected: "success",
		},
		{
			name:     "in progress shows status",
			run:      WorkflowRun{Status: "in_progress"},
			expected: "in_progress",
		},
		{
			name:     "queued shows status",
			run:      WorkflowRun{Status: "queued"},
			expected: "queued",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.run.StatusDisplay()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWorkflowSummaryStatusDisplay(t *testing.T) {
	tests := []struct {
		name     string
		summary  WorkflowSummary
		expected string
	}{
		{
			name:     "no runs",
			summary:  WorkflowSummary{Total: 0},
			expected: "—",
		},
		{
			name:     "all passing",
			summary:  WorkflowSummary{Total: 2, Passing: 2},
			expected: "passing",
		},
		{
			name:     "has failures",
			summary:  WorkflowSummary{Total: 2, Passing: 1, Failing: 1},
			expected: "failing",
		},
		{
			name:     "in progress",
			summary:  WorkflowSummary{Total: 2, Passing: 1, InProgress: 1},
			expected: "running",
		},
		{
			name:     "mixed",
			summary:  WorkflowSummary{Total: 3, Passing: 2},
			expected: "mixed",
		},
		{
			name:     "failing takes priority",
			summary:  WorkflowSummary{Total: 3, Failing: 1, InProgress: 2},
			expected: "failing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.summary.StatusDisplay()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
