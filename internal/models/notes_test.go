package models_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestDetectNotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		files         map[string]string
		wantFile      string
		wantFirstLine string
	}{
		{name: "no notes file", files: nil, wantFile: "", wantFirstLine: ""},
		{
			name:          "doing.md only",
			files:         map[string]string{"doing.md": "Working on M8\nmore detail\n"},
			wantFile:      "doing.md",
			wantFirstLine: "Working on M8",
		},
		{
			name:          "TODO.md only",
			files:         map[string]string{"TODO.md": "Fix the bug\n"},
			wantFile:      "TODO.md",
			wantFirstLine: "Fix the bug",
		},
		{
			name: "priority order prefers .doing over doing.md",
			files: map[string]string{
				".doing":   "from dotfile",
				"doing.md": "from markdown",
			},
			wantFile:      ".doing",
			wantFirstLine: "from dotfile",
		},
		{
			name: "priority order prefers doing.md over doing.txt",
			files: map[string]string{
				"doing.md":  "from markdown",
				"doing.txt": "from txt",
			},
			wantFile:      "doing.md",
			wantFirstLine: "from markdown",
		},
		{
			name:          "skips leading blank lines",
			files:         map[string]string{"doing.md": "\n\n  \nActual first line\nsecond\n"},
			wantFile:      "doing.md",
			wantFirstLine: "Actual first line",
		},
		{
			name:          "empty file yields empty first line",
			files:         map[string]string{"doing.md": ""},
			wantFile:      "doing.md",
			wantFirstLine: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for name, content := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatalf("writing fixture file: %v", err)
				}
			}

			gotFile, gotFirstLine := models.DetectNotes(dir)
			if gotFile != tt.wantFile {
				t.Errorf("file = %q; want %q", gotFile, tt.wantFile)
			}
			if gotFirstLine != tt.wantFirstLine {
				t.Errorf("firstLine = %q; want %q", gotFirstLine, tt.wantFirstLine)
			}
		})
	}
}

func TestReadNotesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "line one\nline two\n"
	if err := os.WriteFile(filepath.Join(dir, "doing.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}

	tests := []struct {
		name      string
		notesFile string
		want      string
	}{
		{name: "no notes file", notesFile: "", want: ""},
		{name: "missing file", notesFile: "doing.txt", want: ""},
		{name: "existing file", notesFile: "doing.md", want: content},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := models.ReadNotesFile(dir, tt.notesFile); got != tt.want {
				t.Errorf("ReadNotesFile() = %q; want %q", got, tt.want)
			}
		})
	}
}
