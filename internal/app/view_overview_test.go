//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
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

func TestOverviewSyncNamesWhatIsMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		summary models.RepoSummary
		want    string
	}{
		{"never pushed", models.RepoSummary{}, "no upstream"},
		{"empty repo", models.RepoSummary{NoCommits: true}, "no commits"},
		{"in sync", models.RepoSummary{Upstream: "origin/main"}, "in sync vs origin/main"},
		{"ahead", models.RepoSummary{Upstream: "origin/main", Ahead: 2}, "↑2 vs origin/main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := overviewSync(tt.summary); got != tt.want {
				t.Errorf("overviewSync = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOverviewFilesWordsJJWithoutAStagingArea(t *testing.T) {
	t.Parallel()

	git := models.RepoSummary{VCSType: models.VCSTypeGit, Unstaged: 2}
	if got := overviewFiles(git); !strings.Contains(got, "2 unstaged") {
		t.Errorf("git files = %q, want it to say unstaged", got)
	}

	jj := models.RepoSummary{VCSType: models.VCSTypeJJ, Unstaged: 2}
	if got := overviewFiles(jj); strings.Contains(got, "unstaged") || !strings.Contains(got, "2 changed") {
		t.Errorf("jj files = %q, want it to avoid a staging distinction jj does not have", got)
	}
}

func TestDirtyLabelSeparatesUnpushedFromUncommitted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		summary models.RepoSummary
		want    string
	}{
		{"neither", models.RepoSummary{}, ""},
		{"ahead only", models.RepoSummary{Ahead: 1}, "unpushed"},
		{"files only", models.RepoSummary{Unstaged: 1}, "uncommitted"},
		{"both", models.RepoSummary{Ahead: 1, Staged: 1}, "uncommitted, unpushed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.summary.DirtyLabel(); got != tt.want {
				t.Errorf("DirtyLabel = %q, want %q", got, tt.want)
			}
		})
	}
}
