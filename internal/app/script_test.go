//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"context"
	"strings"
	"testing"
)

func TestRunScriptLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		line       string
		want       lineOutcome
		wantOutput string
	}{
		{name: "filter", line: ":filter dirty", wantOutput: "2 repos visible"},
		{name: "status message", line: "select where has_pr", wantOutput: "Selected 1 repos"},
		{
			name: "unknown command", line: "bogus",
			want: lineOutcome{failed: true}, wantOutput: "Unknown command: bogus",
		},
		{
			name: "unparsable predicate", line: "filter dirty and",
			want: lineOutcome{failed: true}, wantOutput: "and",
		},
		{
			name: "unknown sort", line: "sort bogus",
			want: lineOutcome{failed: true}, wantOutput: "Unknown sort: bogus",
		},
		{name: "quit", line: "quit", want: lineOutcome{quit: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out strings.Builder
			_, got := runScriptLine(operatorModel(), tt.line, &out)
			if got != tt.want {
				t.Errorf("runScriptLine(%q) = %+v; want %+v", tt.line, got, tt.want)
			}
			if !strings.Contains(out.String(), tt.wantOutput) {
				t.Errorf("output = %q; want it to contain %q", out.String(), tt.wantOutput)
			}
		})
	}
}

func TestRunScriptLineFilterApplies(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	m, _ := runScriptLine(operatorModel(), ":filter dirty", &out)
	if len(m.filteredPaths) != 2 {
		t.Errorf("filtered = %v; want 2 dirty repos", m.filteredPaths)
	}
}

// runScriptIn runs script over an empty directory, where the commands still
// parse and dispatch but no repo work happens.
func runScriptIn(t *testing.T, script string) (string, error) {
	t.Helper()

	var out strings.Builder
	err := RunScript(context.Background(), &out, []string{t.TempDir()}, 1, strings.NewReader(script))

	return out.String(), err
}

func TestRunScriptReportsFailedLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		script  string
		wantErr bool
		wantOut string
	}{
		{name: "clean run", script: "# comment\n\n:filter all\n", wantOut: "repos visible"},
		{name: "unknown command", script: ":bogus\n", wantErr: true, wantOut: "Unknown command: bogus"},
		{name: "bad predicate", script: ":filter dirty and\n", wantErr: true},
		{
			name:    "a failure does not stop the remaining lines",
			script:  ":bogus\n:filter all\n",
			wantErr: true,
			wantOut: "repos visible",
		},
		{name: "quit stops the script", script: ":quit\n:bogus\n", wantOut: "> :quit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := runScriptIn(t, tt.script)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RunScript(%q) error = %v; wantErr %v", tt.script, err, tt.wantErr)
			}
			if !strings.Contains(out, tt.wantOut) {
				t.Errorf("output = %q; want it to contain %q", out, tt.wantOut)
			}
		})
	}
}

func TestRunScriptHelpListsCommands(t *testing.T) {
	t.Parallel()

	out, err := runScriptIn(t, ":help\n")
	if err != nil {
		t.Fatalf("RunScript(:help) errored: %v", err)
	}
	for _, name := range []string{"filter", "help", "quit", "select", "sort"} {
		if !strings.Contains(out, "\n"+name+"\t") && !strings.Contains(out, "  "+name+"\t") {
			t.Errorf("output = %q; want a line for the %q command", out, name)
		}
	}
}
