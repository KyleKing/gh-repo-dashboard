package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeBinDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		script := []byte("#!/bin/sh\nexit 0\n")
		//nolint:gosec // test stub must be executable
		if err := os.WriteFile(filepath.Join(dir, name), script, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestPreflight(t *testing.T) {
	tests := []struct {
		name         string
		binaries     []string
		expectErr    bool
		expectNotice string
	}{
		{"all tools present", []string{"git", "jj", "gh"}, false, ""},
		{"no vcs at all", nil, true, ""},
		{"git only warns about jj and gh", []string{"git"}, false, "jj not found"},
		{"missing gh warns", []string{"git", "jj"}, false, "gh not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PATH", fakeBinDir(t, tt.binaries...))

			var buf strings.Builder
			err := preflight(context.Background(), &buf)
			if tt.expectErr {
				if !errors.Is(err, errNoVCS) {
					t.Fatalf("preflight error = %v; want errNoVCS", err)
				}

				return
			}
			if err != nil {
				t.Fatalf("preflight error = %v; want nil", err)
			}
			if tt.expectNotice == "" && buf.Len() > 0 {
				t.Errorf("expected no notices, got %q", buf.String())
			}
			if tt.expectNotice != "" && !strings.Contains(buf.String(), tt.expectNotice) {
				t.Errorf("notices %q missing %q", buf.String(), tt.expectNotice)
			}
		})
	}
}
