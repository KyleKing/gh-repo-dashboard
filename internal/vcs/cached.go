package vcs

import (
	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// cachedBranchList serves the checkout's branch list while the stamp proves it
// unchanged, however long ago it was read.
func cachedBranchList(repoPath string, read func() ([]models.BranchInfo, error)) ([]models.BranchInfo, error) {
	return stamped(cache.BranchCache, cache.BranchCacheKey(repoPath), repoPath, read)
}

// cachedCommitLog serves the checkout's commit log under the same rule.
func cachedCommitLog(
	repoPath string, count int, read func() ([]models.CommitInfo, error),
) ([]models.CommitInfo, error) {
	return stamped(cache.CommitCache, cache.CommitCacheKey(repoPath, count), repoPath, read)
}

// stamped reads through cache c, skipping it entirely for a checkout that
// could not be stamped: a local value nothing can prove fresh must not be
// served from a timer.
//
//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
func stamped[T any](c *cache.TTLCache[T], key, repoPath string, read func() (T, error)) (T, error) {
	stamp := Stamp(repoPath)
	if cached, ok := c.Fresh(key, stamp); ok {
		return cached, nil
	}

	value, err := read()
	if err != nil {
		var zero T

		return zero, err
	}

	if stamp.Fingerprint != "" {
		c.Set(key, stamp, value)
	}

	return value, nil
}
