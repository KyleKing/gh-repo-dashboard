package filters

import "github.com/kyleking/gh-repo-dashboard/internal/models"

// PRAtoms returns the predicate atoms available over a pull request. Mergeable
// state and PR authorship aren't modeled here: Mergeable is GitHub's raw
// mergeStateStatus (CLEAN/DIRTY/BLOCKED/BEHIND/...), not a boolean, and no
// atom needs it yet; "mine" would need a locally resolved username, and the
// existing "Mine" saved view already covers that need server-side via @me.
func PRAtoms() map[string]Predicate[models.PRInfo] {
	return map[string]Predicate[models.PRInfo]{
		"draft":             func(p models.PRInfo) bool { return p.IsDraft },
		"approved":          func(p models.PRInfo) bool { return p.ReviewDecision == "APPROVED" },
		"changes_requested": func(p models.PRInfo) bool { return p.ReviewDecision == "CHANGES_REQUESTED" },
		"review_required":   func(p models.PRInfo) bool { return p.ReviewDecision == "REVIEW_REQUIRED" },
		"failing":           func(p models.PRInfo) bool { return p.Checks.Failing > 0 },
		"passing":           func(p models.PRInfo) bool { return p.Checks.Total > 0 && p.Checks.Failing == 0 },
	}
}
