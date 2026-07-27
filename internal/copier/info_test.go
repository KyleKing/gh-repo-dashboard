package copier_test

import (
	"context"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
	"github.com/kyleking/gh-repo-dashboard/internal/copier"
)

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetTemplateInfoNoAnswersFile(t *testing.T) {
	cache.ClearAll()

	info, err := copier.GetTemplateInfo(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil info for a non-copier repo, got %+v", info)
	}
}

//nolint:paralleltest // asserts against shared global cache.ClearAll() state
func TestGetTemplateInfoNoSrcPathSkipsNetwork(t *testing.T) {
	cache.ClearAll()

	dir := t.TempDir()
	writeFile(t, dir, ".copier-answers.yml", "_commit: v1.0.0\n")

	ctx, calls := stubLsRemote(nil, nil)

	info, err := copier.GetTemplateInfo(ctx, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil || info.Commit != "v1.0.0" || !info.IsTag {
		t.Fatalf("expected tag info with commit v1.0.0, got %+v", info)
	}
	if info.LatestTag != "" {
		t.Errorf("expected no latest tag without a src path, got %q", info.LatestTag)
	}
	if *calls != 0 {
		t.Errorf("expected no ls-remote invocation without a src path, got %d", *calls)
	}
}
