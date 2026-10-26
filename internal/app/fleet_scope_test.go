//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"github.com/kyleking/aragonite/tui/keyhint"
)

func TestFilterPanelScopeEntryIsOffWithoutAFleet(t *testing.T) {
	t.Parallel()

	m, _ := pressKeys(t, compactModel(120, 40), "ff")
	if m.fleetScope {
		t.Fatal("the scope switched without scan_paths")
	}

	if !strings.Contains(m.statusMessage, "scan_paths") {
		t.Errorf("status = %q, want the scan_paths reason", m.statusMessage)
	}

	var hint keyhint.Hint

	for _, h := range m.filterPanelHints() {
		if h.Key == scopeKey {
			hint = h
		}
	}

	if !hint.Off || !strings.Contains(hint.What, "scan_paths") {
		t.Errorf("hint = %+v, want Off with the scan_paths reason", hint)
	}
}

func TestFilterPanelScopeEntryRescansTheFleet(t *testing.T) {
	t.Parallel()

	m := compactModel(120, 40).WithFleet(Fleet{Roots: []string{"/fleet"}, Authors: []string{"me@example.com"}})
	stale := m.scopeGen

	m, cmd := pressKeys(t, m, "ff")
	if !m.fleetScope || cmd == nil || !m.loading {
		t.Fatalf("scope = %v loading = %v cmd nil = %v, want a fleet rescan", m.fleetScope, m.loading, cmd == nil)
	}

	if len(m.scanPaths) != 1 || m.scanPaths[0] != "/fleet" {
		t.Errorf("scanPaths = %v, want the fleet roots", m.scanPaths)
	}

	next, _ := m.Update(ReposDiscoveredMsg{Paths: []string{"/dev/old"}, Gen: stale})
	if got := mustModel(t, next); len(got.repoPaths) != 0 {
		t.Errorf("a discovery from before the switch listed %v", got.repoPaths)
	}

	next, _ = m.Update(ReposDiscoveredMsg{Paths: []string{"/fleet/a"}, Gen: m.scopeGen, Hidden: 4})
	got := mustModel(t, next)

	if chip := got.scopeChip(); chip != "fleet · 1 active of 5" {
		t.Errorf("chip = %q, want the hidden repos counted", chip)
	}

	back, _ := pressKeys(t, got, "f")
	if back.fleetScope || back.scanPaths[0] != "/dev" || back.scopeChip() != "" {
		t.Errorf("a second press must return to the launch roots, got %v %v", back.fleetScope, back.scanPaths)
	}
}

func TestDetailToggleHintAndKeyByWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		width    int
		wantHint bool
		wantOpen bool
	}{
		{80, false, false},
		{120, true, true},
		{200, false, false},
	}

	for _, tc := range tests {
		m := compactModel(tc.width, 40)

		hasHint := len(m.detailHints()) > 0
		if hasHint != tc.wantHint {
			t.Errorf("width %d: v hint shown = %v, want %v", tc.width, hasHint, tc.wantHint)
		}

		if toggled, _ := pressKeys(t, m, "v"); toggled.detailOpen != tc.wantOpen {
			t.Errorf("width %d: v left detailOpen = %v, want %v", tc.width, toggled.detailOpen, tc.wantOpen)
		}
	}
}
