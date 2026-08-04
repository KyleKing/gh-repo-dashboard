package models

import (
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// PeerCheckout is another working directory holding the same remote repository:
// a sibling clone discovered in the same scan, or a git worktree / jj workspace.
type PeerCheckout struct {
	Path       string
	Branch     string
	Ahead      int
	Behind     int
	LastCommit time.Time
	Dirty      bool
	IsWorktree bool
	IsLocked   bool
}

// Kind reports whether the checkout is a sibling clone or a worktree.
func (p *PeerCheckout) Kind() string {
	if p.IsWorktree {
		return "worktree"
	}

	return "clone"
}

// Folder returns the checkout's directory name.
func (p *PeerCheckout) Folder() string {
	return filepath.Base(p.Path)
}

// TrackingSummary renders the checkout's ahead/behind counts, or a checkmark when in sync.
func (p *PeerCheckout) TrackingSummary() string {
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
			Path:       summary.Path,
			Branch:     summary.Branch,
			Ahead:      summary.Ahead,
			Behind:     summary.Behind,
			LastCommit: summary.LastModified,
			Dirty:      summary.UncommittedCount() > 0,
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
			IsLocked:   wt.IsLocked,
		})
	}

	sortCheckouts(peers)

	return peers
}

// MergeCheckouts concatenates checkout lists, keeping the first entry's
// tracking data for any repeated path so a sibling clone's richer counts win
// over a sparse worktree entry. The worktree and lock flags still carry over,
// because a directory discovered as its own clone is a worktree all the same.
func MergeCheckouts(lists ...[]PeerCheckout) []PeerCheckout {
	index := make(map[string]int)

	var merged []PeerCheckout
	for _, list := range lists {
		for _, checkout := range list {
			at, seen := index[checkout.Path]
			if !seen {
				index[checkout.Path] = len(merged)
				merged = append(merged, checkout)

				continue
			}

			merged[at].IsWorktree = merged[at].IsWorktree || checkout.IsWorktree
			merged[at].IsLocked = merged[at].IsLocked || checkout.IsLocked
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

// ConflictingBranches returns the branches held by more than one checkout of
// the same repo, counting the repo's own branch. Two checkouts on one branch is
// the state that silently loses local commits, so it is worth flagging wherever
// a checkout is named.
func ConflictingBranches(ownBranch string, peers []PeerCheckout) map[string]bool {
	counts := make(map[string]int, len(peers)+1)
	if ownBranch != "" {
		counts[ownBranch]++
	}

	for _, peer := range peers {
		if peer.Branch != "" {
			counts[peer.Branch]++
		}
	}

	conflicts := make(map[string]bool)
	for branch, count := range counts {
		if count > 1 {
			conflicts[branch] = true
		}
	}

	return conflicts
}

func sortCheckouts(peers []PeerCheckout) {
	sort.Slice(peers, func(i, j int) bool { return peers[i].Folder() < peers[j].Folder() })
}
