package models

import (
	"context"
	"fmt"

	"github.com/kyleking/aragonite/vcs"
)

// ReadSummary reads path's summary through ops and attaches the notes files
// sitting beside it. The error is returned alongside a usable summary rather
// than instead of one, because a repo that cannot be read still gets a row, and
// every caller renders that row from the same three fields. The summary's own
// Error stays unwrapped, since it is rendered into a single table cell.
func ReadSummary(ctx context.Context, ops vcs.StatusReader, path string) (RepoSummary, error) {
	summary, err := ops.GetRepoSummary(ctx, path)
	if err != nil {
		return RepoSummary{
			RepoSummary: vcs.RepoSummary{Path: path, VCSType: ops.VCSType()},
			Error:       err,
		}, fmt.Errorf("reading %s: %w", path, err)
	}

	return RepoSummary{RepoSummary: summary, NotesFiles: DetectNotes(path)}, nil
}
