package github_test

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
)

// A view written for one repo says nothing about whose work it is, so widening
// it needs both a subject and gh search's own spelling of the sort.
func TestFleetSearchArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		query   string
		want    []string
		notWant []string
	}{
		{
			name:    "a query with no subject is scoped to the operator",
			query:   "is:open sort:updated-desc",
			want:    []string{"is:open", "involves:@me", "--sort", "updated", "--order", "desc"},
			notWant: []string{"sort:updated-desc"},
		},
		{
			name:    "a query that already names a subject is left alone",
			query:   "is:open review-requested:@me",
			want:    []string{"review-requested:@me"},
			notWant: []string{"involves:@me"},
		},
		{
			name:    "an org query is a subject too",
			query:   "org:acme is:open",
			notWant: []string{"involves:@me"},
			want:    []string{"org:acme"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := strings.Join(github.FleetSearchArgs(tt.query), " ")
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("args %q are missing %q", got, want)
				}
			}
			for _, unwanted := range tt.notWant {
				if strings.Contains(got, unwanted) {
					t.Errorf("args %q still carry %q", got, unwanted)
				}
			}
		})
	}
}
