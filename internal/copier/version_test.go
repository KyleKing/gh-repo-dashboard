package copier_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kyleking/aragonite/cache"

	"github.com/kyleking/gh-repo-dashboard/internal/copier"
)

var errLsRemoteFailed = errors.New("ls-remote failed")

// stubLsRemote returns a context that makes runLsRemote answer with (out,
// err) instead of shelling out, plus a call counter.
func stubLsRemote(out []byte, err error) (context.Context, *int) {
	calls := 0
	stub := func(_ context.Context, _ string) ([]byte, error) {
		calls++
		return out, err
	}

	return copier.WithLsRemoteRunner(context.Background(), stub), &calls
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetTemplateInfoLatestTag(t *testing.T) {
	lsRemoteOutput := []byte(
		"aaa\trefs/tags/v1.0.0\n" +
			"bbb\trefs/tags/v1.2.0\n" +
			"ccc\trefs/tags/not-semver\n" +
			"ddd\trefs/tags/v1.1.0\n",
	)

	tests := []struct {
		name          string
		commit        string
		lsRemoteOut   []byte
		lsRemoteErr   error
		expectLatest  string
		expectBehind  bool
		expectNetwork bool
	}{
		{
			name:          "tag behind latest",
			commit:        "v1.0.0",
			lsRemoteOut:   lsRemoteOutput,
			expectLatest:  "v1.2.0",
			expectBehind:  true,
			expectNetwork: true,
		},
		{
			name:          "tag already latest",
			commit:        "v1.2.0",
			lsRemoteOut:   lsRemoteOutput,
			expectLatest:  "v1.2.0",
			expectBehind:  false,
			expectNetwork: true,
		},
		{
			name:          "commit-pinned, not a tag",
			commit:        "abc1234",
			lsRemoteOut:   lsRemoteOutput,
			expectLatest:  "",
			expectBehind:  false,
			expectNetwork: false,
		},
		{
			name:          "network failure degrades gracefully",
			commit:        "v1.0.0",
			lsRemoteErr:   errLsRemoteFailed,
			expectLatest:  "",
			expectBehind:  false,
			expectNetwork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			dir := t.TempDir()
			writeAnswersFile(t, dir, "https://example.com/tmpl.git", tt.commit)

			ctx, calls := stubLsRemote(tt.lsRemoteOut, tt.lsRemoteErr)

			info, err := copier.GetTemplateInfo(ctx, dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if info.LatestTag != tt.expectLatest {
				t.Errorf("expected latest tag %q, got %q", tt.expectLatest, info.LatestTag)
			}
			if info.Behind != tt.expectBehind {
				t.Errorf("expected behind=%v, got %v", tt.expectBehind, info.Behind)
			}

			expectedCalls := 0
			if tt.expectNetwork {
				expectedCalls = 1
			}
			if *calls != expectedCalls {
				t.Errorf("expected %d ls-remote invocations, got %d", expectedCalls, *calls)
			}
		})
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetTemplateInfoSharesCacheAcrossRepos(t *testing.T) {
	cache.ClearAll()

	const srcPath = "https://example.com/shared-tmpl.git"

	dirA := t.TempDir()
	writeAnswersFile(t, dirA, srcPath, "v1.0.0")
	dirB := t.TempDir()
	writeAnswersFile(t, dirB, srcPath, "v1.0.0")

	ctx, calls := stubLsRemote([]byte("aaa\trefs/tags/v1.1.0\n"), nil)

	if _, err := copier.GetTemplateInfo(ctx, dirA); err != nil {
		t.Fatalf("unexpected error for repo A: %v", err)
	}
	if _, err := copier.GetTemplateInfo(ctx, dirB); err != nil {
		t.Fatalf("unexpected error for repo B: %v", err)
	}

	if *calls != 1 {
		t.Errorf("expected 1 ls-remote invocation shared across repos, got %d", *calls)
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetTemplateInfoResolvesGitHubAbbreviation(t *testing.T) {
	tests := []struct {
		name        string
		srcPath     string
		expectedArg string
	}{
		{
			name:        "gh: abbreviation resolves to a cloneable URL",
			srcPath:     "gh:KyleKing/my_go_template",
			expectedArg: "https://github.com/KyleKing/my_go_template.git",
		},
		{
			name:        "gl: abbreviation resolves to a cloneable URL",
			srcPath:     "gl:KyleKing/my_go_template",
			expectedArg: "https://gitlab.com/KyleKing/my_go_template.git",
		},
		{
			name:        "full URL passes through unchanged",
			srcPath:     "https://example.com/tmpl.git",
			expectedArg: "https://example.com/tmpl.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.ClearAll()
			dir := t.TempDir()
			writeAnswersFile(t, dir, tt.srcPath, "v1.0.0")

			var gotArg string
			stub := func(_ context.Context, srcPath string) ([]byte, error) {
				gotArg = srcPath
				return []byte("aaa\trefs/tags/v1.0.0\n"), nil
			}
			ctx := copier.WithLsRemoteRunner(context.Background(), stub)

			if _, err := copier.GetTemplateInfo(ctx, dir); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotArg != tt.expectedArg {
				t.Errorf("expected ls-remote called with %q, got %q", tt.expectedArg, gotArg)
			}
		})
	}
}

func writeAnswersFile(t *testing.T, dir, srcPath, commit string) {
	t.Helper()

	content := "_src_path: " + srcPath + "\n_commit: " + commit + "\n"
	writeFile(t, dir, ".copier-answers.yml", content)
}
