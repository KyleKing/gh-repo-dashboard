package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

type prResponse struct {
	Number              int    `json:"number"`
	Title               string `json:"title"`
	State               string `json:"state"`
	URL                 string `json:"url"`
	IsDraft             bool   `json:"isDraft"`
	MergeStateStatus    string `json:"mergeStateStatus"`
	HeadRefName         string `json:"headRefName"`
	BaseRefName         string `json:"baseRefName"`
	StatusCheckRollup   []statusCheck `json:"statusCheckRollup"`
}

type statusCheck struct {
	State      string `json:"state,omitempty"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
}

func GetPRForBranch(ctx context.Context, repoPath string, branch string, upstream string) (*models.PRInfo, error) {
	cacheKey := upstream + ":" + branch
	if cached, ok := cache.PRCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	cmd := exec.CommandContext(ctx, "gh", "pr", "view", branch,
		"--json", "number,title,state,url,isDraft,mergeStateStatus,headRefName,baseRefName,statusCheckRollup")
	cmd.Dir = repoPath
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	out, err := cmd.Output()
	if err != nil {
		cache.PRCache.Set(cacheKey, nil)
		return nil, err
	}

	var resp prResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}

	checks := parseChecks(resp.StatusCheckRollup)

	pr := &models.PRInfo{
		Number:    resp.Number,
		Title:     resp.Title,
		State:     resp.State,
		URL:       resp.URL,
		IsDraft:   resp.IsDraft,
		Mergeable: resp.MergeStateStatus,
		HeadRef:   resp.HeadRefName,
		BaseRef:   resp.BaseRefName,
		Checks:    checks,
	}

	cache.PRCache.Set(cacheKey, pr)
	return pr, nil
}

func parseChecks(checks []statusCheck) models.ChecksStatus {
	var status models.ChecksStatus
	status.Total = len(checks)

	for _, c := range checks {
		state := strings.ToLower(c.State)
		conclusion := strings.ToLower(c.Conclusion)

		switch {
		case state == "pending" || c.Status == "IN_PROGRESS" || c.Status == "QUEUED":
			status.Pending++
		case conclusion == "success" || state == "success":
			status.Passing++
		case conclusion == "failure" || conclusion == "error" || state == "failure" || state == "error":
			status.Failing++
		case conclusion == "skipped" || conclusion == "neutral":
			status.Skipped++
		default:
			status.Pending++
		}
	}

	return status
}

func GetPRDetail(ctx context.Context, repoPath string, prNumber int) (*models.PRDetail, error) {
	cacheKey := fmt.Sprintf("%s:pr:%d", repoPath, prNumber)
	if cached, ok := cache.PRDetailCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	cmd := exec.CommandContext(ctx, "gh", "pr", "view", strconv.Itoa(prNumber),
		"--json", "number,title,state,url,isDraft,mergeStateStatus,headRefName,baseRefName,body,author,assignees,reviewRequests,createdAt,updatedAt,additions,deletions,comments,reviewDecision")
	cmd.Dir = repoPath
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var resp struct {
		Number         int    `json:"number"`
		Title          string `json:"title"`
		State          string `json:"state"`
		URL            string `json:"url"`
		IsDraft        bool   `json:"isDraft"`
		MergeStateStatus string `json:"mergeStateStatus"`
		HeadRefName    string `json:"headRefName"`
		BaseRefName    string `json:"baseRefName"`
		Body           string `json:"body"`
		Author         struct {
			Login string `json:"login"`
		} `json:"author"`
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		ReviewRequests []struct {
			Login string `json:"login"`
		} `json:"reviewRequests"`
		CreatedAt      string `json:"createdAt"`
		UpdatedAt      string `json:"updatedAt"`
		Additions      int    `json:"additions"`
		Deletions      int    `json:"deletions"`
		Comments       int    `json:"comments"`
		ReviewDecision string `json:"reviewDecision"`
	}

	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}

	createdAt, _ := time.Parse(time.RFC3339, resp.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, resp.UpdatedAt)

	assignees := make([]string, 0, len(resp.Assignees))
	for _, a := range resp.Assignees {
		assignees = append(assignees, a.Login)
	}

	reviewers := make([]string, 0, len(resp.ReviewRequests))
	for _, r := range resp.ReviewRequests {
		reviewers = append(reviewers, r.Login)
	}

	detail := &models.PRDetail{
		PRInfo: models.PRInfo{
			Number:         resp.Number,
			Title:          resp.Title,
			State:          resp.State,
			URL:            resp.URL,
			IsDraft:        resp.IsDraft,
			Mergeable:      resp.MergeStateStatus,
			HeadRef:        resp.HeadRefName,
			BaseRef:        resp.BaseRefName,
			ReviewDecision: resp.ReviewDecision,
		},
		Body:      resp.Body,
		Author:    resp.Author.Login,
		Assignees: assignees,
		Reviewers: reviewers,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Additions: resp.Additions,
		Deletions: resp.Deletions,
		Comments:  resp.Comments,
	}

	cache.PRDetailCache.Set(cacheKey, detail)
	return detail, nil
}

func GetPRsForRepo(ctx context.Context, repoPath string, upstream string) ([]models.PRInfo, error) {
	if upstream == "" {
		return []models.PRInfo{}, nil
	}

	cacheKey := upstream + ":all_prs"
	if cached, ok := cache.PRListCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	cmd := exec.CommandContext(ctx, "gh", "pr", "list",
		"--json", "number,title,state,url,isDraft,headRefName,baseRefName,reviewDecision",
		"--limit", "100")
	cmd.Dir = repoPath
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	out, err := cmd.Output()
	if err != nil {
		cache.PRListCache.Set(cacheKey, []models.PRInfo{})
		return []models.PRInfo{}, err
	}

	var prList []struct {
		Number         int    `json:"number"`
		Title          string `json:"title"`
		State          string `json:"state"`
		URL            string `json:"url"`
		IsDraft        bool   `json:"isDraft"`
		HeadRefName    string `json:"headRefName"`
		BaseRefName    string `json:"baseRefName"`
		ReviewDecision string `json:"reviewDecision"`
	}

	if err := json.Unmarshal(out, &prList); err != nil {
		return []models.PRInfo{}, err
	}

	result := make([]models.PRInfo, 0, len(prList))
	for _, pr := range prList {
		result = append(result, models.PRInfo{
			Number:         pr.Number,
			Title:          pr.Title,
			State:          pr.State,
			URL:            pr.URL,
			IsDraft:        pr.IsDraft,
			HeadRef:        pr.HeadRefName,
			BaseRef:        pr.BaseRefName,
			ReviewDecision: pr.ReviewDecision,
		})
	}

	cache.PRListCache.Set(cacheKey, result)
	return result, nil
}

func GetPRCount(ctx context.Context, repoPath string, upstream string) (int, error) {
	prs, err := GetPRsForRepo(ctx, repoPath, upstream)
	if err != nil {
		return 0, err
	}
	return len(prs), nil
}
