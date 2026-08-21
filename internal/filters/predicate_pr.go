package filters

import (
	"github.com/kyleking/aragonite/forge"
)

// PRAtoms returns the predicate atoms available over a pull request. Mergeable
// state and PR authorship aren't modeled here: Mergeable is GitHub's raw
// mergeStateStatus (CLEAN/DIRTY/BLOCKED/BEHIND/...), not a boolean, and no
// atom needs it yet; "mine" would need a locally resolved username, and the
// existing "Mine" saved view already covers that need server-side via @me.
func PRAtoms() map[string]Predicate[forge.PullRequest] {
	return map[string]Predicate[forge.PullRequest]{
		"draft":             func(p forge.PullRequest) bool { return p.IsDraft },
		"approved":          func(p forge.PullRequest) bool { return p.ReviewDecision == "APPROVED" },
		"changes_requested": func(p forge.PullRequest) bool { return p.ReviewDecision == "CHANGES_REQUESTED" },
		"review_required":   func(p forge.PullRequest) bool { return p.ReviewDecision == "REVIEW_REQUIRED" },
		"failing":           func(p forge.PullRequest) bool { return p.Checks.Failing > 0 },
		"passing":           func(p forge.PullRequest) bool { return p.Checks.Total > 0 && p.Checks.Failing == 0 },
		"needs_reviewer":    forge.PullRequest.NeedsReviewer,
	}
}

// PRAtomDescriptions gives each PRAtoms() name a one-line explanation, for the
// command bar's completion candidates.
func PRAtomDescriptions() map[string]string {
	return map[string]string{
		"draft":             "pull request is a draft",
		"approved":          "review decision is approved",
		"changes_requested": "review decision is changes requested",
		"review_required":   "a review is required and still pending",
		"failing":           "at least one CI check is failing",
		"passing":           "every CI check is passing",
		"needs_reviewer":    "open, not draft, and nobody is currently requested to review it",
	}
}
