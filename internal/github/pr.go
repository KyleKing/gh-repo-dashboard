package github

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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

// PRCacheKey and PRListCacheKey scope cache entries by the remote the values
// were read from, so parallel checkouts of one repository share a lookup;
// upstream is part of the key only because a repo can track more than one
// remote. Exported so callers seeding the cache can't drift from this format.
func PRCacheKey(repoPath, remoteID, upstream, branch string) string {
	return cache.RemoteScope(repoPath, remoteID) + "\x00" + upstream + ":" + branch
}

// PRListCacheKey builds the cache key for the full PR list of a repo's upstream.
func PRListCacheKey(repoPath, remoteID, upstream string) string {
	return cache.RemoteScope(repoPath, remoteID) + "\x00" + upstream + ":all_prs"
}

// PRDetailCacheKey builds the cache key for one pull request's full detail.
func PRDetailCacheKey(repoPath, remoteID string, prNumber int) string {
	return cache.RemoteScope(repoPath, remoteID) + "\x00pr:" + strconv.Itoa(prNumber)
}

// MergedPRHeadsCacheKey builds the cache key for a remote's merged-PR head map.
func MergedPRHeadsCacheKey(repoPath, remoteID string) string {
	return cache.RemoteScope(repoPath, remoteID) + "\x00merged_pr_heads"
}

// CachedPRForBranch returns the cached pull request for branch, if any, without invoking gh.
func CachedPRForBranch(repoPath, remoteID, branch, upstream string) (*models.PRInfo, bool) {
	return cache.PRCache.Get(PRCacheKey(repoPath, remoteID, upstream, branch), vcs.Stamp(repoPath))
}

// CachedPRs returns the cached open pull request list for the repo without
// invoking gh, reading the remote's cache file when memory misses.
func CachedPRs(repoPath, remoteID, upstream string) ([]models.PRInfo, bool) {
	key := PRListCacheKey(repoPath, remoteID, upstream)

	return cache.Persisted(cache.PRListCache, remoteID, key, vcs.Stamp(repoPath))
}

