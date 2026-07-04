package vcs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

var runCommand = func(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running %s: %w", name, err)
	}

	return strings.TrimSpace(string(out)), nil
}
