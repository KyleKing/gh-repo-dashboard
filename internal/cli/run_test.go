package cli_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/cli"
	"github.com/kyleking/gh-repo-dashboard/internal/config"
	"github.com/kyleking/gh-repo-dashboard/internal/tasks"
)

func TestRunTasks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"ok", "bad"} {
		dir := filepath.Join(root, name)
		runGitCmd(t, "", "init", "-q", dir)
		runGitCmd(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
	}
	defined := []config.Task{
		{Name: "check", Run: `[ "$REPO_NAME" = ok ]`},
		{Name: "guarded", Run: "true", Confirm: true},
	}

	tests := []struct {
		name        string
		opts        cli.RunOptions
		input       string
		wantErr     error
		wantOutput  []string
		wantMissing []string
	}{
		{
			name:       "a failing repo fails the run",
			opts:       cli.RunOptions{Task: "check"},
			wantErr:    cli.ErrTasksFailed,
			wantOutput: []string{"ok  ", "failed", "bad"},
		},
		{name: "unknown task", opts: cli.RunOptions{Task: "nope"}, wantErr: cli.ErrUnknownTask},
		{name: "no task", opts: cli.RunOptions{}, wantErr: cli.ErrNoTask},
		{
			name:        "dry run lists repos and runs nothing",
			opts:        cli.RunOptions{Task: "check", DryRun: true, Where: "clean"},
			wantOutput:  []string{"would run on 2 repos", "bad", "ok"},
			wantMissing: []string{"STATUS"},
		},
		{
			name:       "confirm declined by default",
			opts:       cli.RunOptions{Task: "guarded"},
			wantOutput: []string{"skipped"},
		},
		{
			name:       "confirm accepted",
			opts:       cli.RunOptions{Task: "guarded"},
			input:      "y\n",
			wantOutput: []string{"STATUS"},
		},
		{
			name:       "due runs tasks with a schedule only",
			opts:       cli.RunOptions{Due: true},
			wantOutput: []string{"no repos to run on"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.opts.MaxDepth = 1
			tt.opts.Tasks = defined
			tt.opts.Store = tasks.NewStore(t.TempDir())

			var out bytes.Buffer
			err := cli.RunTasks(t.Context(), &out, strings.NewReader(tt.input), []string{root}, tt.opts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v; want %v", err, tt.wantErr)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output missing %q:\n%s", want, out.String())
				}
			}
			for _, unwanted := range tt.wantMissing {
				if strings.Contains(out.String(), unwanted) {
					t.Errorf("output has %q:\n%s", unwanted, out.String())
				}
			}
		})
	}
}
