package github_test

import (
	"testing"

	"github.com/kyleking/aragonite/forge"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
)

type parseChecksTestCase struct {
	name     string
	input    []github.StatusCheck
	expected forge.ChecksStatus
}

func parseChecksTestCases() []parseChecksTestCase {
	return []parseChecksTestCase{
		{name: "empty checks", input: nil, expected: forge.ChecksStatus{Total: 0}},
		{
			name:     "all passing",
			input:    []github.StatusCheck{{Conclusion: "success"}, {Conclusion: "success"}},
			expected: forge.ChecksStatus{Total: 2, Passing: 2},
		},
		{
			name:     "all failing",
			input:    []github.StatusCheck{{Conclusion: "failure"}, {Conclusion: "error"}},
			expected: forge.ChecksStatus{Total: 2, Failing: 2},
		},
		{
			name: "pending checks",
			input: []github.StatusCheck{
				{State: "pending"}, {Status: "IN_PROGRESS"}, {Status: "QUEUED"},
			},
			expected: forge.ChecksStatus{Total: 3, Pending: 3},
		},
		{
			name:     "skipped checks",
			input:    []github.StatusCheck{{Conclusion: "skipped"}, {Conclusion: "neutral"}},
			expected: forge.ChecksStatus{Total: 2, Skipped: 2},
		},
		{
			name: "mixed status",
			input: []github.StatusCheck{
				{Conclusion: "success"}, {Conclusion: "failure"}, {State: "pending"}, {Conclusion: "skipped"},
			},
			expected: forge.ChecksStatus{Total: 4, Passing: 1, Failing: 1, Pending: 1, Skipped: 1},
		},
		{
			name:     "state success overrides",
			input:    []github.StatusCheck{{State: "success"}},
			expected: forge.ChecksStatus{Total: 1, Passing: 1},
		},
		{
			name:     "state failure overrides",
			input:    []github.StatusCheck{{State: "failure"}},
			expected: forge.ChecksStatus{Total: 1, Failing: 1},
		},
		{
			name:     "unknown state defaults to pending",
			input:    []github.StatusCheck{{State: "unknown"}},
			expected: forge.ChecksStatus{Total: 1, Pending: 1},
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
