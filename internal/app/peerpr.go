package app

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// PeerBranchesLoadedMsg carries the local branch lists read for one repo's
// peer checkouts, keyed by each peer's own path rather than by the repo that
// requested them, so two rows sharing a peer share one fetch.
type PeerBranchesLoadedMsg struct {
	Path     string
	Branches map[string][]vcs.BranchInfo
}

// loadPeerBranchesCmd reads the local branch list for each of peerPaths. The
// read is local git, already cached per path by vcs.Operations, so re-reading
// a peer another row also names costs nothing extra.
func loadPeerBranchesCmd(path string, peerPaths []string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		branches := make(map[string][]vcs.BranchInfo, len(peerPaths))
		for _, peerPath := range peerPaths {
			//nolint:errcheck // best-effort: a peer we cannot read leaves it out of the relevant set
			list, _ := vcs.GetOperations(peerPath).GetBranchList(ctx, peerPath)
			branches[peerPath] = list
		}

		return PeerBranchesLoadedMsg{Path: path, Branches: branches}
	}
}

// relevantPeer is a peer checkout holding a local branch that tracks one of
// path's open pull requests, matched on upstream ref rather than local branch
// name or current-checkout state, so a renamed branch or a second branch
// sitting uncommitted still counts.
type relevantPeer struct {
	models.PeerCheckout
	PR forge.PullRequest
	// OtherScanRoot marks a peer discovered under a different configured scan
	// root than path, so a caller can label it rather than let it read as
	// though it were found alongside the repo currently open.
	OtherScanRoot bool
}

// relevantPeers returns path's peer checkouts that have a local branch
// tracking one of its open pull requests. A peer whose branch list has not
// loaded yet is left out rather than assumed irrelevant, so a slow fetch
// never flashes a wrong answer. Only pull requests already known (from the
// row's own fleet-map fetch) are considered.
func (m Model) relevantPeers(path string) []relevantPeer {
	data, ok := m.prMap[path]
	if !ok || len(data.PRs) == 0 {
		return nil
	}

	owner := repoOwner(m.summaries[path].RemoteRepo)
	root := m.scanRootOf(path)

	var relevant []relevantPeer
	for _, peer := range m.PeerCheckouts(path) {
		branches, loaded := m.peerBranches[peer.Path]
		if !loaded {
			continue
		}

		if pr := matchingPR(branches, data.PRs, owner); pr != nil {
			relevant = append(relevant, relevantPeer{
				PeerCheckout:  peer,
				PR:            *pr,
				OtherScanRoot: m.scanRootOf(peer.Path) != root,
			})
		}
	}

	return relevant
}

// matchingPR returns the first of prs that one of branches tracks, or nil.
// Owner is the repo's own owner, so a fork's head ref (which shares a
// namespace with local branches) is never mistaken for being present here.
func matchingPR(branches []vcs.BranchInfo, prs []forge.PullRequest, owner string) *forge.PullRequest {
	for i := range prs {
		pr := &prs[i]
		for _, branch := range branches {
			if pr.MatchesUpstream(owner, branch.Upstream) {
				return pr
			}
		}
	}

	return nil
}

// scanRootOf returns the longest configured scan path that contains path, so
// two checkouts can be compared for whether discovery found them under the
// same root or two different ones.
func (m Model) scanRootOf(path string) string {
	best := ""
	for _, root := range m.scanPaths {
		if strings.HasPrefix(path, root) && len(root) > len(best) {
			best = root
		}
	}

	return best
}