// GetPRForBranch returns the pull request associated with branch, if any, using the cache when fresh.
func GetPRForBranch(ctx context.Context, repoPath, remoteID, branch, upstream string) (*models.PRInfo, error) {
	cacheKey := PRCacheKey(repoPath, remoteID, upstream, branch)
	if cached, ok := CachedPRForBranch(repoPath, remoteID, branch, upstream); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	out, err := runGH(ctx, repoPath, env, "pr", "view", branch,
		"--json", "number,title,state,url,isDraft,mergeStateStatus,headRefName,baseRefName,statusCheckRollup")
	if err != nil {
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

	cache.PRCache.Set(cacheKey, vcs.Stamp(repoPath), pr)

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

// prComment mirrors one entry of gh's `comments` array. The field is a list of
// comment objects, not a count, so it's decoded as such and counted here.
type prComment struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

// detailedCheck mirrors one entry of gh's `statusCheckRollup` with the
// per-check fields the PR detail view shows, beyond the rollup counts.
type detailedCheck struct {
	Name         string `json:"name"`
	WorkflowName string `json:"workflowName"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	State        string `json:"state"`
	StartedAt    string `json:"startedAt"`
	CompletedAt  string `json:"completedAt"`
}

// latestComment returns the most recently created comment, or nil when the
// pull request has none.
func latestComment(comments []prComment) *models.PRComment {
	if len(comments) == 0 {
		return nil
	}

	latest := comments[0]
	latestAt := parseTime(latest.CreatedAt)
	for i := range comments[1:] {
		c := comments[i+1]
		if at := parseTime(c.CreatedAt); at.After(latestAt) {
			latest, latestAt = c, at
		}
	}

	return &models.PRComment{Author: latest.Author.Login, Body: latest.Body, CreatedAt: latestAt}
}

func parseCheckDetails(checks []detailedCheck) []models.CheckDetail {
	details := make([]models.CheckDetail, 0, len(checks))
	for _, c := range checks {
		// Non-workflow checks (commit statuses) report `state` instead of
		// `status`/`conclusion`, and carry no timings.
		status, conclusion := c.Status, c.Conclusion
		if status == "" && c.State != "" {
			status, conclusion = "COMPLETED", c.State
		}

		details = append(details, models.CheckDetail{
			Name:        c.Name,
			Workflow:    c.WorkflowName,
			Status:      status,
			Conclusion:  conclusion,
			StartedAt:   parseTime(c.StartedAt),
			CompletedAt: parseTime(c.CompletedAt),
		})
	}

	return details
}

// parseTime decodes an RFC 3339 timestamp, degrading to the zero time rather
// than failing the surrounding fetch.
func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

// GetPRDetail returns the full detail for a single pull request, using the cache when fresh.
func GetPRDetail(ctx context.Context, repoPath, remoteID string, prNumber int) (*models.PRDetail, error) {
	cacheKey := PRDetailCacheKey(repoPath, remoteID, prNumber)
	if cached, ok := cache.PRDetailCache.Get(cacheKey, vcs.Stamp(repoPath)); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	prDetailFields := "number,title,state,url,isDraft,mergeStateStatus,headRefName,baseRefName,body," +
		"author,assignees,reviewRequests,createdAt,updatedAt,additions,deletions,comments,reviewDecision," +
		"statusCheckRollup"
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
		CreatedAt         string          `json:"createdAt"`
		UpdatedAt         string          `json:"updatedAt"`
		Additions         int             `json:"additions"`
		Deletions         int             `json:"deletions"`
		Comments          []prComment     `json:"comments"`
		ReviewDecision    string          `json:"reviewDecision"`
		StatusCheckRollup []detailedCheck `json:"statusCheckRollup"`
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
		Body:          resp.Body,
		Author:        resp.Author.Login,
		Assignees:     assignees,
		Reviewers:     reviewers,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		Additions:     resp.Additions,
		Deletions:     resp.Deletions,
		Comments:      len(resp.Comments),
		LatestComment: latestComment(resp.Comments),
		CheckDetails:  parseCheckDetails(resp.StatusCheckRollup),
	}

	cache.PRDetailCache.Set(cacheKey, vcs.Stamp(repoPath), detail)

	return detail, nil
}

// Fields and page size for one `gh pr list` call. Because the rollup, comment,
// and review fields are each walked per pull request, the cost is linear in the
// page size: on a repo with ~105 open pull requests carrying ~25 check runs
// apiece, a page of 100 takes GitHub's GraphQL gateway past its own timeout and
// comes back 504. Thirty is what fits with room to spare.
const (
	prListFields = "number,title,state,url,isDraft,headRefName,headRepositoryOwner,baseRefName," +
		"reviewDecision,statusCheckRollup,comments,reviews"
	prListLimit = "30"
	// Number of filtered pages that make up one repo's list.
	prListPages = 2
)

// GetPRsForRepo returns the open pull requests worth showing for the repo,
// using the cache when fresh.
//
// It reads two pages rather than one: the newest pull requests that are not the
// operator's, and the operator's own. Splitting the budget that way keeps a
// repo whose pull requests all belong to a bot from spending it on the operator
// (who has none there), and keeps the operator's older work from falling off
// the recent list on a busy repo. Either page failing still returns the other.
func GetPRsForRepo(ctx context.Context, repoPath, remoteID, upstream string) ([]models.PRInfo, error) {
	if upstream == "" {
		return []models.PRInfo{}, nil
	}

	cacheKey := PRListCacheKey(repoPath, remoteID, upstream)
	if cached, ok := CachedPRs(repoPath, remoteID, upstream); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	others, othersErr := prListPage(ctx, repoPath, env, "--search", "-author:@me")
	mine, mineErr := prListPage(ctx, repoPath, env, "--author", "@me")

	if othersErr != nil && mineErr != nil {
		// Nothing is cached on failure: an empty list would otherwise read as
		// "this repo has no pull requests" for the whole cache window, and the
		// panel would be hidden on the strength of a timeout.
		return []models.PRInfo{}, othersErr
	}

	result := mergePRPages(others, mine)
	cache.Persist(cache.PRListCache, remoteID, cacheKey, vcs.Stamp(repoPath), result)

	return result, nil
}

// mergePRPages joins the pages newest-first, dropping the pull requests a
// repo's pages both name.
func mergePRPages(pages ...[]models.PRInfo) []models.PRInfo {
	seen := make(map[int]bool)

	merged := make([]models.PRInfo, 0)
	for _, page := range pages {
		for i := range page {
			if seen[page[i].Number] {
				continue
			}
			seen[page[i].Number] = true
			merged = append(merged, page[i])
		}
	}

	slices.SortFunc(merged, func(a, b models.PRInfo) int { return b.Number - a.Number })

	return merged
}

func prListPage(ctx context.Context, repoPath string, env []string, filter ...string) ([]models.PRInfo, error) {
	args := append([]string{"pr", "list", "--json", prListFields, "--limit", prListLimit}, filter...)

	out, err := runGH(ctx, repoPath, env, args...)
	if err != nil {
		return nil, err
	}

	var prList []struct {
		Number              int    `json:"number"`
		Title               string `json:"title"`
		State               string `json:"state"`
		URL                 string `json:"url"`
		IsDraft             bool   `json:"isDraft"`
		HeadRefName         string `json:"headRefName"`
		BaseRefName         string `json:"baseRefName"`
		HeadRepositoryOwner struct {
			Login string `json:"login"`
		} `json:"headRepositoryOwner"`
		ReviewDecision    string        `json:"reviewDecision"`
		StatusCheckRollup []statusCheck `json:"statusCheckRollup"`
		Comments          []prComment   `json:"comments"`
		Reviews           []prReview    `json:"reviews"`
	}

	if err := json.Unmarshal(out, &prList); err != nil {
		return nil, fmt.Errorf("parsing gh pr list output: %w", err)
	}

	result := make([]models.PRInfo, 0, len(prList))
	for i := range prList {
		pr := &prList[i]
		result = append(result, models.PRInfo{
			Number:         pr.Number,
			Title:          pr.Title,
			State:          pr.State,
			URL:            pr.URL,
			IsDraft:        pr.IsDraft,
			HeadRef:        pr.HeadRefName,
			HeadRepoOwner:  pr.HeadRepositoryOwner.Login,
			BaseRef:        pr.BaseRefName,
			ReviewDecision: pr.ReviewDecision,
			Checks:         parseChecks(pr.StatusCheckRollup),
			Activity:       latestActivity(pr.Comments, pr.Reviews),
		})
	}

	return result, nil
}

type prReview struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	SubmittedAt string `json:"submittedAt"`
}

// latestActivity returns the most recent comment or review across both lists,
// or nil when the pull request has neither.
func latestActivity(comments []prComment, reviews []prReview) *models.PRActivity {
	var latest models.PRActivity

	for _, c := range comments {
		if at := parseTime(c.CreatedAt); at.After(latest.At) {
			latest = models.PRActivity{Author: c.Author.Login, At: at}
		}
	}

	for _, r := range reviews {
		if at := parseTime(r.SubmittedAt); at.After(latest.At) {
			latest = models.PRActivity{Author: r.Author.Login, At: at}
		}
	}

	if latest.At.IsZero() {
		return nil
	}

	return &latest
}

// GetPRCount returns the number of open pull requests for the repo, using the cache when fresh.
func GetPRCount(ctx context.Context, repoPath, remoteID, upstream string) (int, error) {
	prs, err := GetPRsForRepo(ctx, repoPath, remoteID, upstream)
	if err != nil {
		return 0, err
	}

	return len(prs), nil
}

type mergedPRHead struct {
	HeadRefName string `json:"headRefName"`
	HeadRefOid  string `json:"headRefOid"`
}

// CachedMergedPRHeads returns the cached merged-PR head map for the remote, if
// any, without invoking gh.
func CachedMergedPRHeads(repoPath, remoteID string) (map[string]string, bool) {
	key := MergedPRHeadsCacheKey(repoPath, remoteID)

	return cache.Persisted(cache.MergedPRHeadsCache, remoteID, key, vcs.Stamp(repoPath))
}

// GetMergedPRHeads returns merged pull requests' head branch name mapped to head commit OID for
// repoPath, using the cache when fresh. A branch whose tip matches one of these OIDs was
// squash-merged: `git branch --merged` won't catch it because the squash commit differs from the
// branch's own tip.
func GetMergedPRHeads(ctx context.Context, repoPath, remoteID string) (map[string]string, error) {
	if cached, ok := CachedMergedPRHeads(repoPath, remoteID); ok {
		return cached, nil
	}

	env := vcs.GetGitHubEnv(repoPath)

	out, err := runGH(ctx, repoPath, env, "pr", "list", "--state", "merged",
		"--json", "headRefName,headRefOid", "--limit", "100")
	if err != nil {
		return map[string]string{}, err
	}

	var prs []mergedPRHead
	if err := json.Unmarshal(out, &prs); err != nil {
		return map[string]string{}, fmt.Errorf("parsing gh pr list output: %w", err)
	}

	heads := make(map[string]string, len(prs))
	for _, pr := range prs {
		heads[pr.HeadRefName] = pr.HeadRefOid
	}

	cache.Persist(cache.MergedPRHeadsCache, remoteID, MergedPRHeadsCacheKey(repoPath, remoteID),
		vcs.Stamp(repoPath), heads)

	return heads, nil
}
