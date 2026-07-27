package copier

import (
	"context"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// networkTimeout bounds the git ls-remote lookup so a stale or unreachable
// _src_path can't stall the repo's async load.
const networkTimeout = 5 * time.Second

// GetTemplateInfo reads repoPath's copier answers file and, when the
// installed commit is a semver tag, compares it against the template's
// latest upstream tag. It returns (nil, nil) for repos with no
// .copier-answers.yml. The latest-tag lookup is best-effort: a network
// failure leaves LatestTag empty rather than failing the whole call.
func GetTemplateInfo(ctx context.Context, repoPath string) (*models.CopierTemplateInfo, error) {
	answers, err := ReadAnswersFile(repoPath)
	if err != nil {
		return nil, err
	}
	if answers == nil {
		return nil, nil //nolint:nilnil // absence isn't an error: most repos aren't copier-generated
	}

	info := &models.CopierTemplateInfo{
		SrcPath: answers.SrcPath,
		Commit:  answers.Commit,
		IsTag:   isSemverTag(answers.Commit),
	}

	if info.IsTag && answers.SrcPath != "" {
		lookupCtx, cancel := context.WithTimeout(ctx, networkTimeout)
		defer cancel()

		if latest, ok := latestSemverTag(lookupCtx, resolveSrcPath(answers.SrcPath)); ok {
			info.LatestTag = latest
			info.Behind = isBehind(answers.Commit, latest)
		}
	}

	return info, nil
}
