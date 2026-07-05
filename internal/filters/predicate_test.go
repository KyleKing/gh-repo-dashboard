package filters_test

import (
	"errors"
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestParsePredicate(t *testing.T) {
	t.Parallel()
	dirty := models.RepoSummary{Unstaged: 2}
	dirtyWithPR := models.RepoSummary{Unstaged: 2, PRInfo: &models.PRInfo{Number: 1}}
	behind := models.RepoSummary{Behind: 3}
	ahead := models.RepoSummary{Ahead: 1}
	clean := models.RepoSummary{}
	jjRepo := models.RepoSummary{VCSType: models.VCSTypeJJ}
	withNotes := models.RepoSummary{NotesFile: "doing.md"}

	tests := []struct {
		name     string
		expr     string
		summary  models.RepoSummary
		expected bool
	}{
		{"single atom match", "dirty", dirty, true},
		{"single atom no match", "dirty", clean, false},
		{"and both", "dirty and has_pr", dirtyWithPR, true},
		{"and half", "dirty and has_pr", dirty, false},
		{"or first", "behind or ahead", behind, true},
		{"or second", "behind or ahead", ahead, true},
		{"or neither", "behind or ahead", clean, false},
		{"not", "not dirty", clean, true},
		{"not negative", "not dirty", dirty, false},
		{"not binds tighter than and", "not dirty and not behind", clean, true},
		{"parens", "(dirty or behind) and has_pr", dirtyWithPR, true},
		{"parens no match", "(dirty or behind) and has_pr", behind, false},
		{"precedence and over or", "dirty and has_pr or behind", behind, true},
		{"chained and", "clean and not behind and not ahead", clean, true},
		{"vcs atom", "jj", jjRepo, true},
		{"clean is not dirty", "clean", dirty, false},
		{"ahead counts as dirty", "dirty", ahead, true},
		{"has_notes match", "has_notes", withNotes, true},
		{"has_notes no match", "has_notes", clean, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pred, err := filters.ParsePredicate(tt.expr)
			if err != nil {
				t.Fatalf("filters.ParsePredicate(%q) error: %v", tt.expr, err)
			}
			if got := pred(tt.summary); got != tt.expected {
				t.Errorf("%q on %+v = %v; want %v", tt.expr, tt.summary, got, tt.expected)
			}
		})
	}
}

func TestParsePredicateErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		expr string
	}{
		{"empty", ""},
		{"whitespace", "   "},
		{"unknown atom", "bogus"},
		{"trailing operator", "dirty and"},
		{"leading operator", "and dirty"},
		{"missing close paren", "(dirty or behind"},
		{"unbalanced close", "dirty)"},
		{"double atom", "dirty clean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := filters.ParsePredicate(tt.expr)
			if err == nil {
				t.Fatalf("filters.ParsePredicate(%q) expected error", tt.expr)
			}
			var parseErr *filters.ParseError
			if !errors.As(err, &parseErr) {
				t.Errorf("expected *filters.ParseError, got %T", err)
			}
		})
	}
}

func TestAtomNamesSorted(t *testing.T) {
	t.Parallel()
	names := filters.AtomNames()
	if len(names) == 0 {
		t.Fatal("expected atoms")
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("filters.AtomNames not sorted: %q before %q", names[i-1], names[i])
		}
	}
}
