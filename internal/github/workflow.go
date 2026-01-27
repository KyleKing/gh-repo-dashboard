package github

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

type workflowResponse struct {
	Runs []workflowRun `json:"workflow_runs"`
}

type workflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func GetWorkflowRunsForCommit(ctx context.Context, repoPath string, commitSHA string) (*models.WorkflowSummary, error) {
	if commitSHA == "" {
		return nil, nil
	}

	cacheKey := repoPath + ":" + commitSHA
	if cached, ok := cache.WorkflowCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	cmd := exec.CommandContext(ctx, "gh", "run", "list",
		"--commit", commitSHA,
		"--json", "databaseId,name,status,conclusion,url,createdAt,updatedAt",
		"--limit", "10")
	cmd.Dir = repoPath
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	out, err := cmd.Output()
	if err != nil {
		cache.WorkflowCache.Set(cacheKey, nil)
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
		return nil, err
	}

	summary := &models.WorkflowSummary{
		Runs:  make([]models.WorkflowRun, 0, len(runs)),
		Total: len(runs),
	}

	for _, r := range runs {
		createdAt, _ := time.Parse(time.RFC3339, r.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)

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
		case r.Conclusion == "failure":
			summary.Failing++
		}
	}

	cache.WorkflowCache.Set(cacheKey, summary)
	return summary, nil
}
