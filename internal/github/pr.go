package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

type prResponse struct {
	Number            int           `json:"number"`
	Title             string        `json:"title"`
	State             string        `json:"state"`
	URL               string        `json:"url"`
	IsDraft           bool          `json:"isDraft"`
	MergeStateStatus  string        `json:"mergeStateStatus"`
	HeadRefName       string        `json:"headRefName"`
	BaseRefName       string        `json:"baseRefName"`
	StatusCheckRollup []statusCheck `json:"statusCheckRollup"`
}

type statusCheck struct {
	State      string `json:"state,omitempty"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
}

// GetPRForBranch returns the pull request associated with branch, if any, using the cache when fresh.
func GetPRForBranch(ctx context.Context, repoPath, branch, upstream string) (*models.PRInfo, error) {
	cacheKey := upstream + ":" + branch
	if cached, ok := cache.PRCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	out, err := runGH(ctx, repoPath, env, "pr", "view", branch,
		"--json", "number,title,state,url,isDraft,mergeStateStatus,headRefName,baseRefName,statusCheckRollup")
	if err != nil {
		cache.PRCache.Set(cacheKey, nil)
		return nil, err
	}

	var resp prResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parsing gh pr view output: %w", err)
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

const (
	checkStateSuccess = "success"
	checkStateFailure = "failure"
	checkStateError   = "error"
)

func parseChecks(checks []statusCheck) models.ChecksStatus {
	var status models.ChecksStatus
	status.Total = len(checks)

	for _, c := range checks {
		state := strings.ToLower(c.State)
		conclusion := strings.ToLower(c.Conclusion)

		switch {
		case state == "pending" || c.Status == "IN_PROGRESS" || c.Status == "QUEUED":
			status.Pending++
		case conclusion == checkStateSuccess || state == checkStateSuccess:
			status.Passing++
		case conclusion == checkStateFailure || conclusion == checkStateError ||
			state == checkStateFailure || state == checkStateError:
			status.Failing++
		case conclusion == "skipped" || conclusion == "neutral":
			status.Skipped++
		default:
			status.Pending++
		}
	}

	return status
}

// GetPRDetail returns the full detail for a single pull request, using the cache when fresh.
func GetPRDetail(ctx context.Context, repoPath string, prNumber int) (*models.PRDetail, error) {
	cacheKey := fmt.Sprintf("%s:pr:%d", repoPath, prNumber)
	if cached, ok := cache.PRDetailCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	prDetailFields := "number,title,state,url,isDraft,mergeStateStatus,headRefName,baseRefName,body," +
		"author,assignees,reviewRequests,createdAt,updatedAt,additions,deletions,comments,reviewDecision"
	out, err := runGH(ctx, repoPath, env, "pr", "view", strconv.Itoa(prNumber), "--json", prDetailFields)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Number           int    `json:"number"`
		Title            string `json:"title"`
		State            string `json:"state"`
		URL              string `json:"url"`
		IsDraft          bool   `json:"isDraft"`
		MergeStateStatus string `json:"mergeStateStatus"`
		HeadRefName      string `json:"headRefName"`
		BaseRefName      string `json:"baseRefName"`
		Body             string `json:"body"`
		Author           struct {
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
		return nil, fmt.Errorf("parsing gh pr view output: %w", err)
	}

	// A malformed timestamp degrades to a zero time rather than failing the
	// whole detail fetch.
	createdAt, _ := time.Parse(time.RFC3339, resp.CreatedAt) //nolint:errcheck // best-effort, see comment above
	updatedAt, _ := time.Parse(time.RFC3339, resp.UpdatedAt) //nolint:errcheck // best-effort, see comment above

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

// GetPRsForRepo returns all open pull requests for the repo, using the cache when fresh.
func GetPRsForRepo(ctx context.Context, repoPath, upstream string) ([]models.PRInfo, error) {
	if upstream == "" {
		return []models.PRInfo{}, nil
	}

	cacheKey := upstream + ":all_prs"
	if cached, ok := cache.PRListCache.Get(cacheKey); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	out, err := runGH(ctx, repoPath, env, "pr", "list",
		"--json", "number,title,state,url,isDraft,headRefName,baseRefName,reviewDecision",
		"--limit", "100")
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
		return []models.PRInfo{}, fmt.Errorf("parsing gh pr list output: %w", err)
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

// GetPRCount returns the number of open pull requests for the repo, using the cache when fresh.
func GetPRCount(ctx context.Context, repoPath, upstream string) (int, error) {
	prs, err := GetPRsForRepo(ctx, repoPath, upstream)
	if err != nil {
		return 0, err
	}

	return len(prs), nil
}
