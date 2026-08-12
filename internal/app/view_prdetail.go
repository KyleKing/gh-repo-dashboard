package app

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/markdown"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
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

// writePRDetailDescription writes the "Description" section as rendered
// markdown, or nothing if the PR has no body.
func writePRDetailDescription(b *strings.Builder, sectionStyle lipgloss.Style, body string, width int) {
	if strings.TrimSpace(body) == "" {
		return
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Description"))
	b.WriteString("\n")

	writeMarkdown(b, body, width, prBodyMaxLines)
}

// writeMarkdown writes a rendered markdown body indented under its section
// heading.
func writeMarkdown(b *strings.Builder, body string, width, maxLines int) {
	padding := lipgloss.NewStyle().PaddingLeft(infoPaddingLeft)
	for _, line := range markdown.Render(body, max(width-infoPaddingLeft, 1), maxLines) {
		b.WriteString(padding.Render(line))
		b.WriteString("\n")
	}
}

// checkStateFailure is one of the check conclusions checkStatusStyle treats
// as failing.
const checkStateFailure = "failure"

// checkStatusStyle colors a single check's status by outcome. Only a failing
// or skipped check draws the eye; passing, pending, and every other outcome
// read as settled, so a long checks list does not read as a wall of color.
func checkStatusStyle(status string) lipgloss.Style {
	switch status {
	//nolint:misspell // GitHub's own conclusion value is spelled "cancelled"
	case checkStateFailure, "error", "cancelled", "timed_out":
		return styles.ErrorStyle
	case "skipped":
		return styles.WarningStyle
	default:
		return styles.SubtitleStyle
	}
}

// writePRDetailChecks writes the "Checks" section: one row per CI check with
// its status and how long it ran. Nothing is written when the pull request has
// no checks.
func writePRDetailChecks(
	b *strings.Builder, sectionStyle lipgloss.Style, checks []models.CheckDetail, layout table.Layout,
) {
	if len(checks) == 0 {
		return
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Checks"))
	b.WriteString("\n")

	rowPadding := lipgloss.NewStyle().PaddingLeft(infoPaddingLeft)
	for _, check := range checks {
		status := check.StatusDisplay()
		values := map[string]string{
			colCheckName:     checkDisplayName(check),
			colCheckState:    status,
			colCheckDuration: check.Duration(),
		}
		cellStyles := map[string]lipgloss.Style{
			colCheckState:    checkStatusStyle(status),
			colCheckDuration: styles.SubtitleStyle,
		}

		cells := renderCells(layout, values, cellStyles, &plainStyle)
		b.WriteString(rowPadding.Render(table.Join(cells)))
		b.WriteString("\n")
	}
}

func checkDisplayName(check models.CheckDetail) string {
	name := check.Name
	if name == "" {
		name = check.Workflow
	}
	if name == "" {
		return "(unnamed check)"
	}

	if check.Workflow != "" && check.Workflow != name {
		return check.Workflow + " / " + name
	}

	return name
}

// writePRDetailLatestComment writes the most recent comment on the pull
// request, or nothing when there are none.
func writePRDetailLatestComment(
	b *strings.Builder, sectionStyle, valueStyle lipgloss.Style, comment *models.PRComment, width int,
) {
	if comment == nil {
		return
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Latest comment"))
	b.WriteString("\n")
	b.WriteString(valueStyle.Render(
		styles.BranchStyle.Render(comment.Author) + " " +
			styles.SubtitleStyle.Render(comment.RelativeCreated())))
	b.WriteString("\n")
	writeMarkdown(b, comment.Body, width, prCommentMaxLines)
}

// prDetailFooter names every key the PR detail view supports, collapsing by
// the shared hint rules so the actions stay visible in the one place that
// never scrolls out of view, rather than in a section the page's own length
// can push off-screen.
//
//nolint:mnd // the numbers are this footer's collapse order, not constants used elsewhere
func prDetailFooter(width int) string {
	hints := []footerHint{
		{key: keyEsc, desc: descBack, priority: 9},
		{key: "?", desc: "help", priority: 8},
		{key: keyNavPair, desc: "scroll", priority: 7},
		{key: "[/]", desc: "prev/next PR", priority: 6},
		{key: "o", desc: "browser", priority: 5},
		{key: "M", desc: "squash-merge", priority: 4},
		{key: "u", desc: descCopyURL, priority: 3},
		{key: "n", desc: "copy number", priority: 2},
		{key: "b", desc: "copy branch", priority: 1},
	}

	parts := make([]string, 0, len(hints))
	for _, h := range fittingHints(hints, width) {
		parts = append(parts, styles.FooterKeyStyle.Render(h.key)+styles.FooterDescStyle.Render(" "+h.desc))
	}

	return strings.Join(parts, "  ")
}

// renderPRDetailLoading renders the placeholder shown before any PR detail
// has arrived (shouldn't normally be seen, since PR info loads progressively).
func (m Model) renderPRDetailLoading(home, sep, repo string) string {
	var b strings.Builder

	b.WriteString(home + sep + repo + sep + styles.SubtitleStyle.Render("PR Detail"))
	b.WriteString("\n\n")
	b.WriteString(m.loadingPlaceholder("Loading PR details"))
	b.WriteString("\n\n")

	footer := styles.FooterKeyStyle.Render(keyEsc) + styles.FooterDescStyle.Render(" back  ") +
		styles.FooterKeyStyle.Render("?") + styles.FooterDescStyle.Render(" help")
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

// prDetailContentLines builds the PR detail page's body as lines, before
// windowing to the scroll offset: the breadcrumb and title, the Pull Request
// info, and the Checks, Description, and Latest comment sections.
func (m Model) prDetailContentLines(home, sep, repo string) []string {
	var b strings.Builder

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

	writePRDetailChecks(&b, sectionStyle, m.prDetail.CheckDetails, fitDetailCols(checkColSpecs, m.width))
	writePRDetailDescription(&b, sectionStyle, m.prDetail.Body, m.width)
	writePRDetailLatestComment(&b, sectionStyle, valueStyle, m.prDetail.LatestComment, m.width)

	return strings.Split(b.String(), "\n")
}

// prDetailScrollMarker reports how much of the page is on screen, so content
// running past the bottom is never mistaken for the whole of it.
func prDetailScrollMarker(scroll, total, visible int) string {
	if total <= visible {
		return ""
	}

	shown := min(scroll+visible, total)

	return styles.SubtitleStyle.Render("  " + strconv.Itoa(shown) + "/" + strconv.Itoa(total))
}

// prDetailChromeHeight is how many lines outside the scrollable body are
// spent: the footer's own line, plus one line of headroom the page has always
// left at the bottom of the terminal.
const prDetailChromeHeight = 2

func (m Model) renderPRDetail() string {
	summary := m.summaries[m.selectedRepo]
	home := styles.SubtitleStyle.Render("Repos")
	sep := styles.SubtitleStyle.Render(" > ")
	repo := styles.BranchStyle.Render(summary.Name())

	// Check if PR detail has been loaded
	if m.prDetail.Number == 0 {
		return m.renderPRDetailLoading(home, sep, repo)
	}

	lines := m.prDetailContentLines(home, sep, repo)

	visible := max(m.height-prDetailChromeHeight, 1)
	start := min(m.prDetailScroll, max(len(lines)-visible, 0))
	shown := padBottom(lines[start:min(start+visible, len(lines))], visible)

	footer := prDetailFooter(contentWidth(m.width)) + prDetailScrollMarker(start, len(lines), visible)

	return strings.Join(shown, "\n") + "\n" + footer
}
