package filters

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kyleking/aragonite/filter"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// Predicate reports whether a value of T matches a filter expression. It is
// aragonite/filter's own, so the parser below composes with All, Any, and Not
// rather than spelling the same combinators out again.
type Predicate[T any] = filter.Predicate[T]

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
		"git":             func(s models.RepoSummary) bool { return s.VCSType == vcs.TypeGit },
		"has_notes":       models.RepoSummary.HasNotes,
		"has_pr":          func(s models.RepoSummary) bool { return s.PRInfo != nil },
		"has_stash":       func(s models.RepoSummary) bool { return s.StashCount > 0 },
		"has_upstream":    func(s models.RepoSummary) bool { return s.Upstream != "" },
		atomHTTPS:         func(s models.RepoSummary) bool { return s.RemoteProtocol == atomHTTPS },
		"jj":              func(s models.RepoSummary) bool { return s.VCSType == vcs.TypeJJ },
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

// Field builds a comparison predicate such as "git_size > 1G", where arg is the
// parenthesized argument of a call like "last_run(gc)" and empty for a bare name.
type Field[T any] func(arg, op, operand string) (Predicate[T], error)

// Call builds a predicate from a call with no operator, such as "due(gc)".
type Call[T any] func(arg string) (Predicate[T], error)

// Grammar is the vocabulary a predicate expression may use: bare atoms,
// comparisons, and calls.
type Grammar[T any] struct {
	Atoms  map[string]Predicate[T]
	Calls  map[string]Call[T]
	Fields map[string]Field[T]
}

type parser[T any] struct {
	input   string
	tokens  []string
	pos     int
	grammar Grammar[T]
}

// ParsePredicate parses expressions like "dirty and has_pr",
// "behind or ahead", "not clean", and "(dirty or behind) and has_pr" against
// atoms, an atom name to Predicate map such as RepoAtoms() or PRAtoms().
// Precedence: not > and > or.
func ParsePredicate[T any](input string, atoms map[string]Predicate[T]) (Predicate[T], error) {
	return ParseGrammar(input, Grammar[T]{Atoms: atoms})
}

// ParseGrammar parses input like ParsePredicate, additionally accepting the
// comparisons and calls g defines.
func ParseGrammar[T any](input string, g Grammar[T]) (Predicate[T], error) {
	p := &parser[T]{input: input, tokens: tokenize(input), grammar: g}
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

// tokenize splits on whitespace and standalone parens. A paren glued to a name
// (not a keyword) and closed on the same expression stays in one token, so
// "last_run(gc)" reaches the parser whole.
func tokenize(input string) []string {
	var tokens []string
	start := -1
	flush := func(end int) {
		if start >= 0 {
			tokens = append(tokens, input[start:end])
			start = -1
		}
	}

	for i := 0; i < len(input); i++ {
		switch c := input[i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			flush(i)
		case c == '(' && start >= 0 && !isKeyword(input[start:i]):
			closing := strings.IndexByte(input[i:], ')')
			if closing < 0 {
				flush(i)
				tokens = append(tokens, "(")

				continue
			}
			i += closing
		case c == '(' || c == ')':
			flush(i)
			tokens = append(tokens, string(c))
		default:
			if start < 0 {
				start = i
			}
		}
	}
	flush(len(input))

	return tokens
}

const (
	kwAnd = "and"
	kwNot = "not"
	kwOr  = "or"
)

func isKeyword(tok string) bool {
	return tok == kwNot || tok == kwAnd || tok == kwOr
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
		if !ok || tok != kwOr {
			return left, nil
		}
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = filter.Any(left, right)
	}
}

func (p *parser[T]) parseAnd() (Predicate[T], error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		tok, ok := p.peek()
		if !ok || tok != kwAnd {
			return left, nil
		}
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = filter.All(left, right)
	}
}

func (p *parser[T]) parseUnary() (Predicate[T], error) {
	tok, ok := p.peek()
	if !ok {
		return nil, &ParseError{Input: p.input, Message: "unexpected end of expression"}
	}

	switch tok {
	case kwNot:
		p.pos++
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}

		return filter.Not(inner), nil

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

	case ")", kwAnd, kwOr:
		return nil, &ParseError{Input: p.input, Message: fmt.Sprintf("unexpected token %q", tok)}

	default:
		return p.parseTerm(tok)
	}
}

func (p *parser[T]) parseTerm(tok string) (Predicate[T], error) {
	name, arg := splitCall(tok)

	if call, found := p.grammar.Calls[name]; found && arg != "" {
		p.pos++
		pred, err := call(arg)
		if err != nil {
			return nil, &ParseError{Input: p.input, Message: err.Error()}
		}

		return pred, nil
	}

	if field, found := p.grammar.Fields[name]; found {
		return p.parseComparison(tok, field, arg)
	}

	atom, found := p.grammar.Atoms[tok]
	if !found {
		msg := fmt.Sprintf("unknown atom %q (valid: %s)", tok, strings.Join(p.validNames(), ", "))
		return nil, &ParseError{Input: p.input, Message: msg}
	}
	p.pos++

	return atom, nil
}

func (p *parser[T]) parseComparison(tok string, field Field[T], arg string) (Predicate[T], error) {
	if p.pos+2 >= len(p.tokens) {
		return nil, &ParseError{Input: p.input, Message: fmt.Sprintf("%q needs an operator and a value", tok)}
	}
	op, operand := p.tokens[p.pos+1], p.tokens[p.pos+2]
	pred, err := field(arg, op, operand)
	if err != nil {
		return nil, &ParseError{Input: p.input, Message: err.Error()}
	}
	p.pos += 3

	return pred, nil
}

func (p *parser[T]) validNames() []string {
	names := AtomNames(p.grammar.Atoms)
	for name := range p.grammar.Fields {
		names = append(names, name)
	}
	for name := range p.grammar.Calls {
		names = append(names, name+"(...)")
	}
	slices.Sort(names)

	return names
}

// splitCall separates "name(arg)" into its parts, and returns tok with an empty
// argument when it is not a call.
func splitCall(tok string) (string, string) {
	open := strings.IndexByte(tok, '(')
	if open <= 0 || !strings.HasSuffix(tok, ")") {
		return tok, ""
	}

	return tok[:open], tok[open+1 : len(tok)-1]
}
