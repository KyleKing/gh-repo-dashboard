package app

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

func (m Model) branchDetailScreen() screen {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return screen{title: appTitle, body: m.renderBranchDetail(), hints: m.branchDetailHints()}
	}

	branch := m.branchDetail.Branch

	var facts []frame.Fact
	if branch.IsCurrent {
		facts = append(facts, frame.Fact{Text: "current", Tone: frame.Accent})
	}

	if branch.Ahead > 0 {
		facts = append(facts, frame.Fact{Text: fmt.Sprintf("↑%d", branch.Ahead), Tone: frame.Warning})
	}

	if branch.Behind > 0 {
		facts = append(facts, frame.Fact{Text: fmt.Sprintf("↓%d", branch.Behind)})
	}

	return screen{
		title: "Repos › " + summary.Name() + " › " + branch.Name,
		facts: facts,
		body:  m.renderBranchDetail(),
		hints: m.branchDetailHints(),
	}
}

// branchDetailHints are the branch view's own keys, in the order they are
// worth offering: the ones that act on the branch go before the ones that only
// leave.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func (m Model) branchDetailHints() []keyhint.Hint {
	hints := []keyhint.Hint{
		{Key: "b", What: "copy branch name", Priority: 3},
		{Key: "c", What: "switch", Priority: 6},
		{Key: "p", What: "push", Priority: 5},
	}

	if m.branchDetail.PRInfo == nil {
		hints = append(hints, keyhint.Hint{Key: "N", What: "new PR", Priority: 4})
	} else {
		hints = append(hints,
			keyhint.Hint{Key: "o", What: "open PR URL", Priority: 4},
			keyhint.Hint{Key: "M", What: "squash-merge", Priority: 2})
	}

	return append(hints,
		keyhint.Hint{Key: keyEsc, What: descBack, Priority: 9},
		keyhint.Hint{Key: "?", What: nameHelp, Priority: 7})
}

// branchDetailStyles are the shared styles used across renderBranchDetail's sections.
type branchDetailStyles struct {
	section lipgloss.Style
	info    lipgloss.Style
	label   lipgloss.Style
}

func newBranchDetailStyles() branchDetailStyles {
	return branchDetailStyles{
		section: styles.Skin.Heading.PaddingLeft(1),
		info:    styles.Skin.Body.PaddingLeft(infoPaddingLeft),
		label:   styles.Skin.Subtitle.Width(detailLabelWidth),
	}
}

// writeInfoLine renders "label: value" through the shared info style and
// appends it (plus a trailing newline) to b.
func (s branchDetailStyles) writeInfoLine(b *strings.Builder, label, value string) {
	b.WriteString(s.info.Render(s.label.Render(label) + " " + value))
	b.WriteString("\n")
}

// aheadBehindStatus renders an ahead/behind pair through AheadStyle/BehindStyle,
// or empty if both are zero.
func aheadBehindStatus(ahead, behind int) string {
	status := ""
	if ahead > 0 {
		status += styles.Ahead.Render(fmt.Sprintf("↑%d ahead", ahead))
	}
	if behind > 0 {
		if status != "" {
			status += " "
		}
		status += styles.Behind.Render(fmt.Sprintf("↓%d behind", behind))
	}

	return status
}

func (m Model) writeBranchInfoSection(b *strings.Builder, s branchDetailStyles) {
	b.WriteString(s.section.Render("Branch Information"))
	b.WriteString("\n\n")

	branch := m.branchDetail.Branch
	if branch.Upstream != "" {
		s.writeInfoLine(b, "Upstream:", branch.Upstream)
	}

	if branch.Ahead > 0 || branch.Behind > 0 {
		s.writeInfoLine(b, "Tracking:", aheadBehindStatus(branch.Ahead, branch.Behind))
	}

	m.writeCheckoutLine(b, s)
	m.writeDefaultBranchComparison(b, s)

	if len(m.branchDetail.Commits) > 0 {
		lastCommit := m.branchDetail.Commits[0]
		s.writeInfoLine(b, "Last commit:", display.CommitRelativeDate(lastCommit))
		s.writeInfoLine(b, "Author:", lastCommit.Author)
	}

	fileChanges := m.branchDetail.FileChangesSummary(m.summaries[m.selectedRepo].VCSType)
	fileStyle := s.info
	if m.branchDetail.UncommittedCount() > 0 {
		fileStyle = styles.Skin.Warning.PaddingLeft(infoPaddingLeft)
	}
	b.WriteString(fileStyle.Render(s.label.Render("File changes:") + " " + fileChanges))
	b.WriteString("\n")

	if summary := m.summaries[m.selectedRepo]; summary.VCSType == vcs.TypeJJ {
		if m.branchDetail.ChangeID != "" {
			s.writeInfoLine(b, "Change ID:", styles.Skin.Subtitle.Render(m.branchDetail.ChangeID))
		}
		if m.branchDetail.Description != "" {
			s.writeInfoLine(b, "Description:", truncate(m.branchDetail.Description, descriptionTruncLen))
		}
	}
}

// writeCheckoutLine reports where the branch is checked out: this working
// directory, or the parallel checkout holding it along with that checkout's
// own tracking and dirty state.
func (m Model) writeCheckoutLine(b *strings.Builder, s branchDetailStyles) {
	if m.branchDetail.Branch.IsCurrent {
		s.writeInfoLine(b, "Checked out in:", styles.Skin.Success.Render("here"))
		return
	}

	checkout, ok := models.CheckoutForBranch(m.RepoCheckouts(), m.branchDetail.Branch.Name)
	if !ok {
		return
	}

	detail := checkout.Folder() + " (" + checkout.TrackingSummary()
	if checkout.Dirty {
		detail += ", dirty"
	}
	detail += ")"

	s.writeInfoLine(b, "Checked out in:", styles.Skin.Warning.Render("⧉ "+detail))
}

