package github

import (
	"context"
	"fmt"
	"os/exec"
)

type ghRunner func(ctx context.Context, dir string, env []string, args ...string) ([]byte, error)

type ghRunnerKey struct{}

// withGHRunner returns a context that makes runGH call fn instead of executing
// a real gh subprocess. Used by tests to stub gh invocations without touching
// shared package state, so subtests can run in parallel.
func withGHRunner(ctx context.Context, fn ghRunner) context.Context {
	return context.WithValue(ctx, ghRunnerKey{}, fn)
}

func runGH(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	if fn, ok := ctx.Value(ghRunnerKey{}).(ghRunner); ok {
		return fn(ctx, dir, env, args...)
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running gh: %w", err)
	}

	return out, nil
}
