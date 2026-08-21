package app

import (
	"strconv"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// overviewEmpty is what a panel with nothing to list shows. The overview's own
// rows use emDash instead: a column of dashes reads as "nothing here" at a
// glance, where a column of the word "none" has to be read.
const overviewEmpty = "none"

// overviewFullRows is how many rows the non-compact pane holds.
const overviewFullRows = 8

// Row and panel labels shared between the overview pane and the panel grid, so
// a rename cannot leave the two disagreeing.
const (
	rowNameSync     = "Sync"
	rowNameFiles    = "Files"
	tabNameBranches = "Branches"
	tabNamePeers    = "Peers"
	tabNameStashes  = "Stashes"
	tabNamePRs      = "PRs"
	tabNameNotes    = "Notes"
)

type overviewRow struct {
	label string
	value string
}

func overviewIdentity(s models.RepoSummary) string {
	parts := []string{s.VCSType.String()}
	if s.RemoteProtocol != "" {
		parts = append(parts, s.RemoteProtocol)
	}

	return strings.Join(parts, compactSignalSep)
}

// overviewRows builds the panel's body in a fixed order, so the same line
// always sits at the same height whatever the repo's state. The compact
// layout keeps only Sync and Files: at that width the rest costs more rows
// than the answers are worth.
//
// Each row names what it is waiting on rather than what it would say if the
// answer were in: a row read from the repo's own summary waits on that summary,
// Peers waits on every summary because a peer is another repo, and the rows
// with a fetch of their own wait on that fetch as well.
func (m Model) overviewRows(s models.RepoSummary, compact bool) []overviewRow {
	unread := m.summaryPending(s.Path)

	rows := make([]overviewRow, 0, overviewFullRows)
	rows = append(rows,
		overviewRow{label: rowNameSync, value: section(unread, overviewSync(s))},
		overviewRow{label: rowNameFiles, value: section(unread, overviewFiles(s))},
	)

	if compact {
		return rows
	}

	return append(rows,
		overviewRow{label: tabNamePeers, value: section(m.loading, overviewPeers(m.PeerCheckouts(s.Path)))},
		overviewRow{label: tabNameStashes, value: section(unread, overviewStashes(s))},
		overviewRow{label: tabNameNotes, value: section(unread, overviewNotes(s.NotesFiles))},
		overviewRow{
			label: "Template",
			value: section(unread || m.fetchPending(s.Path, fetchTemplate), formatCopierCell(s, maxContentWidth)),
		},
		overviewRow{label: tabNamePRs, value: section(unread || m.fetchPending(s.Path, fetchPR), overviewPRs(s))},
		overviewRow{label: "CI", value: section(unread || m.fetchPending(s.Path, fetchCI), m.overviewCI(s))},
	)
}

// overviewCI reports the default branch's CI rollup, naming the branch it
// belongs to so the line is not mistaken for the current branch's checks.
func (m Model) overviewCI(s models.RepoSummary) string {
	text := m.ciCell(s)

	if branch := m.ciBranch[s.Path]; branch != "" {
		return text + " on " + branch
	}

	return text
}

// overviewSync reports the branch's position against its upstream. With no
// upstream there is nothing to be in sync with, so the row says so rather than
// claiming a position against a remote that does not exist.
func overviewSync(s models.RepoSummary) string {
	switch {
	case s.NoCommits:
		return "no commits"
	case s.Upstream == "":
		return "no upstream"
	}

	position := "in sync"
	switch {
	case s.Ahead > 0 && s.Behind > 0:
		position = "↑" + strconv.Itoa(s.Ahead) + " ↓" + strconv.Itoa(s.Behind)
	case s.Ahead > 0:
		position = "↑" + strconv.Itoa(s.Ahead)
	case s.Behind > 0:
		position = "↓" + strconv.Itoa(s.Behind)
	}

	return position + " vs " + s.Upstream
}

// overviewFiles reports the working tree alone. Unpushed commits belong to the
// Sync row, so a repo that is only ahead reads as clean here.
func overviewFiles(s models.RepoSummary) string {
	if s.UncommittedCount() == 0 {
		return "clean"
	}

	counts := []struct {
		count int
		label string
	}{
		{s.Staged, "staged"},
		{s.Unstaged, models.ModifiedFilesLabel(s.VCSType)},
		{s.Untracked, "untracked"},
		{s.Conflicted, "conflicted"},
	}

	parts := make([]string, 0, len(counts))
	for _, c := range counts {
		if c.count > 0 {
			parts = append(parts, strconv.Itoa(c.count)+" "+c.label)
		}
	}

	return strings.Join(parts, compactSignalSep)
}

// overviewStashes counts stashes, naming jj's absence of them rather than
// reporting zero as though the repo simply had none.
func overviewStashes(s models.RepoSummary) string {
	if s.VCSType == models.VCSTypeJJ {
		return "n/a"
	}

	return overviewCount(s.StashCount)
}

// overviewPRs reports the pull request open on the current branch.
func overviewPRs(s models.RepoSummary) string {
	if s.PRInfo == nil {
		return emDash
	}

	return formatPRCell(s) + " " + s.PRInfo.Title
}

// overviewPeers lists the fleet's other checkouts of this repo, naming the
// branch alongside any peer that holds one of its own. A peer just sitting on
// the default branch is not doing anything a reader needs to know about, so
// it names only its folder.
func overviewPeers(peers []models.PeerCheckout) string {
	if len(peers) == 0 {
		return emDash
	}

	folders := make([]string, 0, len(peers))
	for _, peer := range peers {
		folder := peer.Folder()
		if peer.Branch != "" && !models.IsDefaultBranchName(peer.Branch) {
			folder += " (" + peer.Branch + ")"
		}

		folders = append(folders, folder)
	}

	return "⧉ " + strconv.Itoa(len(peers)) + " " + strings.Join(folders, ", ")
}

// overviewRelevantPeers summarizes the peer checkouts holding a branch that
// tracks one of this repo's open pull requests. Each names its own directory
// rather than just its folder, since a worktree's folder name alone does not
// say which checkout it is, and is tagged when discovery found it under a
// different scan root than the repo currently open.
func overviewRelevantPeers(peers []relevantPeer) string {
	if len(peers) == 0 {
		return emDash
	}

	entries := make([]string, 0, len(peers))
	for i := range peers {
		peer := &peers[i]
		entry := "#" + strconv.Itoa(peer.PR.Number) + " " + peer.Kind() + " at " + peer.Path
		if peer.OtherScanRoot {
			entry += " (other scan root)"
		}

		entries = append(entries, entry)
	}

	return "⧉ " + strconv.Itoa(len(peers)) + " " + strings.Join(entries, "; ")
}

func overviewCount(count int) string {
	if count == 0 {
		return emDash
	}

	return strconv.Itoa(count)
}

func overviewNotes(notes []models.NoteFile) string {
	if len(notes) == 0 {
		return emDash
	}

	// One note is the common case, and its first line says more than its
	// filename does; several fall back to naming them.
	if len(notes) == 1 && notes[0].FirstLine != "" {
		return notes[0].Name + ": " + notes[0].FirstLine
	}

	names := make([]string, 0, len(notes))
	for _, note := range notes {
		names = append(names, note.Name)
	}

	return strings.Join(names, ", ")
}
