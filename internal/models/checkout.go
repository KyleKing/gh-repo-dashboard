package models

import (
	"path/filepath"
	"sort"
	"strconv"
)

// PeerCheckout is another working directory holding the same remote repository:
// a sibling clone discovered in the same scan, or a git worktree / jj workspace.
type PeerCheckout struct {
	Path       string
	Branch     string
	Ahead      int
	Behind     int
	Dirty      bool
	IsWorktree bool
}

// Folder returns the checkout's directory name.
func (p PeerCheckout) Folder() string {
	return filepath.Base(p.Path)
}

// TrackingSummary renders the checkout's ahead/behind counts, or a checkmark when in sync.
func (p PeerCheckout) TrackingSummary() string {
	summary := ""
	if p.Ahead > 0 {
		summary += "↑" + strconv.Itoa(p.Ahead)
	}
	if p.Behind > 0 {
		if summary != "" {
			summary += " "
		}
		summary += "↓" + strconv.Itoa(p.Behind)
	}
	if summary == "" {
		return "✓"
	}

	return summary
}

// FindPeerCheckouts returns the other discovered repos sharing current's remote,
// sorted by folder name. Repos without a known remote never peer with anything,
// since an empty remote would otherwise group every unrelated local-only repo.
func FindPeerCheckouts(current *RepoSummary, all []RepoSummary) []PeerCheckout {
	if current == nil || current.RemoteRepo == "" {
		return nil
	}

	var peers []PeerCheckout
	for i := range all {
		summary := &all[i]
		if summary.Path == current.Path || summary.RemoteRepo != current.RemoteRepo {
			continue
		}

		peers = append(peers, PeerCheckout{
			Path:   summary.Path,
			Branch: summary.Branch,
			Ahead:  summary.Ahead,
			Behind: summary.Behind,
			Dirty:  summary.UncommittedCount() > 0,
		})
	}

	sortCheckouts(peers)

	return peers
}

// WorktreeCheckouts converts a repo's worktree list into peer checkouts,
// dropping the repo's own working directory and any bare entry.
func WorktreeCheckouts(repoPath string, worktrees []WorktreeInfo) []PeerCheckout {
	var peers []PeerCheckout
	for _, wt := range worktrees {
		if wt.IsBare || wt.Path == "" || wt.Path == repoPath {
			continue
		}

		peers = append(peers, PeerCheckout{
			Path:       wt.Path,
			Branch:     wt.Branch,
			IsWorktree: true,
		})
	}

	sortCheckouts(peers)

	return peers
}

// MergeCheckouts concatenates checkout lists, keeping the first entry for any
// repeated path so a sibling clone's richer tracking data wins over a bare
// worktree entry for the same directory.
func MergeCheckouts(lists ...[]PeerCheckout) []PeerCheckout {
	seen := make(map[string]bool)

	var merged []PeerCheckout
	for _, list := range lists {
		for _, checkout := range list {
			if seen[checkout.Path] {
				continue
			}
			seen[checkout.Path] = true
			merged = append(merged, checkout)
		}
	}

	sortCheckouts(merged)

	return merged
}

// CheckoutForBranch returns the peer checkout that has branch checked out, if any.
func CheckoutForBranch(peers []PeerCheckout, branch string) (PeerCheckout, bool) {
	if branch == "" {
		return PeerCheckout{}, false
	}

	for _, peer := range peers {
		if peer.Branch == branch {
			return peer, true
		}
	}

	return PeerCheckout{}, false
}

func sortCheckouts(peers []PeerCheckout) {
	sort.Slice(peers, func(i, j int) bool { return peers[i].Folder() < peers[j].Folder() })
}
