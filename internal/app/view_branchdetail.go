package app

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

func (m Model) renderBranchDetailBreadcrumbs() string {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return styles.TitleStyle.Render("repo-dashboard")
	}

	home := styles.SubtitleStyle.Render("Repos")
	sep := styles.SubtitleStyle.Render(" > ")
	repo := styles.BranchStyle.Render(summary.Name())
	branch := styles.TitleStyle.Render(m.branchDetail.Branch.Name)

	var badges []string
	if m.branchDetail.Branch.IsCurrent {
		badges = append(badges, styles.Badge("current", styles.PROpenStyle))
	}
	if m.branchDetail.Branch.Ahead > 0 {
		badges = append(badges, styles.Badge(fmt.Sprintf("↑%d", m.branchDetail.Branch.Ahead), styles.AheadStyle))
	}
	if m.branchDetail.Branch.Behind > 0 {
		badges = append(badges, styles.Badge(fmt.Sprintf("↓%d", m.branchDetail.Branch.Behind), styles.BehindStyle))
	}

	return home + sep + repo + sep + branch + "  " + strings.Join(badges, " ")
}

// branchDetailStyles are the shared styles used across renderBranchDetail's sections.
type branchDetailStyles struct {
	section lipgloss.Style
	info    lipgloss.Style
	label   lipgloss.Style
}

