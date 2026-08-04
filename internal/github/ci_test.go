package github_test

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestDefaultBranchCIConclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ci   models.DefaultBranchCI
		want string
	}{
		{name: "no runs", ci: models.DefaultBranchCI{}, want: "—"},
		{
			name: "every workflow green",
			ci: models.DefaultBranchCI{Workflows: []models.CIWorkflowRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "success"},
			}},
			want: models.StatusPassing,
		},
		{
			name: "one red outweighs the rest",
			ci: models.DefaultBranchCI{Workflows: []models.CIWorkflowRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "failure"},
			}},
			want: models.StatusFailing,
		},
		{
			name: "still running",
			ci: models.DefaultBranchCI{Workflows: []models.CIWorkflowRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "in_progress"},
			}},
			want: "pending",
		},
		{
			name: "a red run wins even while another is running",
			ci: models.DefaultBranchCI{Workflows: []models.CIWorkflowRun{
				{Status: "in_progress"},
				{Status: "completed", Conclusion: "failure"},
			}},
			want: models.StatusFailing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.ci.Conclusion(); got != tt.want {
				t.Errorf("Conclusion = %q, want %q", got, tt.want)
			}
		})
	}
}

//nolint:paralleltest // asserts against the shared gh runner stub
func TestDependabotAlertsGroupsBySeverity(t *testing.T) {
	alertsJSON := []byte(`[
		{"security_advisory": {"severity": "high"}},
		{"security_advisory": {"severity": "high"}},
		{"security_advisory": {"severity": "low"}}
	]`)

	ctx, calls := stubRunGH(alertsJSON, nil)

	counts := github.DependabotAlerts(ctx, "/repo", "acme/app")
	if counts["high"] != 2 || counts["low"] != 1 {
		t.Errorf("alert counts = %v, want 2 high and 1 low", counts)
	}

	if joined := strings.Join((*calls)[0], " "); !strings.Contains(joined, "state=open") {
		t.Errorf("alerts call %q does not ask for open alerts only", joined)
	}
}

//nolint:paralleltest // asserts against the shared gh runner stub
func TestDependabotAlertsAreEmptyWhenAccessIsDenied(t *testing.T) {
	ctx, _ := stubRunGH(nil, errGHFailed)

	if counts := github.DependabotAlerts(ctx, "/repo", "acme/archived"); len(counts) != 0 {
		t.Errorf("a denied endpoint reported %v, want an empty map", counts)
	}

	if counts := github.DependabotAlerts(ctx, "/repo", ""); len(counts) != 0 {
		t.Errorf("a repo with no remote reported %v, want an empty map", counts)
	}
}
