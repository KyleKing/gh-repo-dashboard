package app

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

// writePRDetailInfo writes the "Pull Request" section's info lines (title,
// author, assignees, reviewers, branch, state, review, and - once fully
// loaded - change stats) via the given writeLine(label, value) callback.
func (m Model) writePRDetailInfo(writeLine func(label, value string)) {
	writeLine("Title:", m.prDetail.Title)

	// Author might not be loaded yet (progressive loading)
	if m.prDetail.Author != "" {
		writeLine("Author:", m.prDetail.Author)
	}

	if len(m.prDetail.Assignees) > 0 {
		writeLine("Assignees:", strings.Join(m.prDetail.Assignees, ", "))
	}

	if len(m.prDetail.Reviewers) > 0 {
		writeLine("Reviewers:", strings.Join(m.prDetail.Reviewers, ", "))
	}

	writeLine("Branch:",
		styles.BranchStyle.Render(m.prDetail.HeadRef)+" → "+styles.BranchStyle.Render(m.prDetail.BaseRef))

	stateStyle := styles.PROpenStyle
	switch {
	case m.prDetail.IsDraft:
		stateStyle = styles.PRDraftStyle
	case m.prDetail.State == models.PRStatusMerged:
		stateStyle = styles.PRMergedStyle
	case m.prDetail.State == models.PRStatusClosed:
		stateStyle = styles.ErrorStyle
	}
	writeLine("State:", stateStyle.Render(m.prDetail.StatusDisplay()))

	reviewStyle := styles.SubtitleStyle
	reviewStatus := m.prDetail.ReviewStatus()
	switch reviewStatus {
	case models.ReviewApproved:
		reviewStyle = styles.CleanStyle
	case models.ReviewChangesRequested:
		reviewStyle = styles.ErrorStyle
	}
	writeLine("Review:", reviewStyle.Render(reviewStatus))

	// Only show detailed stats if fully loaded
	if m.prDetail.Author == "" {
		return
	}

	writeLine("Changes:",
		styles.CleanStyle.Render(fmt.Sprintf("+%d", m.prDetail.Additions))+" "+
			styles.ErrorStyle.Render(fmt.Sprintf("-%d", m.prDetail.Deletions)))

	if m.prDetail.Comments > 0 {
		writeLine("Comments:", strconv.Itoa(m.prDetail.Comments))
	}

	if !m.prDetail.CreatedAt.IsZero() {
		writeLine("Created:", m.prDetail.RelativeCreated())
	}

	if !m.prDetail.UpdatedAt.IsZero() {
		writeLine("Updated:", m.prDetail.RelativeUpdated())
	}
}

// writePRDetailDescription writes the "Description" section (truncated to
// prBodyMaxLen), or nothing if the PR has no body.
func writePRDetailDescription(b *strings.Builder, sectionStyle, valueStyle lipgloss.Style, body string) {
	if body == "" {
		return
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Description"))
	b.WriteString("\n")

	desc := body
	if len(desc) > prBodyMaxLen {
		desc = desc[:prBodyMaxLen] + "..."
	}
	b.WriteString(valueStyle.Render(desc))
	b.WriteString("\n")
}

// writePRDetailActions writes the "Actions" section's footer key hints.
func writePRDetailActions(b *strings.Builder, sectionStyle lipgloss.Style) {
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Actions"))
	b.WriteString("\n")

	actionPadding := lipgloss.NewStyle().PaddingLeft(infoPaddingLeft)
	actions := []string{
		styles.FooterKeyStyle.Render("o") + styles.FooterDescStyle.Render(" open in browser"),
		styles.FooterKeyStyle.Render("u") + styles.FooterDescStyle.Render(" copy URL"),
		styles.FooterKeyStyle.Render("n") + styles.FooterDescStyle.Render(" copy PR number"),
		styles.FooterKeyStyle.Render("b") + styles.FooterDescStyle.Render(" copy branch name"),
	}
	b.WriteString(actionPadding.Render(strings.Join(actions, "    ")))
	b.WriteString("\n")
}

// renderPRDetailLoading renders the placeholder shown before any PR detail
// has arrived (shouldn't normally be seen, since PR info loads progressively).
func renderPRDetailLoading(home, sep, repo string) string {
	var b strings.Builder

	b.WriteString(home + sep + repo + sep + styles.SubtitleStyle.Render("PR Detail"))
	b.WriteString("\n\n")
	loadingStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		Padding(loadingStatePad)
	b.WriteString(loadingStyle.Render("Loading PR details..."))
	b.WriteString("\n\n")

	footer := styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" back  ") +
		styles.FooterKeyStyle.Render("?") + styles.FooterDescStyle.Render(" help")
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

func (m Model) renderPRDetail() string {
	var b strings.Builder

	summary := m.summaries[m.selectedRepo]
	home := styles.SubtitleStyle.Render("Repos")
	sep := styles.SubtitleStyle.Render(" > ")
	repo := styles.BranchStyle.Render(summary.Name())

	// Check if PR detail has been loaded
	if m.prDetail.Number == 0 {
		return renderPRDetailLoading(home, sep, repo)
	}

	prTitle := styles.TitleStyle.Render(fmt.Sprintf("PR #%d", m.prDetail.Number))
	b.WriteString(home + sep + repo + sep + prTitle)
	b.WriteString("\n\n")

	// Show loading indicator for additional details if not yet loaded
	if m.prDetail.Author == "" {
		loadingIndicator := lipgloss.NewStyle().
			Foreground(styles.Subtext0).
			Italic(true).
			Render(" (loading details...)")
		b.WriteString(loadingIndicator)
		b.WriteString("\n")
	}

	sectionStyle := lipgloss.NewStyle().
		Foreground(styles.Blue).
		Bold(true).
		PaddingLeft(1).
		PaddingTop(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(styles.Subtext0).
		Width(detailLabelWidthPR)

	valueStyle := lipgloss.NewStyle().
		Foreground(styles.Text).
		PaddingLeft(infoPaddingLeft)

	writeLine := func(label, value string) {
		b.WriteString(valueStyle.Render(labelStyle.Render(label) + " " + value))
		b.WriteString("\n")
	}

	b.WriteString(sectionStyle.Render("Pull Request"))
	b.WriteString("\n")

	m.writePRDetailInfo(writeLine)

	writePRDetailDescription(&b, sectionStyle, valueStyle, m.prDetail.Body)
	writePRDetailActions(&b, sectionStyle)

	contentLines := strings.Count(b.String(), "\n")
	statusLines := 0
	if m.statusMessage != "" {
		statusLines = 1
	}
	paddingNeeded := m.height - contentLines - statusLines - statusBarHeight
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	}

	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(styles.Green).
			Background(styles.Surface0).
			Padding(0, 1)
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	footer := styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" back  ") +
		styles.FooterKeyStyle.Render("?") + styles.FooterDescStyle.Render(" help")
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}