func newBranchDetailStyles() branchDetailStyles {
	return branchDetailStyles{
		section: lipgloss.NewStyle().Foreground(styles.Blue).Bold(true).PaddingLeft(1),
		info:    lipgloss.NewStyle().Foreground(styles.Text).PaddingLeft(infoPaddingLeft),
		label:   lipgloss.NewStyle().Foreground(styles.Subtext0).Width(detailLabelWidth),
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
		status += styles.AheadStyle.Render(fmt.Sprintf("↑%d ahead", ahead))
	}
	if behind > 0 {
		if status != "" {
			status += " "
		}
		status += styles.BehindStyle.Render(fmt.Sprintf("↓%d behind", behind))
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

	m.writeDefaultBranchComparison(b, s)

	if len(m.branchDetail.Commits) > 0 {
		lastCommit := m.branchDetail.Commits[0]
		s.writeInfoLine(b, "Last commit:", lastCommit.RelativeDate())
		s.writeInfoLine(b, "Author:", lastCommit.Author)
	}

	fileChanges := m.branchDetail.FileChangesSummary()
	fileStyle := s.info
	if m.branchDetail.UncommittedCount() > 0 {
		fileStyle = lipgloss.NewStyle().Foreground(styles.Peach).PaddingLeft(infoPaddingLeft)
	}
	b.WriteString(fileStyle.Render(s.label.Render("File changes:") + " " + fileChanges))
	b.WriteString("\n")

	if summary := m.summaries[m.selectedRepo]; summary.VCSType == models.VCSTypeJJ {
		if m.branchDetail.ChangeID != "" {
			s.writeInfoLine(b, "Change ID:", styles.SubtitleStyle.Render(m.branchDetail.ChangeID))
		}
		if m.branchDetail.Description != "" {
			s.writeInfoLine(b, "Description:", truncate(m.branchDetail.Description, descriptionTruncLen))
		}
	}
}

func (m Model) writeDefaultBranchComparison(b *strings.Builder, s branchDetailStyles) {
	detail := m.branchDetail
	if detail.DefaultBranch == "" || detail.Branch.Name == detail.DefaultBranch {
		return
	}

	status := aheadBehindStatus(detail.DefaultAhead, detail.DefaultBehind)
	if status == "" {
		status = styles.CleanStyle.Render("up to date")
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
		wfStatus := wf.StatusDisplay()
		wfStyle := styles.SubtitleStyle
		switch wfStatus {
		case "passing":
			wfStyle = styles.CleanStyle
		case models.StatusFailing:
			wfStyle = styles.ErrorStyle
		}
		wfDetail := fmt.Sprintf("%s (%d/%d passing)", wfStatus, wf.Passing, wf.Total)
		s.writeInfoLine(b, "Workflows:", wfStyle.Render(wfDetail))
	}
}

func writeBranchPRInfo(b *strings.Builder, s branchDetailStyles, pr *models.PRInfo) {
	prStatus := pr.StatusDisplay()
	prStyle := styles.PROpenStyle
	switch prStatus {
	case models.PRStatusMerged:
		prStyle = styles.CleanStyle
	case models.PRStatusClosed:
		prStyle = styles.SubtitleStyle
	}

	s.writeInfoLine(b, "PR:", prStyle.Render(fmt.Sprintf("#%d %s", pr.Number, prStatus)))
	s.writeInfoLine(b, "Title:", truncate(pr.Title, descriptionTruncLen))

	reviewStatus := pr.ReviewStatus()
	reviewStyle := styles.SubtitleStyle
	switch reviewStatus {
	case models.ReviewApproved:
		reviewStyle = styles.CleanStyle
	case models.ReviewChangesRequested:
		reviewStyle = styles.ErrorStyle
	}
	s.writeInfoLine(b, "Review:", reviewStyle.Render(reviewStatus))

	if len(pr.ApprovedBy) > 0 {
		approvers := strings.Join(pr.ApprovedBy, ", ")
		s.writeInfoLine(b, "Approved by:", truncate(approvers, descriptionTruncLen))
	}

	if pr.Checks.Total > 0 {
		checkStatus := pr.Checks.Summary()
		checkStyle := styles.SubtitleStyle
		switch checkStatus {
		case "passing":
			checkStyle = styles.CleanStyle
		case models.StatusFailing:
			checkStyle = styles.ErrorStyle
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
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(commitEmptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)
		b.WriteString(emptyStyle.Render("No commits found"))

		return
	}

	maxCommits := min(branchDetailMaxCommits, len(m.branchDetail.Commits))
	for i := range maxCommits {
		commit := m.branchDetail.Commits[i]
		line := fmt.Sprintf("  %s  %-50s  %s  %s\n",
			styles.SubtitleStyle.Render(commit.ShortHash),
			truncate(commit.Subject, commitSubjectLen),
			styles.SubtitleStyle.Render(truncate(commit.Author, commitAuthorLen)),
			styles.SubtitleStyle.Render(commit.RelativeDate()),
		)
		b.WriteString(line)
	}
}

func (m Model) writeBranchActionsSection(b *strings.Builder, s branchDetailStyles) {
	b.WriteString("\n")
	b.WriteString(s.section.Render("Actions"))
	b.WriteString("\n\n")

	actionStyle := lipgloss.NewStyle().Foreground(styles.Blue).PaddingLeft(infoPaddingLeft)

	actions := []string{
		styles.FooterKeyStyle.Render("y") + actionStyle.Render(" copy branch name"),
	}

	if m.branchDetail.PRInfo != nil {
		actions = append(actions,
			styles.FooterKeyStyle.Render("p")+actionStyle.Render(" open PR in browser"),
			styles.FooterKeyStyle.Render("o")+actionStyle.Render(" open PR URL"))
	} else {
		actions = append(actions,
			styles.FooterKeyStyle.Render("p")+actionStyle.Render(" create new PR"))
	}

	b.WriteString(strings.Join(actions, "  "))
	b.WriteString("\n")
}

func (m Model) renderBranchDetail() string {
	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n\n")

	s := newBranchDetailStyles()
	m.writeBranchInfoSection(&b, s)
	m.writeBranchPRSection(&b, s)
	m.writeBranchCommitsSection(&b, s)
	m.writeBranchActionsSection(&b, s)

	contentLines := strings.Count(b.String(), "\n")
	footerHeight := 1
	paddingNeeded := m.height - contentLines - footerHeight - 1
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(styles.FooterStyle.Render("esc: back  ?: help"))

	return b.String()
}
