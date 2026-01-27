package vcs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestDetectVCSType(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) error
		expected models.VCSType
	}{
		{
			name:     "git repo",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0755) },
			expected: models.VCSTypeGit,
		},
		{
			name:     "jj repo",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".jj"), 0755) },
			expected: models.VCSTypeJJ,
		},
		{
			name: "colocated prefers jj",
			setup: func(dir string) error {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
					return err
				}
				return os.Mkdir(filepath.Join(dir, ".jj"), 0755)
			},
			expected: models.VCSTypeJJ,
		},
		{
			name:     "empty dir defaults to git",
			setup:    func(dir string) error { return nil },
			expected: models.VCSTypeGit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}

			result := DetectVCSType(dir)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetOperations(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) error
		expectedVCS models.VCSType
	}{
		{
			name:        "returns git ops for git repo",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0755) },
			expectedVCS: models.VCSTypeGit,
		},
		{
			name:        "returns jj ops for jj repo",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".jj"), 0755) },
			expectedVCS: models.VCSTypeJJ,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}

			ops := GetOperations(dir)
			if ops.VCSType() != tt.expectedVCS {
				t.Errorf("expected %v, got %v", tt.expectedVCS, ops.VCSType())
			}
		})
	}
}

func TestIsRepo(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) error
		expected bool
	}{
		{
			name:     "git repo",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0755) },
			expected: true,
		},
		{
			name:     "jj repo",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".jj"), 0755) },
			expected: true,
		},
		{
			name:     "not a repo",
			setup:    func(dir string) error { return nil },
			expected: false,
		},
		{
			name:     "has other dot dirs but not vcs",
			setup:    func(dir string) error { return os.Mkdir(filepath.Join(dir, ".config"), 0755) },
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}

			result := IsRepo(dir)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetGitHubEnv(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(dir string) error
		expectEmpty bool
	}{
		{
			name:        "git repo returns nil",
			setup:       func(dir string) error { return os.Mkdir(filepath.Join(dir, ".git"), 0755) },
			expectEmpty: true,
		},
		{
			name: "colocated jj repo returns nil",
			setup: func(dir string) error {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
					return err
				}
				return os.Mkdir(filepath.Join(dir, ".jj"), 0755)
			},
			expectEmpty: true,
		},
		{
			name: "non-colocated jj repo sets GIT_DIR",
			setup: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, ".jj", "repo", "store", "git"), 0755)
			},
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := tt.setup(dir); err != nil {
				t.Fatal(err)
			}

			env := GetGitHubEnv(dir)
			if tt.expectEmpty && len(env) > 0 {
				t.Errorf("expected empty env, got %v", env)
			}
			if !tt.expectEmpty {
				if len(env) == 0 {
					t.Error("expected GIT_DIR env var")
				} else if env[0] != "GIT_DIR="+filepath.Join(dir, ".jj", "repo", "store", "git") {
					t.Errorf("unexpected env: %v", env)
				}
			}
		})
	}
}
