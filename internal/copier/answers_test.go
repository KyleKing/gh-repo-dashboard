package copier_test

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/copier"
)

func TestReadAnswersFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		writeFile bool
		expected  *copier.AnswersFile
		expectErr bool
	}{
		{
			name:      "no answers file",
			writeFile: false,
			expected:  nil,
		},
		{
			name:      "parses src path and commit",
			writeFile: true,
			content:   "_src_path: https://github.com/kyleking/my_go_template\n_commit: v1.2.3\n",
			expected:  &copier.AnswersFile{SrcPath: "https://github.com/kyleking/my_go_template", Commit: "v1.2.3"},
		},
		{
			name:      "malformed yaml",
			writeFile: true,
			content:   "_src_path: [unterminated",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runReadAnswersFileCase(t, tt)
		})
	}
}

func runReadAnswersFileCase(t *testing.T, tt struct {
	name      string
	content   string
	writeFile bool
	expected  *copier.AnswersFile
	expectErr bool
},
) {
	t.Helper()

	dir := t.TempDir()
	if tt.writeFile {
		writeFile(t, dir, ".copier-answers.yml", tt.content)
	}

	answers, err := copier.ReadAnswersFile(dir)
	if tt.expectErr {
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		return
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tt.expected == nil {
		if answers != nil {
			t.Errorf("expected nil, got %+v", answers)
		}

		return
	}

	if answers == nil || *answers != *tt.expected {
		t.Errorf("expected %+v, got %+v", tt.expected, answers)
	}
}
