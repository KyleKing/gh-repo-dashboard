package discovery_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
)

func writeMani(t *testing.T, body string, dirs ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(root, discovery.ManiFilename)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestManiPathsResolvesProjectsRelativeToTheFile(t *testing.T) {
	t.Parallel()

	path := writeMani(t, `
projects:
  beta:
    url: git@github.com:acme/beta.git
  alpha:
    url: git@github.com:acme/alpha.git
`, "alpha", "beta")

	got, err := discovery.ManiPaths(path)
	if err != nil {
		t.Fatalf("ManiPaths: %v", err)
	}

	root := filepath.Dir(path)
	want := []string{filepath.Join(root, "alpha"), filepath.Join(root, "beta")}

	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("roster = %v, want %v sorted", got, want)
	}
}

func TestManiPathsHonoursAnExplicitPath(t *testing.T) {
	t.Parallel()

	path := writeMani(t, `
projects:
  alpha:
    path: nested/elsewhere
`, "nested/elsewhere")

	got, err := discovery.ManiPaths(path)
	if err != nil {
		t.Fatalf("ManiPaths: %v", err)
	}

	want := filepath.Join(filepath.Dir(path), "nested", "elsewhere")
	if len(got) != 1 || got[0] != want {
		t.Errorf("roster = %v, want [%s]", got, want)
	}
}

func TestManiPathsSkipsProjectsNotOnDisk(t *testing.T) {
	t.Parallel()

	path := writeMani(t, `
projects:
  present:
    url: git@github.com:acme/present.git
  cloned-elsewhere:
    url: git@github.com:acme/cloned-elsewhere.git
`, "present")

	got, err := discovery.ManiPaths(path)
	if err != nil {
		t.Fatalf("ManiPaths: %v", err)
	}

	if len(got) != 1 || filepath.Base(got[0]) != "present" {
		t.Errorf("roster = %v, want only the project that exists locally", got)
	}
}

func TestManiPathsReportsAnUnreadableRoster(t *testing.T) {
	t.Parallel()

	if _, err := discovery.ManiPaths(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Error("expected an error for a missing roster")
	}

	path := writeMani(t, "projects: [this is not a map]")
	if _, err := discovery.ManiPaths(path); err == nil {
		t.Error("expected an error for a malformed roster")
	}
}
