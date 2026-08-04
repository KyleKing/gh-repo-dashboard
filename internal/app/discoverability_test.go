//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plainText(rendered string) string {
	return ansiPattern.ReplaceAllString(rendered, "")
}

func TestFooterAdvertisesCommandMode(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width = 120

	footer := plainText(m.renderFooter())
	if !strings.Contains(footer, ": command") {
		t.Errorf("footer does not mention command mode:\n%s", footer)
	}
}

func TestFooterCollapsesByPriorityInsteadOfClipping(t *testing.T) {
	t.Parallel()

	for _, termWidth := range []int{80, 100, 160, 220} {
		m := New(nil, 1)
		m.width = termWidth

		footer := m.renderFooter()
		if got, limit := lipgloss.Width(footer), contentWidth(termWidth); got > limit {
			t.Errorf("width %d: footer is %d cells, content width is %d: %q",
				termWidth, got, limit, plainText(footer))
		}

		// A dropped hint takes its key with it, so no half-rendered hint is left.
		plain := plainText(footer)
		for _, essential := range []string{"enter select", "? help", "q quit"} {
			if !strings.Contains(plain, essential) {
				t.Errorf("width %d: footer dropped %q, which must survive: %q", termWidth, essential, plain)
			}
		}
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