func (m Model) writeDefaultBranchComparison(b *strings.Builder, s branchDetailStyles) {
	detail := m.branchDetail
	if detail.DefaultBranch == "" || detail.Branch.Name == detail.DefaultBranch {
		return
	}

	status := aheadBehindStatus(detail.DefaultAhead, detail.DefaultBehind)
	if status == "" {
		status = styles.Skin.Success.Render("up to date")
	}
	s.writeInfoLine(b, "vs "+detail.DefaultBranch+":", status)
}

func (m Model) writeBranchPRSection(b *strings.Builder, s branchDetailStyles) {
	if m.branchDetail.PRInfo == nil && m.branchDetail.WorkflowInfo == nil {
		return
	}

	b.WriteString("\n")
	b.WriteString(s.section.Render("Pull Request & CI/CD"))
	b.WriteString("\n\n")

	if pr := m.branchDetail.PRInfo; pr != nil {
		writeBranchPRInfo(b, s, pr)
	}

	if wf := m.branchDetail.WorkflowInfo; wf != nil {
		wfStatus := display.WorkflowSummaryStatusDisplay(*wf)
		wfStyle := styles.Skin.Subtitle
		switch wfStatus {
		case forge.StatusPassing:
			wfStyle = styles.Skin.Success
		case forge.StatusFailing:
			wfStyle = styles.Skin.Error
		}
		wfDetail := fmt.Sprintf("%s (%d/%d passing)", wfStatus, wf.Passing, wf.Total)
		s.writeInfoLine(b, "Workflows:", wfStyle.Render(wfDetail))
	}
}

func writeBranchPRInfo(b *strings.Builder, s branchDetailStyles, pr *forge.PullRequest) {
	prStatus := display.PRStatusDisplay(*pr)
	prStyle := styles.PROpen
	switch prStatus {
	case forge.PRStatusMerged:
		prStyle = styles.Skin.Success
	case forge.PRStatusClosed:
		prStyle = styles.Skin.Subtitle
	}

	s.writeInfoLine(b, "PR:", prStyle.Render(fmt.Sprintf("#%d %s", pr.Number, prStatus)))
	s.writeInfoLine(b, "Title:", truncate(pr.Title, descriptionTruncLen))

	reviewStatus := display.PRReviewStatus(*pr)
	reviewStyle := styles.Skin.Subtitle
	switch reviewStatus {
	case forge.ReviewApproved:
		reviewStyle = styles.Skin.Success
	case forge.ReviewChangesRequested:
		reviewStyle = styles.Skin.Error
	}
	s.writeInfoLine(b, "Review:", reviewStyle.Render(reviewStatus))

	if len(pr.ApprovedBy) > 0 {
		approvers := strings.Join(pr.ApprovedBy, ", ")
		s.writeInfoLine(b, "Approved by:", truncate(approvers, descriptionTruncLen))
	}

	if pr.Checks.Total > 0 {
		checkStatus := display.ChecksSummary(pr.Checks)
		checkStyle := styles.Skin.Subtitle
		switch checkStatus {
		case forge.StatusPassing:
			checkStyle = styles.Skin.Success
		case forge.StatusFailing:
			checkStyle = styles.Skin.Error
		}
		checkDetail := fmt.Sprintf("%s (%d/%d passing)", checkStatus, pr.Checks.Passing, pr.Checks.Total)
		s.writeInfoLine(b, "Checks:", checkStyle.Render(checkDetail))
	}
}

func (m Model) writeBranchCommitsSection(b *strings.Builder, s branchDetailStyles) {
	b.WriteString("\n")
	b.WriteString(s.section.Render("Recent Commits"))
	b.WriteString("\n\n")

	if len(m.branchDetail.Commits) == 0 {
		if m.branchDetailLoading {
			b.WriteString(m.loadingPlaceholder("Loading commits"))
		} else {
			b.WriteString(m.emptyPlaceholder("No commits found", ""))
		}

		return
	}

	layout := fitDetailCols(commitColSpecs, m.viewWidth())
	subtleStyles := map[string]lipgloss.Style{
		colCommitHash:   styles.Skin.Subtitle,
		colCommitAuthor: styles.Skin.Subtitle,
		colCommitDate:   styles.Skin.Subtitle,
	}
	maxCommits := min(branchDetailMaxCommits, len(m.branchDetail.Commits))

	for i := range maxCommits {
		commit := m.branchDetail.Commits[i]
		values := map[string]string{
			colCommitHash:    commit.ShortHash,
			colCommitSubject: commit.Subject,
			colCommitAuthor:  commit.Author,
			colCommitDate:    display.CommitRelativeDate(commit),
		}

		cells := renderCells(layout, values, subtleStyles, &plainStyle)
		b.WriteString(detailRow(strings.Repeat(" ", gutterWidth), layout, cells, plainStyle))
		b.WriteString("\n")
	}
}

func (m Model) renderBranchDetail() string {
	var b strings.Builder

	s := newBranchDetailStyles()
	m.writeBranchInfoSection(&b, s)
	m.writeBranchPRSection(&b, s)
	m.writeBranchCommitsSection(&b, s)

	return strings.TrimRight(b.String(), "\n")
}
