//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestMarkFillsTheSelectedTextObject is the point of the whole feature: the
// "sr" text object named a set the keyboard had no way to fill, so "!fsr" was
// a sentence nobody could say. Marking two rows and composing the operator
// over them proves the loop closed.
func TestMarkFillsTheSelectedTextObject(t *testing.T) {
	t.Parallel()

	marked, _ := pressKeys(t, operatorModel(), "xx")
	if got := len(marked.markedPaths()); got != 2 {
		t.Fatalf("expected 2 marked rows, got %d", got)
	}

	ran, cmd := pressKeys(t, marked, "!fsr")
	if ran.batchTask != "Fetch All (marked)" {
		t.Errorf("expected the marked scope, got %q", ran.batchTask)
	}
	if ran.batchTotal != 2 {
		t.Errorf("expected 2 repos, got %d", ran.batchTotal)
	}
	if cmd == nil {
		t.Error("expected a batch cmd")
	}
}

// TestMarkStepsPastTheRowItMarked covers marking a run: the cursor advances so
// holding one key marks consecutive rows, and it stops at the last one rather
// than running off the end.
func TestMarkStepsPastTheRowItMarked(t *testing.T) {
	t.Parallel()

	m := operatorModel()

	one, _ := pressKeys(t, m, "x")
	if one.cursor != 1 {
		t.Errorf("expected the cursor to step past the marked row, got %d", one.cursor)
	}

	all, _ := pressKeys(t, m, strings.Repeat("x", len(m.filteredPaths)+2))
	if all.cursor != len(m.filteredPaths)-1 {
		t.Errorf("the cursor should stop at the last row, got %d", all.cursor)
	}
}

// TestVisualRangeShowsOverTheMarksAndLeavesThemIntact covers the two mark
// sources staying separate: a live range is what operators see while it is
// open, and closing it restores whatever was marked before.
func TestVisualRangeShowsOverTheMarksAndLeavesThemIntact(t *testing.T) {
	t.Parallel()

	// Mark the first row, then open a range further down.
	marked, _ := pressKeys(t, operatorModel(), "x")
	first := marked.filteredPaths[0]

	ranged, _ := pressKeys(t, marked, "Vj")
	if !ranged.visualMode {
		t.Fatal("expected V to open a range")
	}
	if got := len(ranged.markedPaths()); got != 2 {
		t.Errorf("expected the range's 2 rows, got %d", got)
	}
	if ranged.isMarked(first) {
		t.Error("a row outside the range must not draw its mark while the range is open")
	}
	if !ranged.selectedPaths[first] {
		t.Error("the range must not overwrite what was already marked")
	}
}

// TestEscClearsMarksBeforeAnythingElse covers the esc ordering: a range or a
// set of marks is the nearest layer to back out of, ahead of the expanded
// region underneath it.
func TestEscClearsMarksBeforeAnythingElse(t *testing.T) {
	t.Parallel()

	m := operatorModel()
	m.expandOpen = true

	marked, _ := pressKeys(t, m, "x")

	cleared := mustModel(t, mustUpdate(t, &marked, tea.KeyPressMsg{Code: tea.KeyEscape}))
	if len(cleared.markedPaths()) != 0 {
		t.Errorf("esc should clear the marks, got %v", cleared.markedPaths())
	}
	if !cleared.expandOpen {
		t.Error("esc must not close the expanded region while marks are still up")
	}
}

// TestActionMenuNamesWhatItActsOn covers the discoverability half: the menu's
// target line is the only place the mark count is stated, so a batch verb run
// from it can never be a surprise.
func TestActionMenuNamesWhatItActsOn(t *testing.T) {
	t.Parallel()

	bare := operatorModel().actionMenu()
	if !strings.Contains(bare.target, "match the current filters") {
		t.Errorf("with nothing marked the menu should name the filtered set, got %q", bare.target)
	}

	marked, _ := pressKeys(t, operatorModel(), "xx")
	if got := marked.actionMenu().target; got != "2 marked" {
		t.Errorf("expected the menu to name the marks, got %q", got)
	}
}
