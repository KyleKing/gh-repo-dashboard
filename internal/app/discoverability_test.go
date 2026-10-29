//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/keyhint"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plainText(rendered string) string {
	return ansiPattern.ReplaceAllString(rendered, "")
}

func headerLine(m Model) string {
	return plainText(strings.SplitN(m.renderScreen(), "\n", 2)[0])
}

func footerLine(m Model) string {
	lines := strings.Split(m.renderScreen(), "\n")

	return plainText(lines[len(lines)-1])
}

func TestFooterAdvertisesCommandMode(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width, m.height = 120, 30

	footer := footerLine(m)
	if !strings.Contains(footer, "[:] command") {
		t.Errorf("footer does not mention command mode:\n%s", footer)
	}
}

func TestFooterCollapsesByPriorityInsteadOfClipping(t *testing.T) {
	t.Parallel()

	for _, termWidth := range []int{80, 100, 160, 220} {
		m := New(nil, 1)
		m.width, m.height = termWidth, 30

		footer := footerLine(m)
		if got := lipgloss.Width(footer); got > termWidth {
			t.Errorf("width %d: footer is %d cells: %q", termWidth, got, footer)
		}

		// A dropped hint takes its key with it, so no half-rendered hint is left.
		plain := footer
		for _, essential := range []string{"[enter] select", "[?] help", "[q]uit"} {
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

// legendText is every page of the open legend, followed down each chord key.
func legendText(m Model) string {
	var walk func(page []keyhint.Hint) []string

	walk = func(page []keyhint.Hint) []string {
		out := keyhint.Column(legendStyles(), page, 200)
		for _, h := range page {
			out = append(out, walk(h.Kids)...)
		}

		return out
	}

	return plainText(strings.Join(walk(m.helpTree()), "\n"))
}

func TestHelpCoversTheCommandLayer(t *testing.T) {
	t.Parallel()

	help := legendText(New(nil, 1).openHelp())

	for _, want := range []string{"commands", "@:", ":history", operatorLeader + "  an operator"} {
		if !strings.Contains(help, want) {
			t.Errorf("help legend is missing %q:\n%s", want, help)
		}
	}
}

func TestHelpListsEveryTextObject(t *testing.T) {
	t.Parallel()

	help := legendText(New(nil, 1).openHelp())
	for _, obj := range textObjects() {
		if !strings.Contains(help, obj.Key) {
			t.Errorf("help legend omits the %q text object (%s)", obj.Key, obj.Name)
		}
		if !strings.Contains(help, obj.Name) {
			t.Errorf("help legend omits the name of the %q text object", obj.Key)
		}
	}
}

// At 80x24 the root page is longer than the frame, so the legend has to scroll
// and keep the footer where it is, and a chord key has to open its own page.
func TestHelpScrollsWalksPagesAndReturnsToItsView(t *testing.T) {
	t.Parallel()

	m := New(nil, 1)
	m.width, m.height = 80, 24
	m = afterUpdate(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})

	top := plainText(m.renderScreen())
	if got := strings.Count(top, "\n") + 1; got != m.height {
		t.Fatalf("help drew %d lines into a %d-line terminal", got, m.height)
	}
	if !strings.Contains(top, "moving") || !strings.Contains(top, "[?] close") {
		t.Errorf("help does not open at the top of the legend with its footer:\n%s", top)
	}

	for range 40 {
		m = afterUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if bottom := plainText(m.renderScreen()); !strings.Contains(bottom, "this legend") || bottom == top {
		t.Errorf("help did not scroll to the end of the legend:\n%s", bottom)
	}

	m = afterUpdate(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if page := plainText(m.renderScreen()); !strings.Contains(page, "a then") || !strings.Contains(page, "up a page") {
		t.Errorf("the chord key did not open its page:\n%s", page)
	}

	m = afterUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.helpPath) != 0 || m.viewMode != ViewModeHelp {
		t.Errorf("esc should leave the page and stay in help: path %v, view %v", m.helpPath, m.viewMode)
	}

	m = afterUpdate(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.viewMode != ViewModeRepoList {
		t.Errorf("esc on the root page should return to the list, got %v", m.viewMode)
	}
}

// footerScreens builds one model per screen at a given size, for the tests that
// hold every screen to the same frame.
func footerScreens() map[string]func(t *testing.T, width, height int) Model {
	return map[string]func(t *testing.T, width, height int) Model{
		"list":   func(_ *testing.T, w, h int) Model { return compactModel(w, h) },
		"detail": func(_ *testing.T, w, h int) Model { return focusedModel(w, h) },
		"branch": func(_ *testing.T, w, h int) Model {
			m := focusedModel(w, h)
			m.viewMode = ViewModeBranchDetail

			return m
		},
		"prmap": func(_ *testing.T, w, h int) Model {
			m := mapFleet()
			m.width, m.height = w, h

			return m
		},
		"palette": func(t *testing.T, w, h int) Model {
			t.Helper()

			m := paletteFleet()
			m.width, m.height = w, h
			opened, _ := m.openPalette()

			return mustModel(t, opened)
		},
		"tasks": func(_ *testing.T, w, h int) Model {
			m := tasksGoldenModel()
			m.width, m.height = w, h

			return m
		},
		"preview": func(t *testing.T, w, h int) Model {
			t.Helper()

			m := tasksGoldenModel()
			m.width, m.height = w, h
			next, _ := m.openPreview(m.taskList()[0])

			return mustModel(t, next)
		},
		"run": func(_ *testing.T, w, h int) Model {
			m := tasksGoldenModel()
			m.width, m.height = w, h
			m.viewMode = ViewModeTaskRun
			m.run = taskRun{label: "fetch", paths: m.repoPaths, running: true}
			for _, path := range m.repoPaths {
				m.run.rows = append(m.run.rows, runRow{path: path})
			}

			return m
		},
		"log": func(_ *testing.T, w, h int) Model {
			m := tasksGoldenModel()
			m.width, m.height = w, h
			m.viewMode = ViewModeTaskLog
			m.run = taskRun{label: "fetch"}
			m.runLog = runLog{repo: "alpha", lines: []string{"fetching origin", "done"}}

			return m
		},
		"help": func(_ *testing.T, w, h int) Model { return compactModel(w, h).openHelp() },
		"actions": func(_ *testing.T, w, h int) Model {
			m := compactModel(w, h)
			m.panelActions = true

			return m
		},
		"filter": func(_ *testing.T, w, h int) Model {
			m := compactModel(w, h)
			m.viewMode = ViewModeFilter

			return m
		},
		"confirm": func(_ *testing.T, w, h int) Model {
			m := compactModel(w, h)
			m.viewMode = ViewModeConfirm
			m.pendingAction = &pendingAction{prompt: "Push branch?", detail: "feature/login"}

			return m
		},
	}
}

// Every screen draws through the shared frame, so each fills the terminal
// exactly and keeps its footer on the last row at every size.
func TestEveryScreenPinsItsFooter(t *testing.T) {
	t.Parallel()

	screens := footerScreens()

	for name, build := range screens {
		for _, size := range [][2]int{{80, 24}, {120, 35}, {220, 50}} {
			t.Run(fmt.Sprintf("%s-%dx%d", name, size[0], size[1]), func(t *testing.T) {
				t.Parallel()

				m := build(t, size[0], size[1])
				lines := strings.Split(m.renderScreen(), "\n")

				if len(lines) != size[1] {
					t.Fatalf("%dx%d: drew %d lines", size[0], size[1], len(lines))
				}

				for i, line := range lines {
					if got := lipgloss.Width(line); got != size[0]-screenEdge {
						t.Errorf("%dx%d: line %d is %d cells wide", size[0], size[1], i, got)
					}
				}

				if footer := plainText(lines[len(lines)-1]); !strings.Contains(footer, "[") {
					t.Errorf("%dx%d: the last row is not the footer: %q", size[0], size[1], footer)
				}
			})
		}
	}
}

// A status message takes the footer's right end and fades, so it never covers
// the keys and never outlives its delay.
func TestStatusMessageSharesTheFooterAndFades(t *testing.T) {
	t.Parallel()

	m := compactModel(120, 30)

	updated, cmd := m.Update(StatusMsg{Message: "Marks cleared"})
	m = mustModel(t, updated)

	if cmd == nil {
		t.Fatal("a status message must schedule its own fade")
	}

	footer := footerLine(m)
	if !strings.Contains(footer, "Marks cleared") || !strings.Contains(footer, "[enter] select") {
		t.Errorf("the status should sit beside the keys, got %q", footer)
	}

	cleared, _ := m.Update(ClearStatusMsg{})
	if strings.Contains(footerLine(mustModel(t, cleared)), "Marks cleared") {
		t.Error("the status survived its fade")
	}
}

// A clean tree renders blank and the sort shows as an arrow on its column.
func TestListDrawsQuietStateAndTheSortArrow(t *testing.T) {
	t.Parallel()

	rendered := plainText(compactModel(140, 30).renderScreen())
	rows := strings.Split(rendered, "\n")

	if !strings.Contains(rows[2], "NAME ▴") {
		t.Errorf("the sort arrow is missing from its column title:\n%s", rendered)
	}

	if rowText := strings.Join(rows[3:6], "\n"); strings.Contains(rowText, "✓") {
		t.Errorf("a clean tree should render blank, not a check:\n%s", rendered)
	}
}
