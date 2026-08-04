//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestPanelWidthOnlyAtWideSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width, height int
		wantPanel     bool
	}{
		{name: "compact has no panel", width: 80, height: 24},
		{name: "standard has no panel", width: 140, height: 40},
		{name: "wide has a panel", width: 160, height: 50, wantPanel: true},
		{name: "a short wide terminal drops the panel", width: 200, height: 18},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			panel := panelWidth(tt.width, tt.height)
			if (panel > 0) != tt.wantPanel {
				t.Fatalf("panelWidth(%d, %d) = %d, want a panel: %v", tt.width, tt.height, panel, tt.wantPanel)
			}

			if !tt.wantPanel {
				return
			}

			if panel < overviewMinWidth || panel > overviewMaxWidth {
				t.Errorf("panel is %d cells, want between %d and %d", panel, overviewMinWidth, overviewMaxWidth)
			}

			if got := listWidth(tt.width, tt.height); got < overviewMinListWidth {
				t.Errorf("list keeps only %d cells, want at least %d", got, overviewMinListWidth)
			}
		})
	}
}

func TestWidePanelPreservesTheFullTable(t *testing.T) {
	t.Parallel()

	if got := layoutRepoCols(listWidth(180, 40)).Hidden; got != 0 {
		t.Errorf("the preview panel forced %d repo columns to hide; the list must keep them all", got)
	}
}

func TestWidePanelTracksTheCursor(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 40)

	first := plainText(m.renderListWithPreview())
	if !strings.Contains(first, "alpha") || !strings.Contains(first, "Stashes") {
		t.Fatalf("panel missing for the first repo:\n%s", first)
	}

	m.cursor = 1
	second := plainText(m.renderListWithPreview())
	if !strings.Contains(second, ".doing") {
		t.Errorf("panel did not follow the cursor to bravo:\n%s", second)
	}
}

func TestWideFrameLinesAreUniform(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 40)

	lines := strings.Split(m.renderScreen(), "\n")
	want := lipgloss.Width(lines[0])

	for i, line := range lines {
		if got := lipgloss.Width(line); got != want {
			t.Errorf("line %d is %d cells wide, want %d; ragged lines leave stale cells on screen", i, got, want)
		}
	}
}

// TestResizeIsStateless reproduces the 220 → 80 → 220 round trip from the M14
// exit criteria: layout is derived from size alone, so the original frame must
// come back byte for byte.
func TestResizeIsStateless(t *testing.T) {
	t.Parallel()

	m := compactModel(220, 50)
	before := m.renderScreen()

	m.width, m.height = 80, 24
	narrow := m.renderScreen()

	m.width, m.height = 220, 50
	after := m.renderScreen()

	if narrow == before {
		t.Error("the 80x24 frame is identical to the 220x50 one; the layout is not responding to width")
	}

	if after != before {
		t.Errorf("resizing back to 220x50 did not reproduce the original frame:\nbefore:\n%s\nafter:\n%s",
			plainText(before), plainText(after))
	}
}
