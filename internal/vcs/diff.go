package vcs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// headRef names the commit an uncommitted diff compares the working tree
// against.
const headRef = "HEAD"

// externalDiff overrides git's own diff.external for the patches this tool
// renders. Empty means the repo's git config decides.
var externalDiff string

// SetExternalDiffCommand replaces the external diff command, in the form git's
// diff.external takes. Intended for startup config application only; not safe
// to call concurrently with a diff read.
func SetExternalDiffCommand(command string) {
	externalDiff = command
}

// ExternalDiffCommand is the viewer a patch from repoPath renders through: the
// configured override, or whatever git itself would run for diff.external. It
// is empty when neither names one, and the caller then reads a plain patch.
func (g *GitOperations) ExternalDiffCommand(ctx context.Context, repoPath string) string {
	if externalDiff != "" {
		return externalDiff
	}

	//nolint:errcheck // an unset diff.external exits non-zero, which is the empty answer
	out, _ := g.runGit(ctx, repoPath, "config", "--get", "diff.external")

	return out
}

// StashDiffExternal is one stash's patch rendered by an external diff command
// rather than by git. A viewer running without a terminal assumes eighty
// columns and drops its color, so the environment says otherwise. A flag the
// command itself carries still wins over any of it.
func (*GitOperations) StashDiffExternal(
	ctx context.Context, repoPath string, index, width int, command string,
) (string, error) {
	cells := strconv.Itoa(max(width, 1))
	env := []string{
		"GIT_EXTERNAL_DIFF=" + command,
		"COLUMNS=" + cells,
		"CLICOLOR_FORCE=1",
		"DFT_COLOR=always",
		"DFT_DISPLAY=inline",
		"DFT_WIDTH=" + cells,
	}

	args := []string{"stash", "show", "--patch", "--ext-diff", fmt.Sprintf("stash@{%d}", index)}

	out, err := runCommandEnv(ctx, repoPath, env, "git", args...)
	if err != nil {
		return "", gitError(args, err)
	}

	return strings.TrimRight(out, "\n"), nil
}

// UncommittedDiffExternal is the working tree's patch against HEAD rendered by
// an external diff command rather than by git, for the same reasons
// StashDiffExternal sets its environment.
func (*GitOperations) UncommittedDiffExternal(
	ctx context.Context, repoPath string, width int, command string,
) (string, error) {
	cells := strconv.Itoa(max(width, 1))
	env := []string{
		"GIT_EXTERNAL_DIFF=" + command,
		"COLUMNS=" + cells,
		"CLICOLOR_FORCE=1",
		"DFT_COLOR=always",
		"DFT_DISPLAY=inline",
		"DFT_WIDTH=" + cells,
	}

	args := []string{"diff", headRef, "--ext-diff"}

	out, err := runCommandEnv(ctx, repoPath, env, "git", args...)
	if err != nil {
		return "", gitError(args, err)
	}

	return strings.TrimRight(out, "\n"), nil
}
