//nolint:testpackage // Model internals are tested directly by design; see ROADMAP.md
package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// TestListWidthFollowsTheWiderRule pins the redesign's geometry: the table
// stands alone and takes the focused grid's width wherever that is the wider
// rule, so a terminal that used to carry the preview panel spends the whole of
// it on columns, and one that never did keeps every cell it had.
func TestListWidthFollowsTheWiderRule(t *testing.T) {
	t.Parallel()

	for _, width := range []int{80, 100, 140, 160, 180, 220, 260} {
		m := compactModel(width, 40)

		if got, floor := m.frameWidth(), contentWidth(width); got < floor {
			t.Errorf("at %d cells the list frames to %d, narrower than the %d it had", width, got, floor)
		}

		if grid := m.gridWidth(); grid > contentWidth(width) && m.frameWidth() != grid {
			t.Errorf("at %d cells the list frames to %d and the grid to %d", width, m.frameWidth(), grid)
		}

		if got := layoutRepoCols(listWidth(width)).Hidden; width >= wideMinWidth && got != 0 {
			t.Errorf("at %d cells the table hides %d columns; there is width for all of them", width, got)
		}
	}
}

func TestExpandRegionTracksTheCursor(t *testing.T) {
	t.Parallel()

	m := compactModel(180, 40)
	m.expandOpen = true

	first := plainText(m.renderListBody())
	if !strings.Contains(first, "dev/alpha") {
		t.Fatalf("the region does not name the first repo:\n%s", first)
	}

	m.cursor = 1
	second := plainText(m.renderListBody())
	if !strings.Contains(second, "dev/bravo") {
		t.Errorf("the region did not follow the cursor to bravo:\n%s", second)
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
		{"empty repo", models.RepoSummary{RepoSummary: vcs.RepoSummary{NoCommits: true}}, "no commits"},
		{
			"in sync",
			models.RepoSummary{RepoSummary: vcs.RepoSummary{Upstream: "origin/main"}},
			"in sync vs origin/main",
		},
		{
			"ahead",
			models.RepoSummary{RepoSummary: vcs.RepoSummary{Upstream: "origin/main", Ahead: 2}},
			"↑2 vs origin/main",
		},
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

	git := models.RepoSummary{RepoSummary: vcs.RepoSummary{VCSType: vcs.TypeGit, Unstaged: 2}}
	if got := overviewFiles(git); !strings.Contains(got, "2 unstaged") {
		t.Errorf("git files = %q, want it to say unstaged", got)
	}

	jj := models.RepoSummary{RepoSummary: vcs.RepoSummary{VCSType: vcs.TypeJJ, Unstaged: 2}}
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
		{"ahead only", models.RepoSummary{RepoSummary: vcs.RepoSummary{Ahead: 1}}, "unpushed"},
		{"files only", models.RepoSummary{RepoSummary: vcs.RepoSummary{Unstaged: 1}}, "uncommitted"},
		{"both", models.RepoSummary{RepoSummary: vcs.RepoSummary{Ahead: 1, Staged: 1}}, "uncommitted, unpushed"},
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
