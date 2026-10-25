package models

import (
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

// RepoSummary is a repo list row: what the checkout says about itself, plus
// what the dashboard hangs off it.
type RepoSummary struct {
	vcs.RepoSummary
	// PRInfo is the pull request on the checked-out branch, and OpenPRs every open pull request in the repo.
	PRInfo       *forge.PullRequest
	OpenPRs      []forge.PullRequest
	CI           *forge.DefaultBranchCI
	TemplateInfo *CopierTemplateInfo
	NotesFiles   []NoteFile
	Loading      bool
	Error        error
}

// HasNotes reports whether any notes file was detected at the repo's root.
func (r RepoSummary) HasNotes() bool {
	return len(r.NotesFiles) > 0
}

// CIFailing counts the default branch's failing workflows, then all of them.
func (r RepoSummary) CIFailing() (int, int) {
	if r.CI == nil {
		return 0, 0
	}

	failing := 0

	for i := range r.CI.Workflows {
		if r.CI.Workflows[i].Conclusion == CIFailureConclusion {
			failing++
		}
	}

	return failing, len(r.CI.Workflows)
}

// CIRunning counts the default-branch workflows that have not finished.
func (r RepoSummary) CIRunning() int {
	if r.CI == nil {
		return 0
	}

	running := 0

	for i := range r.CI.Workflows {
		if r.CI.Workflows[i].Status != forge.StatusCompleted {
			running++
		}
	}

	return running
}

// CIFailureConclusion is the conclusion GitHub reports for a failed workflow run.
const CIFailureConclusion = "failure"
