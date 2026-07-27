// Package copier reads a repo's .copier-answers.yml and compares its
// installed template commit against the template's latest upstream semver
// tag.
package copier

import (
	"context"
	"fmt"
	"os/exec"
)

type lsRemoteRunner func(ctx context.Context, srcPath string) ([]byte, error)

type lsRemoteRunnerKey struct{}

// withLsRemoteRunner returns a context that makes runLsRemote call fn instead
// of executing a real git subprocess. Used by tests to stub the network call
// without touching shared package state.
func withLsRemoteRunner(ctx context.Context, fn lsRemoteRunner) context.Context {
	return context.WithValue(ctx, lsRemoteRunnerKey{}, fn)
}

// runLsRemote lists a remote's tag refs via `git ls-remote --tags --refs`.
// The given path may be a remote URL or a local filesystem path, both
// understood directly by git.
func runLsRemote(ctx context.Context, srcPath string) ([]byte, error) {
	if fn, ok := ctx.Value(lsRemoteRunnerKey{}).(lsRemoteRunner); ok {
		return fn(ctx, srcPath)
	}

	// #nosec G204 -- fixed git subcommand, srcPath from local template metadata
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", srcPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running git ls-remote: %w", err)
	}

	return out, nil
}
