package filters_test

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestParsePredicatePR(t *testing.T) {
	t.Parallel()
	draftFailing := models.PRInfo{IsDraft: true, Checks: models.ChecksStatus{Total: 3, Failing: 1}}
	readyPassing := models.PRInfo{ReviewDecision: "APPROVED", Checks: models.ChecksStatus{Total: 3}}
	changesRequested := models.PRInfo{ReviewDecision: "CHANGES_REQUESTED"}

	tests := []struct {
		name     string
		expr     string
		pr       models.PRInfo
		expected bool
	}{
		{"draft match", "draft", draftFailing, true},
		{"draft no match", "draft", readyPassing, false},
		{"failing match", "failing", draftFailing, true},
		{"passing match", "passing", readyPassing, true},
		{"passing excludes failing", "passing", draftFailing, false},
		{"approved match", "approved", readyPassing, true},
		{"changes_requested match", "changes_requested", changesRequested, true},
		{"draft and failing", "draft and failing", draftFailing, true},
		{"not draft", "not draft", readyPassing, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pred, err := filters.ParsePredicate(tt.expr, filters.PRAtoms())
			if err != nil {
				t.Fatalf("filters.ParsePredicate(%q) error: %v", tt.expr, err)
			}
			if got := pred(tt.pr); got != tt.expected {
				t.Errorf("%q on %+v = %v; want %v", tt.expr, tt.pr, got, tt.expected)
			}
		})
	}
}
