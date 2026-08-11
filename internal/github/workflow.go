package github

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// GetWorkflowRunsForCommit returns the CI workflow run summary for a commit, using the cache when fresh.
func GetWorkflowRunsForCommit(
	ctx context.Context, repoPath, remoteID, commitSHA string,
) (*models.WorkflowSummary, error) {
	if commitSHA == "" {
		//nolint:nilnil // no commit means nothing to look up, not a failure
		return nil, nil
	}

	cacheKey := cache.RemoteScope(repoPath, remoteID) + "\x00run:" + commitSHA
	if cached, ok := cache.WorkflowCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	out, err := runGH(ctx, repoPath, env, "run", "list",
		"--commit", commitSHA,
		"--json", "databaseId,name,status,conclusion,url,createdAt,updatedAt",
		"--limit", "10")
	if err != nil {
		return nil, err
	}

	var runs []struct {
		DatabaseID int64  `json:"databaseId"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		URL        string `json:"url"`
		CreatedAt  string `json:"createdAt"`
		UpdatedAt  string `json:"updatedAt"`
	}

	if err := json.Unmarshal(out, &runs); err != nil {
		return nil, fmt.Errorf("parsing gh run list output: %w", err)
	}

	summary := &models.WorkflowSummary{
		Runs:  make([]models.WorkflowRun, 0, len(runs)),
		Total: len(runs),
	}

	for _, r := range runs {
		// A malformed timestamp degrades to a zero time rather than failing
		// the whole run list.
		createdAt, _ := time.Parse(time.RFC3339, r.CreatedAt) //nolint:errcheck // best-effort, see comment above
		updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt) //nolint:errcheck // best-effort, see comment above

		run := models.WorkflowRun{
			ID:         r.DatabaseID,
			Name:       r.Name,
			Status:     r.Status,
			Conclusion: r.Conclusion,
			URL:        r.URL,
			CreatedAt:  createdAt,
			UpdatedAt:  updatedAt,
		}
		summary.Runs = append(summary.Runs, run)

		switch {
		case r.Status == "in_progress" || r.Status == "queued":
			summary.InProgress++
		case r.Conclusion == "success":
			summary.Passing++
		case r.Conclusion == checkStateFailure:
			summary.Failing++
		}
	}

	cache.WorkflowCache.Set(cacheKey, summary)

	return summary, nil
}

// ciRunLimit bounds how many runs one commit's history is read back to. A
// commit with more runs than this has re-run workflows; the newest win.
const ciRunLimit = "30"

// GetDefaultBranchCI returns the latest run of each workflow on the repo's
// default branch head, with the failing job names filled in for red runs.
//
// The branch and commit come from local refs, so a healthy repo costs one gh
// call; each failing workflow costs one more to name what broke.
func GetDefaultBranchCI(ctx context.Context, repoPath string) (*models.DefaultBranchCI, error) {
	def, err := vcs.DefaultBranchHead(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolving the default branch: %w", err)
	}

	runs, err := latestRunPerWorkflow(ctx, repoPath, def.SHA)
	if err != nil {
		return nil, err
	}

	for i := range runs {
		if runs[i].Conclusion != checkStateFailure {
			continue
		}

		//nolint:errcheck // a missing job list leaves the run red without detail
		runs[i].FailingJobs, _ = failingJobs(ctx, repoPath, runs[i].ID)
	}

	return &models.DefaultBranchCI{Branch: def.Name, SHA: def.SHA, Workflows: runs}, nil
}

func latestRunPerWorkflow(ctx context.Context, repoPath, sha string) ([]models.CIWorkflowRun, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "run", "list",
		"--commit", sha,
		"--json", "databaseId,workflowName,status,conclusion,url,startedAt,updatedAt",
		"--limit", ciRunLimit)
	if err != nil {
		return nil, fmt.Errorf("listing runs for %s: %w", sha, err)
	}

	var raw []struct {
		DatabaseID   int64  `json:"databaseId"`
		WorkflowName string `json:"workflowName"`
		Status       string `json:"status"`
		Conclusion   string `json:"conclusion"`
		URL          string `json:"url"`
		StartedAt    string `json:"startedAt"`
		UpdatedAt    string `json:"updatedAt"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing gh run list output: %w", err)
	}

	latest := make(map[string]models.CIWorkflowRun, len(raw))
	for _, r := range raw {
		run := models.CIWorkflowRun{
			ID:         r.DatabaseID,
			Workflow:   r.WorkflowName,
			Status:     r.Status,
			Conclusion: r.Conclusion,
			URL:        r.URL,
			StartedAt:  parseTime(r.StartedAt),
			UpdatedAt:  parseTime(r.UpdatedAt),
		}

		if seen, ok := latest[run.Workflow]; ok && seen.UpdatedAt.After(run.UpdatedAt) {
			continue
		}

		latest[run.Workflow] = run
	}

	runs := make([]models.CIWorkflowRun, 0, len(latest))
	for name := range latest {
		runs = append(runs, latest[name])
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Workflow < runs[j].Workflow })

	return runs, nil
}

func failingJobs(ctx context.Context, repoPath string, runID int64) ([]string, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "run", "view",
		strconv.FormatInt(runID, 10), "--json", "jobs")
	if err != nil {
		return nil, fmt.Errorf("reading run %d: %w", runID, err)
	}

	var payload struct {
		Jobs []struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
			Steps      []struct {
				Name       string `json:"name"`
				Conclusion string `json:"conclusion"`
			} `json:"steps"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parsing gh run view output: %w", err)
	}

	var failing []string
	for _, job := range payload.Jobs {
		if job.Conclusion != checkStateFailure {
			continue
		}

		named := false
		for _, step := range job.Steps {
			if step.Conclusion == checkStateFailure {
				failing = append(failing, job.Name+" / "+step.Name)
				named = true
			}
		}

		if !named {
			failing = append(failing, job.Name)
		}
	}

	return failing, nil
}

// DependabotAlerts returns the count of open Dependabot alerts by severity.
// A repo whose alerts endpoint denies access (archived repos, or a token
// without the scope) reports an empty map rather than an error, because the
// distinction is not actionable to a fleet report.
func DependabotAlerts(ctx context.Context, repoPath, remoteRepo string) map[string]int {
	counts := map[string]int{}
	if remoteRepo == "" {
		return counts
	}

	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api",
		"repos/"+remoteRepo+"/dependabot/alerts?state=open&per_page=100")
	if err != nil {
		return counts
	}

	var alerts []struct {
		SecurityAdvisory struct {
			Severity string `json:"severity"`
		} `json:"security_advisory"`
	}
	if err := json.Unmarshal(out, &alerts); err != nil {
		return counts
	}

	for _, alert := range alerts {
		counts[alert.SecurityAdvisory.Severity]++
	}

	return counts
}
