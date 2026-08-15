package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// Predicate reports whether a value of T matches a filter expression.
type Predicate[T any] func(T) bool

// ParseError reports a filter predicate expression that failed to parse.
type ParseError struct {
	Input   string
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parsing %q: %s", e.Input, e.Message)
}

// Remote protocol atom names, shared by RepoAtoms and RepoAtomDescriptions so
// each spells the name once rather than duplicating the literal.
const (
	atomHTTPS = "https"
	atomSSH   = "ssh"
)

// RepoAtoms returns the predicate atoms available over a repo summary.
func RepoAtoms() map[string]Predicate[models.RepoSummary] {
	return map[string]Predicate[models.RepoSummary]{
		"ahead":           func(s models.RepoSummary) bool { return s.Ahead > 0 },
		"behind":          func(s models.RepoSummary) bool { return s.Behind > 0 },
		"clean":           func(s models.RepoSummary) bool { return !s.IsDirty() },
		"config_override": models.RepoSummary.HasConfigOverrides,
		"dirty":           models.RepoSummary.IsDirty,
		"error":           func(s models.RepoSummary) bool { return s.Error != nil },
		"git":             func(s models.RepoSummary) bool { return s.VCSType == models.VCSTypeGit },
		"has_notes":       models.RepoSummary.HasNotes,
		"has_pr":          func(s models.RepoSummary) bool { return s.PRInfo != nil },
		"has_stash":       func(s models.RepoSummary) bool { return s.StashCount > 0 },
		"has_upstream":    func(s models.RepoSummary) bool { return s.Upstream != "" },
		atomHTTPS:         func(s models.RepoSummary) bool { return s.RemoteProtocol == atomHTTPS },
		"jj":              func(s models.RepoSummary) bool { return s.VCSType == models.VCSTypeJJ },
		atomSSH:           func(s models.RepoSummary) bool { return s.RemoteProtocol == atomSSH },
		"template_drift": func(s models.RepoSummary) bool {
			return s.TemplateInfo != nil && (s.TemplateInfo.Behind || !s.TemplateInfo.IsTag)
		},
	}
}

// RepoAtomDescriptions gives each RepoAtoms() name a one-line explanation, for
// the command bar's completion candidates.
func RepoAtomDescriptions() map[string]string {
	return map[string]string{
		"ahead":           "local branch has unpushed commits",
		"behind":          "local branch is missing commits from upstream",
		"clean":           "no uncommitted changes",
		"config_override": "repo has local git config overrides",
		"dirty":           "has uncommitted changes",
		"error":           "the last scan of this repo failed",
		"git":             "uses git rather than jj",
		"has_notes":       "has a NOTES file",
		"has_pr":          "has an open pull request",
		"has_stash":       "has stashed changes",
		"has_upstream":    "current branch tracks a remote",
		atomHTTPS:         "remote uses an https URL",
		"jj":              "uses Jujutsu rather than git",
		atomSSH:           "remote uses an ssh URL",
		"template_drift":  "behind or mismatched with its copier template",
	}
}

// AtomNames returns atoms' names, sorted, for completion and error messages.
func AtomNames[T any](atoms map[string]Predicate[T]) []string {
	names := make([]string, 0, len(atoms))
	for name := range atoms {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}

type parser[T any] struct {
	input  string
	tokens []string
	pos    int
	atoms  map[string]Predicate[T]
}

// ParsePredicate parses expressions like "dirty and has_pr",
// "behind or ahead", "not clean", and "(dirty or behind) and has_pr" against
// atoms, an atom name to Predicate map such as RepoAtoms() or PRAtoms().
// Precedence: not > and > or.
func ParsePredicate[T any](input string, atoms map[string]Predicate[T]) (Predicate[T], error) {
	p := &parser[T]{input: input, tokens: tokenize(input), atoms: atoms}
	if len(p.tokens) == 0 {
		return nil, &ParseError{Input: input, Message: "empty expression"}
	}
	pred, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.tokens) {
		return nil, &ParseError{Input: input, Message: fmt.Sprintf("unexpected token %q", p.tokens[p.pos])}
	}

	return pred, nil
}

func tokenize(input string) []string {
	replaced := strings.NewReplacer("(", " ( ", ")", " ) ").Replace(input)
	return strings.Fields(replaced)
}

func (p *parser[T]) peek() (string, bool) {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos], true
	}

	return "", false
}

func (p *parser[T]) parseOr() (Predicate[T], error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		tok, ok := p.peek()
		if !ok || tok != "or" {
			return left, nil
		}
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		l := left
		left = func(v T) bool { return l(v) || right(v) }
	}
}

func (p *parser[T]) parseAnd() (Predicate[T], error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		tok, ok := p.peek()
		if !ok || tok != "and" {
			return left, nil
		}
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		l := left
		left = func(v T) bool { return l(v) && right(v) }
	}
}

func (p *parser[T]) parseUnary() (Predicate[T], error) {
	tok, ok := p.peek()
	if !ok {
		return nil, &ParseError{Input: p.input, Message: "unexpected end of expression"}
	}

	switch tok {
	case "not":
		p.pos++
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}

		return func(v T) bool { return !inner(v) }, nil

	case "(":
		p.pos++
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		closing, ok := p.peek()
		if !ok || closing != ")" {
			return nil, &ParseError{Input: p.input, Message: "missing closing paren"}
		}
		p.pos++

		return inner, nil

	case ")", "and", "or":
		return nil, &ParseError{Input: p.input, Message: fmt.Sprintf("unexpected token %q", tok)}

	default:
		atom, found := p.atoms[tok]
		if !found {
			msg := fmt.Sprintf("unknown atom %q (valid: %s)", tok, strings.Join(AtomNames(p.atoms), ", "))
			return nil, &ParseError{Input: p.input, Message: msg}
		}
		p.pos++

		return atom, nil
	}
}
