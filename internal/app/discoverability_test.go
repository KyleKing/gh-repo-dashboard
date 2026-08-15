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

// TestCommandBarShowsCompletionsLive confirms the completion candidates
// render as soon as text matches them, without a Tab press first, each with
// its own one-line explanation.
func TestCommandBarShowsCompletionsLive(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width, m.height = 120, 30
	m.commandMode = true
	m.commandInput.SetValue("filter di")

	rendered := plainText(m.renderScreen())
	if !strings.Contains(rendered, "dirty") {
		t.Errorf("expected the live completion candidate before any Tab press:\n%s", rendered)
	}
	if !strings.Contains(rendered, "uncommitted changes") {
		t.Errorf("expected the atom's one-line description alongside it:\n%s", rendered)
	}
}

// TestBareCommandPromptListsEveryCommandWithItsDescription confirms a bare
// ":" is itself a discoverability surface: every registered command, each
// with the same description the help overlay gives it.
func TestBareCommandPromptListsEveryCommandWithItsDescription(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width, m.height = 120, 30
	m.commandMode = true
	m.commandInput.SetValue("")

	rendered := plainText(m.renderScreen())
	if !strings.Contains(rendered, nameFilter) {
		t.Errorf("expected the filter command listed:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Filter repos") {
		t.Errorf("expected the filter command's own description alongside it:\n%s", rendered)
	}
}

// TestSearchBoxShowsScopeLegend confirms every scope prefix is spelled out,
// short and long form, the moment the search box opens.
func TestSearchBoxShowsScopeLegend(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width, m.height = 120, 30
	m.searching = true

	rendered := plainText(m.renderRepoList())
	for _, want := range []string{"[r]epo:", "[b]ranch:", "[p]r:", "[t]emplate:", "[n]otes:", "[c]ommit:"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("search legend is missing %q:\n%s", want, rendered)
		}
	}
}

func TestHelpCoversTheCommandLayer(t *testing.T) {
	t.Parallel()

	help := plainText(New(nil, 1).renderHelp())

	for _, want := range []string{"Command Mode", "@:", ":history", "!fdr"} {
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
