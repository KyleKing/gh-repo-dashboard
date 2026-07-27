package copier

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
)

// isSemverTag reports whether commit parses as a semver version (with or
// without a leading "v"), as opposed to a raw commit SHA or branch name.
func isSemverTag(commit string) bool {
	_, err := semver.NewVersion(commit)
	return err == nil
}

// abbreviationURLs maps copier's built-in _src_path shorthand prefixes
// (e.g. "gh:owner/repo") to the URL template git can clone directly.
var abbreviationURLs = map[string]string{
	"gh": "https://github.com/%s.git",
	"gl": "https://gitlab.com/%s.git",
}

// resolveSrcPath expands a copier abbreviation like "gh:owner/repo" into a
// full git URL. Anything else (a full URL or a local path) passes through
// unchanged.
func resolveSrcPath(srcPath string) string {
	prefix, rest, hasPrefix := strings.Cut(srcPath, ":")
	if !hasPrefix {
		return srcPath
	}

	tmpl, known := abbreviationURLs[prefix]
	if !known {
		return srcPath
	}

	return fmt.Sprintf(tmpl, rest)
}

// isBehind reports whether latestTag is a strictly newer semver version than
// commit. Both must already be known-good semver tags.
func isBehind(commit, latestTag string) bool {
	current, err := semver.NewVersion(commit)
	if err != nil {
		return false
	}

	latest, err := semver.NewVersion(latestTag)
	if err != nil {
		return false
	}

	return latest.GreaterThan(current)
}

// latestSemverTag returns the newest semver-parseable tag published at
// srcPath, sharing a cache entry across every repo generated from the same
// template. The bool result is false when the lookup failed or the remote
// has no semver-parseable tags.
func latestSemverTag(ctx context.Context, srcPath string) (string, bool) {
	if cached, hit := cache.CopierLatestTagCache.Get(srcPath); hit {
		return cached, true
	}

	out, err := runLsRemote(ctx, srcPath)
	if err != nil {
		return "", false
	}

	tag, ok := newestSemverTagFromLsRemote(string(out))
	if ok {
		cache.CopierLatestTagCache.Set(srcPath, tag)
	}

	return tag, ok
}

const lsRemoteLineFields = 2 // ls-remote lines are "<sha>\t<ref>"

// newestSemverTagFromLsRemote picks the highest semver-parseable tag out of
// `git ls-remote --tags --refs` output, ignoring any ref line that isn't a
// valid semver tag.
func newestSemverTagFromLsRemote(output string) (string, bool) {
	var best *semver.Version

	var tag string

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != lsRemoteLineFields {
			continue
		}

		name, isTagRef := strings.CutPrefix(fields[1], "refs/tags/")
		if !isTagRef {
			continue
		}

		v, err := semver.NewVersion(name)
		if err != nil {
			continue
		}

		if best == nil || v.GreaterThan(best) {
			best = v
			tag = name
		}
	}

	return tag, best != nil
}
