package models

import (
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

// RepoSummary is a repo list row: what the checkout says about itself, plus
// what the dashboard hangs off it.
type RepoSummary struct {
	vcs.RepoSummary

	PRInfo       *forge.PullRequest
	WorkflowInfo *forge.WorkflowSummary
	TemplateInfo *CopierTemplateInfo
	NotesFiles   []NoteFile
	Loading      bool
	Error        error
}

// HasNotes reports whether any notes file was detected at the repo's root.
func (r RepoSummary) HasNotes() bool {
	return len(r.NotesFiles) > 0
}
