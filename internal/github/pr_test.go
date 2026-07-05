package github_test

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type parseChecksTestCase struct {
	name     string
	input    []github.StatusCheck
	expected models.ChecksStatus
}

func parseChecksTestCases() []parseChecksTestCase {
	return []parseChecksTestCase{
		{name: "empty checks", input: nil, expected: models.ChecksStatus{Total: 0}},
		{
			name:     "all passing",
			input:    []github.StatusCheck{{Conclusion: "success"}, {Conclusion: "success"}},
			expected: models.ChecksStatus{Total: 2, Passing: 2},
		},
		{
			name:     "all failing",
			input:    []github.StatusCheck{{Conclusion: "failure"}, {Conclusion: "error"}},
			expected: models.ChecksStatus{Total: 2, Failing: 2},
		},
		{
			name: "pending checks",
			input: []github.StatusCheck{
				{State: "pending"}, {Status: "IN_PROGRESS"}, {Status: "QUEUED"},
			},
			expected: models.ChecksStatus{Total: 3, Pending: 3},
		},
		{
			name:     "skipped checks",
			input:    []github.StatusCheck{{Conclusion: "skipped"}, {Conclusion: "neutral"}},
			expected: models.ChecksStatus{Total: 2, Skipped: 2},
		},
		{
			name: "mixed status",
			input: []github.StatusCheck{
				{Conclusion: "success"}, {Conclusion: "failure"}, {State: "pending"}, {Conclusion: "skipped"},
			},
			expected: models.ChecksStatus{Total: 4, Passing: 1, Failing: 1, Pending: 1, Skipped: 1},
		},
		{
			name:     "state success overrides",
			input:    []github.StatusCheck{{State: "success"}},
			expected: models.ChecksStatus{Total: 1, Passing: 1},
		},
		{
			name:     "state failure overrides",
			input:    []github.StatusCheck{{State: "failure"}},
			expected: models.ChecksStatus{Total: 1, Failing: 1},
		},
		{
			name:     "unknown state defaults to pending",
			input:    []github.StatusCheck{{State: "unknown"}},
			expected: models.ChecksStatus{Total: 1, Pending: 1},
		},
	}
}

func TestParseChecks(t *testing.T) {
	t.Parallel()

	for _, tt := range parseChecksTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := github.ParseChecks(tt.input)
			if result != tt.expected {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}
