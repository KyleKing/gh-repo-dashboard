package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// Predicate reports whether a repo summary matches a filter expression.
type Predicate func(models.RepoSummary) bool

// ParseError reports a filter predicate expression that failed to parse.
type ParseError struct {
	Input   string
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parsing %q: %s", e.Input, e.Message)
}

func atoms() map[string]Predicate {
	return map[string]Predicate{
		"ahead":        func(s models.RepoSummary) bool { return s.Ahead > 0 },
		"behind":       func(s models.RepoSummary) bool { return s.Behind > 0 },
		"clean":        func(s models.RepoSummary) bool { return !s.IsDirty() },
		"dirty":        models.RepoSummary.IsDirty,
		"error":        func(s models.RepoSummary) bool { return s.Error != nil },
		"git":          func(s models.RepoSummary) bool { return s.VCSType == models.VCSTypeGit },
		"has_pr":       func(s models.RepoSummary) bool { return s.PRInfo != nil },
		"has_stash":    func(s models.RepoSummary) bool { return s.StashCount > 0 },
		"has_upstream": func(s models.RepoSummary) bool { return s.Upstream != "" },
		"jj":           func(s models.RepoSummary) bool { return s.VCSType == models.VCSTypeJJ },
	}
}

// AtomNames returns the valid predicate atoms, sorted, for completion.
func AtomNames() []string {
	names := make([]string, 0, len(atoms()))
	for name := range atoms() {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}

type parser struct {
	input  string
	tokens []string
	pos    int
}

// ParsePredicate parses expressions like "dirty and has_pr",
// "behind or ahead", "not clean", and "(dirty or behind) and has_pr".
// Precedence: not > and > or.
func ParsePredicate(input string) (Predicate, error) {
	p := &parser{input: input, tokens: tokenize(input)}
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

func (p *parser) peek() (string, bool) {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos], true
	}

	return "", false
}

func (p *parser) parseOr() (Predicate, error) {
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
		left = func(s models.RepoSummary) bool { return l(s) || right(s) }
	}
}

func (p *parser) parseAnd() (Predicate, error) {
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
		left = func(s models.RepoSummary) bool { return l(s) && right(s) }
	}
}

func (p *parser) parseUnary() (Predicate, error) {
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

		return func(s models.RepoSummary) bool { return !inner(s) }, nil

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
		atom, found := atoms()[tok]
		if !found {
			msg := fmt.Sprintf("unknown atom %q (valid: %s)", tok, strings.Join(AtomNames(), ", "))
			return nil, &ParseError{Input: p.input, Message: msg}
		}
		p.pos++

		return atom, nil
	}
}
