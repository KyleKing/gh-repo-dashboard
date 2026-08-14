package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
)

var errNoVCS = errors.New(
	"neither git nor jj was found on PATH; install git (https://git-scm.com) or jj (https://jj-vcs.github.io)")

func notice(warn io.Writer, msg string) {
	fmt.Fprintln(warn, "Note: "+msg) //nolint:errcheck // best-effort stderr notice
}

// preflight verifies the VCS binaries the dashboard shells out to, writing a
// one-line notice per degraded one, and erroring only when neither is
// available at all. The gh CLI's own presence and auth state are checked once
// the TUI is already running (see checkGHAuthCmd): "gh auth status" is a
// network call that can take the better part of a second, and gh's PR and
// workflow reads already degrade to a dash on their own when it fails, so
// there is nothing to gain by holding the first frame on it.
func preflight(warn io.Writer) error {
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

	return nil
}
