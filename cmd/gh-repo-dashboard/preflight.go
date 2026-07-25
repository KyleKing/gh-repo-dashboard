package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

var errNoVCS = errors.New(
	"neither git nor jj was found on PATH; install git (https://git-scm.com) or jj (https://jj-vcs.github.io)")

const ghAuthTimeout = 2 * time.Second

func notice(warn io.Writer, msg string) {
	fmt.Fprintln(warn, "Note: "+msg) //nolint:errcheck // best-effort stderr notice
}

// preflight verifies the CLI tools the dashboard shells out to, writing a
// one-line notice per degraded feature. It only errors when no VCS binary is
// available at all.
func preflight(ctx context.Context, warn io.Writer) error {
	_, gitErr := exec.LookPath("git")
	_, jjErr := exec.LookPath("jj")
	if gitErr != nil && jjErr != nil {
		return errNoVCS
	}
	if gitErr != nil {
		notice(warn, "git not found on PATH; git repositories will show errors.")
	}
	if jjErr != nil {
		notice(warn, "jj not found on PATH; colocated repositories fall back to git"+
			" and jj-only repositories will show errors.")
	}

	if _, err := exec.LookPath("gh"); err != nil {
		notice(warn, "gh not found on PATH; PR and workflow columns will be blank."+
			" Install https://cli.github.com and run 'gh auth login'.")

		return nil
	}

	authCtx, cancel := context.WithTimeout(ctx, ghAuthTimeout)
	defer cancel()
	if err := exec.CommandContext(authCtx, "gh", "auth", "status").Run(); err != nil && authCtx.Err() == nil {
		notice(warn, "gh is not authenticated; PR and workflow columns will be blank. Run 'gh auth login'.")
	}

	return nil
}
