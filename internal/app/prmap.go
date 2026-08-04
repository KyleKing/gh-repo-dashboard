package app

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

// PRMapLoadedMsg carries one repo's open pull requests and local branches for
// the fleet map. Failures arrive as empty slices, so one unreachable repo
// leaves a gap in the map rather than blocking it.
type PRMapLoadedMsg struct {
	Path     string
	PRs      []models.PRInfo
	Branches []models.BranchInfo
}

// prMapEntry is one row of the fleet map: an open pull request and where its
// head ref lives locally, or a local branch with commits and no pull request.
type prMapEntry struct {
	Repo     string
	PR       *models.PRInfo
	Branch   string
	Location string
}

func (e prMapEntry) HasPR() bool {
	return e.PR != nil
}

// loadPRMapCmd fetches one repo's pull requests and branch list. The pull
// request call is the same per-repo list the detail view already makes; the
// branch list is local git and costs nothing.
func loadPRMapCmd(path, upstream string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		var prs []models.PRInfo
		if upstream != "" {
			//nolint:errcheck // best-effort: a repo we cannot reach leaves a gap in the map
			prs, _ = github.GetPRsForRepo(ctx, path, upstream)
		}

		//nolint:errcheck // best-effort, see above
		branches, _ := vcs.GetOperations(path).GetBranchList(ctx, path)

		return PRMapLoadedMsg{Path: path, PRs: prs, Branches: branches}
	}
}

const localOnlyLabel = "(no PR)"

// buildPRMap joins the loaded pull requests against the loaded branch lists,
// answering both "where is the branch for PR #N" and "which local branches
// have no PR". Rows sort by repo, then by descending PR number, with each
// repo's local-only branches last.
func (m *Model) buildPRMap() []prMapEntry {
	var entries []prMapEntry

	for _, path := range m.filteredPaths {
		data, loaded := m.prMap[path]
		if !loaded {
			continue
		}

		repo := filepath.Base(path)
		claimed := make(map[string]bool, len(data.PRs))

		for i := range data.PRs {
			pr := &data.PRs[i]
			claimed[pr.HeadRef] = true
			entries = append(entries, prMapEntry{
				Repo:     repo,
				PR:       pr,
				Branch:   pr.HeadRef,
				Location: m.locateBranch(path, pr.HeadRef),
			})
		}

		for _, branch := range data.Branches {
			if claimed[branch.Name] || branch.Ahead == 0 {
				continue
			}

			entries = append(entries, prMapEntry{
				Repo:     repo,
				Branch:   branch.Name,
				Location: "here: " + branch.Name,
			})
		}
	}

	sortPRMap(entries)

	return entries
}

// locateBranch names the checkout holding ref: this repo, a peer checkout, or
// nowhere local. Only cached branch lists and checkout data are consulted, so
// the join costs no API calls.
func (m *Model) locateBranch(path, ref string) string {
	if data, ok := m.prMap[path]; ok {
		for _, branch := range data.Branches {
			if branch.Name == ref {
				return "here: " + ref
			}
		}
	}

	for _, peer := range m.PeerCheckouts(path) {
		if peer.Branch == ref {
			return "peer: " + peer.Folder()
		}
	}

	return emDash
}

func sortPRMap(entries []prMapEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i], entries[j]
		if left.Repo != right.Repo {
			return left.Repo < right.Repo
		}

		if left.HasPR() != right.HasPR() {
			return left.HasPR()
		}

		if left.HasPR() {
			return left.PR.Number > right.PR.Number
		}

		return left.Branch < right.Branch
	})
}

func prMapSummary(entries []prMapEntry) string {
	open, withLocal, localOnly := 0, 0, 0
	for _, entry := range entries {
		switch {
		case !entry.HasPR():
			localOnly++
		default:
			open++
			if entry.Location != emDash {
				withLocal++
			}
		}
	}

	return strconv.Itoa(open) + " open" + compactSignalSep +
		strconv.Itoa(withLocal) + " with local branch" + compactSignalSep +
		strconv.Itoa(localOnly) + " local-only branches"
}
