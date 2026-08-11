package app

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
	// HasLocal marks a row whose head ref is checked out somewhere in the
	// fleet. A fork's head ref is not, however much its name looks local.
	HasLocal bool
}

func (e prMapEntry) HasPR() bool {
	return e.PR != nil
}

// loadPRMapCmd fetches one repo's pull requests and branch list. The pull
// request call is the same per-repo list the detail view already makes; the
// branch list is local git and costs nothing.
func loadPRMapCmd(path, remoteID, upstream string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		var prs []models.PRInfo
		if upstream != "" {
			//nolint:errcheck // best-effort: a repo we cannot reach leaves a gap in the map
			prs, _ = github.GetPRsForRepo(ctx, path, remoteID, upstream)
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

	seen := make(map[string]bool, len(m.filteredPaths))
	for _, path := range m.filteredPaths {
		data, loaded := m.prMap[path]
		if !loaded {
			continue
		}

		identity := vcs.CheckoutIdentity(path)
		if seen[identity] {
			continue
		}
		seen[identity] = true

		entries = append(entries, m.repoPRMapEntries(path, data)...)
	}

	sortPRMap(entries)

	return entries
}

// repoPRMapEntries builds one repo's rows: a row per open pull request, then
// its local branches that no pull request accounts for.
func (m *Model) repoPRMapEntries(path string, data PRMapLoadedMsg) []prMapEntry {
	repo := filepath.Base(path)
	owner := repoOwner(m.summaries[path].RemoteRepo)
	defaultBranch := findDefaultBranch(data.Branches)

	entries := make([]prMapEntry, 0, len(data.PRs)+len(data.Branches))
	claimed := make(map[string]bool, len(data.PRs))

	for i := range data.PRs {
		pr := &data.PRs[i]
		location := "fork: " + pr.HeadRepoOwner
		if !pr.FromFork(owner) {
			claimed[pr.HeadRef] = true
			location = m.locateBranch(path, pr.HeadRef)
		}

		entries = append(entries, prMapEntry{
			Repo:     repo,
			PR:       pr,
			Branch:   pr.HeadLabel(owner),
			Location: location,
			HasLocal: !pr.FromFork(owner) && location != emDash,
		})
	}

	for _, branch := range data.Branches {
		if claimed[branch.Name] || branch.Ahead == 0 || branch.Name == defaultBranch {
			continue
		}

		entries = append(entries, prMapEntry{
			Repo:     repo,
			Branch:   branch.Name,
			Location: "here: " + branch.Name,
			HasLocal: true,
		})
	}

	return entries
}

// repoOwner is the owner half of a "owner/name" remote path.
func repoOwner(remoteRepo string) string {
	owner, _, _ := strings.Cut(remoteRepo, "/")

	return owner
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
			if entry.HasLocal {
				withLocal++
			}
		}
	}

	return strconv.Itoa(open) + " open" + compactSignalSep +
		strconv.Itoa(withLocal) + " with local branch" + compactSignalSep +
		strconv.Itoa(localOnly) + " local-only branches"
}
