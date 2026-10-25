package filters

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

var (
	errLastRunNeedsTask = errors.New("last_run needs a task name, as in last_run(gc)")
	errNoArgument       = errors.New("takes no argument")
	errTemplateOp       = errors.New("template only takes ~")
	errUnknownOp        = errors.New("unknown operator (valid: > >= < <= =)")
	errInvalidSize      = errors.New("invalid size (want a number with optional K, M, G, or T)")
)

const bitsPerSizeStep = 10

// Runs is how the run-history predicates read task state. Nil funcs read as
// "never ran" and "not due", which lets config validation parse an expression
// without a store.
type Runs struct {
	LastRun func(task, repoPath string) (time.Time, bool)
	Due     func(task string, s models.RepoSummary) bool
	Now     func() time.Time
}

func (r Runs) since(at time.Time) time.Duration {
	if r.Now != nil {
		return r.Now().Sub(at)
	}

	return time.Since(at)
}

// RepoGrammar is RepoAtoms plus the comparisons and calls over a repo summary:
// git_size, template, last_run(task), and due(task).
func RepoGrammar(runs Runs) Grammar[models.RepoSummary] {
	return Grammar[models.RepoSummary]{
		Atoms: RepoAtoms(),
		Calls: map[string]Call[models.RepoSummary]{
			"due": func(task string) (Predicate[models.RepoSummary], error) {
				return func(s models.RepoSummary) bool {
					return runs.Due != nil && runs.Due(task, s)
				}, nil
			},
		},
		Fields: map[string]Field[models.RepoSummary]{
			"git_size": gitSizeField,
			"last_run": lastRunField(runs),
			"template": templateField,
		},
	}
}

func gitSizeField(arg, op, operand string) (Predicate[models.RepoSummary], error) {
	if arg != "" {
		return nil, fmt.Errorf("git_size %w", errNoArgument)
	}
	limit, err := ParseSize(operand)
	if err != nil {
		return nil, err
	}
	compare, err := comparator[int64](op)
	if err != nil {
		return nil, err
	}

	return func(s models.RepoSummary) bool {
		return compare(models.GitSize(s.Path), limit)
	}, nil
}

func lastRunField(runs Runs) Field[models.RepoSummary] {
	return func(task, op, operand string) (Predicate[models.RepoSummary], error) {
		if task == "" {
			return nil, errLastRunNeedsTask
		}
		age, err := ParseAge(operand)
		if err != nil {
			return nil, err
		}
		compare, err := comparator[time.Duration](op)
		if err != nil {
			return nil, err
		}

		return func(s models.RepoSummary) bool {
			if runs.LastRun == nil {
				return true
			}
			at, ok := runs.LastRun(task, s.Path)
			if !ok {
				return true
			}

			return compare(runs.since(at), age)
		}, nil
	}
}

func templateField(arg, op, operand string) (Predicate[models.RepoSummary], error) {
	if arg != "" {
		return nil, fmt.Errorf("template %w", errNoArgument)
	}
	if op != "~" {
		return nil, fmt.Errorf("%w, not %q", errTemplateOp, op)
	}
	needle := strings.ToLower(operand)

	return func(s models.RepoSummary) bool {
		return s.TemplateInfo != nil && strings.Contains(strings.ToLower(s.TemplateInfo.SrcPath), needle)
	}, nil
}

func comparator[V cmp.Ordered](op string) (func(a, b V) bool, error) {
	switch op {
	case ">":
		return func(a, b V) bool { return a > b }, nil
	case ">=":
		return func(a, b V) bool { return a >= b }, nil
	case "<":
		return func(a, b V) bool { return a < b }, nil
	case "<=":
		return func(a, b V) bool { return a <= b }, nil
	case "=":
		return func(a, b V) bool { return a == b }, nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownOp, op)
	}
}

// ParseSize parses a byte count with an optional binary K, M, G, or T suffix
// ("1G" is 1024^3), as in "500M" or "1.5G".
func ParseSize(s string) (int64, error) {
	upper := strings.ToUpper(strings.TrimSuffix(strings.ToUpper(s), "B"))
	multiplier := int64(1)
	for i, suffix := range []string{"K", "M", "G", "T"} {
		if strings.HasSuffix(upper, suffix) {
			upper = strings.TrimSuffix(upper, suffix)
			multiplier = int64(1) << (bitsPerSizeStep * (i + 1))

			break
		}
	}
	n, err := strconv.ParseFloat(upper, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%w: %q", errInvalidSize, s)
	}

	return int64(n * float64(multiplier)), nil
}

// ParseAge parses an age such as "30d", "12h", or "2w".
func ParseAge(s string) (time.Duration, error) {
	return parseRecencyDuration(s)
}
