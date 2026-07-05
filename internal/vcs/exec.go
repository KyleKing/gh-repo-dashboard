// Package vcs abstracts git and jj repository operations behind a common Operations interface.
package vcs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type commandRunner func(ctx context.Context, dir, name string, args ...string) (string, error)

type commandRunnerKey struct{}

// withCommandRunner returns a context that makes runCommand call fn instead of
// executing a real subprocess. Used by tests to stub git/jj invocations
// without touching shared package state, so subtests can run in parallel.
func withCommandRunner(ctx context.Context, fn commandRunner) context.Context {
	return context.WithValue(ctx, commandRunnerKey{}, fn)
}

func runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	if fn, ok := ctx.Value(commandRunnerKey{}).(commandRunner); ok {
		return fn(ctx, dir, name, args...)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running %s: %w", name, err)
	}

	return strings.TrimSpace(string(out)), nil
}
