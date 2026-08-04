//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"regexp"
	"strings"
	"testing"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plainText(rendered string) string {
	return ansiPattern.ReplaceAllString(rendered, "")
}

func TestFooterAdvertisesCommandMode(t *testing.T) {
	t.Parallel()

	footer := plainText(New(nil, 1).renderFooter())
	if !strings.Contains(footer, ": command") {
		t.Errorf("footer does not mention command mode:\n%s", footer)
	}
}

func TestHelpCoversTheCommandLayer(t *testing.T) {
	t.Parallel()

	help := plainText(New(nil, 1).renderHelp())

	for _, want := range []string{"Command Mode", "@:", ":history", "Fdr"} {
		if !strings.Contains(help, want) {
			t.Errorf("help overlay is missing %q:\n%s", want, help)
		}
	}
}

func TestHelpListsEveryTextObject(t *testing.T) {
	t.Parallel()

	help := plainText(New(nil, 1).renderHelp())
	for _, obj := range textObjects() {
		if !strings.Contains(help, obj.Key) {
			t.Errorf("help overlay omits the %q text object (%s)", obj.Key, obj.Name)
		}
		if !strings.Contains(help, obj.Name) {
			t.Errorf("help overlay omits the name of the %q text object", obj.Key)
		}
	}
}
