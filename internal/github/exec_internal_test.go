package github

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

var errBoom = errors.New("boom")

// TestGhExecErrorSurfacesStderr pins the bug where a failed gh call reported
// only "exit status 1": (*exec.ExitError).Error() never includes Stderr, so
// the caller has to pull it out itself.
func TestGhExecErrorSurfacesStderr(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(context.Background(), "sh", "-c", "echo no git remotes found >&2; exit 1")
	_, err := cmd.Output()
	if err == nil {
		t.Fatal("expected the command to fail")
	}

	if got := ghExecError(err).Error(); !strings.Contains(got, "no git remotes found") {
		t.Errorf("expected the wrapped error to carry stderr, got %q", got)
	}
}

func TestGhExecErrorPassesThroughNonExitErrors(t *testing.T) {
	t.Parallel()

	if got := ghExecError(errBoom); !errors.Is(got, errBoom) {
		t.Errorf("expected a non-ExitError to pass through unchanged, got %v", got)
	}
}
