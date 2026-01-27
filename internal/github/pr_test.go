package github

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestParseChecks(t *testing.T) {
	tests := []struct {
		name     string
		input    []statusCheck
		expected models.ChecksStatus
	}{
		{
			name:  "empty checks",
			input: nil,
			expected: models.ChecksStatus{
				Total: 0,
			},
		},
		{
			name: "all passing",
			input: []statusCheck{
				{Conclusion: "success"},
				{Conclusion: "success"},
			},
			expected: models.ChecksStatus{
				Total:   2,
				Passing: 2,
			},
		},
		{
			name: "all failing",
			input: []statusCheck{
				{Conclusion: "failure"},
				{Conclusion: "error"},
			},
			expected: models.ChecksStatus{
				Total:   2,
				Failing: 2,
			},
		},
		{
			name: "pending checks",
			input: []statusCheck{
				{State: "pending"},
				{Status: "IN_PROGRESS"},
				{Status: "QUEUED"},
			},
			expected: models.ChecksStatus{
				Total:   3,
				Pending: 3,
			},
		},
		{
			name: "skipped checks",
			input: []statusCheck{
				{Conclusion: "skipped"},
				{Conclusion: "neutral"},
			},
			expected: models.ChecksStatus{
				Total:   2,
				Skipped: 2,
			},
		},
		{
			name: "mixed status",
			input: []statusCheck{
				{Conclusion: "success"},
				{Conclusion: "failure"},
				{State: "pending"},
				{Conclusion: "skipped"},
			},
			expected: models.ChecksStatus{
				Total:   4,
				Passing: 1,
				Failing: 1,
				Pending: 1,
				Skipped: 1,
			},
		},
		{
			name: "state success overrides",
			input: []statusCheck{
				{State: "success"},
			},
			expected: models.ChecksStatus{
				Total:   1,
				Passing: 1,
			},
		},
		{
			name: "state failure overrides",
			input: []statusCheck{
				{State: "failure"},
			},
			expected: models.ChecksStatus{
				Total:   1,
				Failing: 1,
			},
		},
		{
			name: "unknown state defaults to pending",
			input: []statusCheck{
				{State: "unknown"},
			},
			expected: models.ChecksStatus{
				Total:   1,
				Pending: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseChecks(tt.input)
			if result != tt.expected {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}
