package github

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

func GetWorkflowRunsForCommit(ctx context.Context, repoPath, commitSHA string) (*models.WorkflowSummary, error) {
	if commitSHA == "" {
		//nolint:nilnil // no commit means nothing to look up, not a failure
		return nil, nil
	}

	cacheKey := repoPath + ":" + commitSHA
	if cached, ok := cache.WorkflowCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	out, err := runGH(ctx, repoPath, env, "run", "list",
		"--commit", commitSHA,
		"--json", "databaseId,name,status,conclusion,url,createdAt,updatedAt",
		"--limit", "10")
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
		case r.Conclusion == "failure":
			summary.Failing++
		}
	}

	cache.WorkflowCache.Set(cacheKey, summary)

	return summary, nil
}
