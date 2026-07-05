package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateDocs = flag.Bool("update-docs", false, "regenerate docs/USAGE.md from fixtures")

const usageDocsPath = "../../docs/USAGE.md"

func describeAssertion(a fixtureAssertion) string {
	value := a.Value
	if value == "" {
		value = "(cleared)"
	}
	switch a.Field {
	case "filtered":
		return "shows " + strings.ReplaceAll(value, " ", ", ")
	case "selected":
		return "selects " + strings.ReplaceAll(value, " ", ", ")
	case "cursor":
		return "cursor on row " + value
	case "view":
		return "opens the " + value + " view"
	case "task":
		return "starts batch " + value
	case "total":
		return "over " + value + " repos"
	case "input":
		return "input reads " + value
	default:
		return a.Field + ": " + value
	}
}

func describeInput(step fixtureStep) string {
	if step.IsCommand {
		return "`" + step.Input + "`"
	}
	keys := make([]string, len(step.Keys))
	for i, k := range step.Keys {
		keys[i] = "`" + k + "`"
	}

	return "press " + strings.Join(keys, " ")
}

func generateUsageDocs(fixtures []fixture) string {
	var b strings.Builder
	b.WriteString("# Usage\n\n")
	b.WriteString("<!-- Generated from internal/app/testdata/fixtures/ by `mise run docs:usage`; do not edit by hand. -->\n\n")
	b.WriteString("Every example below is executed as a test (`TestFixtures`), so this page\n")
	b.WriteString("cannot drift from the implementation. Commands (`:...`) can be typed after\n")
	b.WriteString("pressing `:`; bare keys act on the repo list.\n")

	for _, f := range fixtures {
		b.WriteString("\n## " + f.Doc + "\n\n")
		b.WriteString("| Input | Result |\n|---|---|\n")
		for _, step := range f.Steps {
			results := make([]string, 0, len(step.Assertions))
			for _, a := range step.Assertions {
				results = append(results, describeAssertion(a))
			}
			result := strings.Join(results, "; ")
			if result == "" {
				result = emDash
			}
			fmt.Fprintf(&b, "| %s | %s |\n", describeInput(step), result)
		}
	}

	return b.String()
}

//nolint:paralleltest // conditionally writes usageDocsPath under -update-docs; not safe to run concurrently with other doc-writing tests
func TestUsageDocsCurrent(t *testing.T) {
	generated := generateUsageDocs(loadFixtures(t))

	if *updateDocs {
		if err := os.MkdirAll(filepath.Dir(usageDocsPath), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(usageDocsPath, []byte(generated), 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	existing, err := os.ReadFile(usageDocsPath)
	if err != nil {
		t.Fatalf("read %s (run `mise run docs:usage` to generate): %v", usageDocsPath, err)
	}
	if string(existing) != generated {
		t.Errorf("%s is stale; run `mise run docs:usage`", usageDocsPath)
	}
}
