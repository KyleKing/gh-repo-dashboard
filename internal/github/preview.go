package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
)

// previewFields are the cheapest set that still answers "is this worth
// opening": the description, who owes a review, and how big the diff is.
// Asking for comments or statusCheckRollup costs a second GraphQL traversal
// per row, which is what made the inline preview feel stuck while the cursor
// moved.
const previewFields = "body,reviewRequests,reviewDecision,additions,deletions"

// PreviewTimeout bounds one preview read. A preview is a glance, so a gh
// invocation that outlives this is reported as a failure rather than left
// pending forever.
const PreviewTimeout = 10 * time.Second

// PRPreviewCacheKey keys a preview on the pull request's own URL, which
// identifies it across every repository a fleet-wide search reaches.
func PRPreviewCacheKey(prURL string) string {
	return "pr_preview\x00" + prURL
}

// GetPRPreview reads the little a PRs-tab row shows inline. It addresses the
// pull request by URL, so a row from a repository that was never scanned
// locally previews the same as one that was; repoPath and its environment
// only supply gh's credentials and can be any repo in the fleet.
func GetPRPreview(ctx context.Context, repoPath, prURL string) (*forge.PRPreview, error) {
	key := PRPreviewCacheKey(prURL)
	if cached, ok := cache.PRPreviewCache.Get(key, cache.NoStamp); ok {
		return cached, nil
	}

	ctx, cancel := context.WithTimeout(ctx, PreviewTimeout)
	defer cancel()

	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "pr", "view", prURL, "--json", previewFields)
	if err != nil {
		return nil, err
	}

	//nolint:tagliatelle // gh speaks camelCase, and these tags name its fields
	var resp struct {
		Body           string `json:"body"`
		ReviewDecision string `json:"reviewDecision"`
		Additions      int    `json:"additions"`
		Deletions      int    `json:"deletions"`
		ReviewRequests []struct {
			Login string `json:"login"`
		} `json:"reviewRequests"`
	}

	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parsing gh pr view output: %w", err)
	}

	reviewers := make([]string, 0, len(resp.ReviewRequests))
	for _, r := range resp.ReviewRequests {
		reviewers = append(reviewers, r.Login)
	}

	preview := &forge.PRPreview{
		Body:           resp.Body,
		Reviewers:      reviewers,
		ReviewDecision: resp.ReviewDecision,
		Additions:      resp.Additions,
		Deletions:      resp.Deletions,
	}

	cache.PRPreviewCache.Set(key, cache.NoStamp, preview)

	return preview, nil
}
