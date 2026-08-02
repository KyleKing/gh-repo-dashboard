package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

func (m Model) renderRepoDetailBreadcrumbs() string {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return styles.TitleStyle.Render("repo-dashboard")
	}

	home := styles.SubtitleStyle.Render("Repos")
	sep := styles.SubtitleStyle.Render(" > ")
	repo := styles.TitleStyle.Render(summary.Name())

	var badges []string
	badges = append(badges, styles.Badge(summary.VCSType.String(), styles.CountBadgeStyle))
	if summary.IsDirty() {
		badges = append(badges, styles.Badge("dirty", styles.FilterBadgeStyle))
	}
	if summary.PRInfo != nil {
		badges = append(badges, styles.Badge(fmt.Sprintf("PR #%d", summary.PRInfo.Number), styles.PROpenStyle))
	}
	if checkouts := m.RepoCheckouts(); len(checkouts) > 0 {
		label := fmt.Sprintf("⧉ %d parallel checkouts", len(checkouts))
		if len(checkouts) == 1 {
			label = "⧉ " + checkouts[0].Folder()
		}
		badges = append(badges, styles.Badge(label, styles.WarningStyle))
	}
	if summary.RemoteProtocol != "" {
		badges = append(badges, styles.Badge(summary.RemoteProtocol, styles.CountBadgeStyle))
	}
	for _, override := range summary.ConfigOverrides {
		text := fmt.Sprintf("%s: %s≠%s", override.Key, override.LocalValue, override.GlobalValue)
		badges = append(badges, styles.Badge(text, styles.WarningStyle))
	}

	return home + sep + repo + "  " + strings.Join(badges, " ")
}

func (m Model) renderRepoDetail() string {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return "Repository not found"
	}

	var b strings.Builder

	b.WriteString(m.renderBreadcrumbs())
	b.WriteString("\n")
	b.WriteString(styles.SubtitleStyle.Render(summary.Path))
	b.WriteString("\n\n")

	b.WriteString(m.renderDetailTabs())
	b.WriteString("\n\n")

	switch m.detailTab {
	case DetailTabBranches:
		b.WriteString(m.renderBranchList())
	case DetailTabStashes:
		b.WriteString(m.renderStashList())
	case DetailTabWorktrees:
		b.WriteString(m.renderWorktreeList())
	case DetailTabPRs:
		b.WriteString(m.renderPRList())
	case DetailTabNotes:
		b.WriteString(m.renderNotesTab())
	}

	footer := "tab: switch tabs  j/k: navigate  esc: back"
	switch m.detailTab {
	case DetailTabBranches:
		footer = "tab: tabs  j/k: nav  enter: branch  c: switch  p: push  N: new PR  M: merge  esc: back"
	case DetailTabPRs:
		footer = "tab: tabs  j/k: nav  enter: PR  M: squash-merge  esc: back"
	default:
		// stashes/worktrees tabs use the generic footer above
	}

	contentLines := strings.Count(b.String(), "\n")
	footerHeight := 1
	paddingNeeded := m.height - contentLines - footerHeight - 1
	if paddingNeeded > 0 {
		b.WriteString(strings.Repeat("\n", paddingNeeded))
	} else {
		b.WriteString("\n")
	}
	b.WriteString(styles.FooterStyle.Render(footer))

	return b.String()
}

func (m Model) renderDetailTabs() string {
	summary := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	worktreeLabel := "Worktrees"
	if isJJ {
		worktreeLabel = "Workspaces"
	}

	notesCount := len(m.notesFiles)

	tabs := []struct {
		name  string
		tab   DetailTab
		count int
	}{
		{"Branches", DetailTabBranches, len(m.branches)},
		{"Stashes", DetailTabStashes, len(m.stashes)},
		{worktreeLabel, DetailTabWorktrees, len(m.worktrees)},
		{"PRs", DetailTabPRs, len(m.prs)},
		{"Notes", DetailTabNotes, notesCount},
	}

	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("%s (%d)", t.name, t.count)
		if t.tab == m.detailTab {
			parts = append(parts, styles.TabActiveStyle.Render(label))
		} else {
			parts = append(parts, styles.TabInactiveStyle.Render(label))
		}
	}

	tabRow := strings.Join(parts, styles.TabSeparatorStyle.Render(" │ "))

	ruleWidth := lipgloss.Width(tabRow)
	rule := styles.SubtitleStyle.Render(strings.Repeat("─", ruleWidth))

	return tabRow + "\n" + rule
}

func (m Model) renderBranchList() string {
	if len(m.branches) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		return emptyStyle.Render("No branches found")
	}

	prsByBranch := prsByHeadRef(m.prs)
	checkouts := m.RepoCheckouts()

	var rows []string
	header := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %-*s  %-*s  %s",
		branchNameTruncLen, "BRANCH",
		upstreamTruncLen, "UPSTREAM",
		branchStatusColWidth, "STATUS",
		branchPRColWidth, "PR",
		branchChecksColWidth, "CHECKS",
		checkoutColWidth, "CHECKED OUT",
		"LAST COMMIT")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, branch := range m.branches {
		row := branchRow{
			branch:    branch,
			pr:        prsByBranch[branch.Name],
			deletable: m.deletableBranches[branch.Name],
			selected:  i == m.detailCursor,
		}
		if checkout, ok := models.CheckoutForBranch(checkouts, branch.Name); ok {
			row.checkout = checkout.Folder()
		}
		rows = append(rows, renderBranchRow(row))
	}

	return strings.Join(rows, "\n")
}

// prsByHeadRef indexes open pull requests by their head branch name.
func prsByHeadRef(prs []models.PRInfo) map[string]*models.PRInfo {
	byRef := make(map[string]*models.PRInfo, len(prs))
	for i := range prs {
		byRef[prs[i].HeadRef] = &prs[i]
	}

	return byRef
}

// branchAheadBehindStatus renders a branch's ahead/behind indicator, or a
// checkmark if it's fully in sync with its upstream.
func branchAheadBehindStatus(branch models.BranchInfo) string {
	status := ""
	if branch.Ahead > 0 {
		status += fmt.Sprintf("↑%d", branch.Ahead)
	}
	if branch.Behind > 0 {
		if status != "" {
			status += " "
		}
		status += fmt.Sprintf("↓%d", branch.Behind)
	}
	if status == "" {
		status = "✓"
	}

	return status
}

// branchRow is one rendered row of the branch list: the branch itself plus the
// pull request, parallel checkout, and cleanup state discovered for it.
type branchRow struct {
	branch    models.BranchInfo
	pr        *models.PRInfo
	checkout  string
	deletable bool
	selected  bool
}

// formatBranchPRCell renders "#N" plus a draft/review marker, or emDash when
// the branch has no open pull request.
func formatBranchPRCell(pr *models.PRInfo) string {
	if pr == nil {
		return emDash
	}

	cell := fmt.Sprintf("#%d", pr.Number)
	switch {
	case pr.IsDraft:
		cell += " draft"
	case pr.ReviewStatus() == models.ReviewApproved:
		cell += " ✓"
	case pr.ReviewStatus() == models.ReviewChangesRequested:
		cell += " ✗"
	}

	return cell
}

// formatChecksCell renders a pull request's CI rollup as "passing 3/3", or
// emDash when there is no pull request or it has no checks.
func formatChecksCell(pr *models.PRInfo) string {
	if pr == nil || pr.Checks.Total == 0 {
		return emDash
	}

	return fmt.Sprintf("%s %d/%d", pr.Checks.Summary(), pr.Checks.Passing, pr.Checks.Total)
}

// checksCellStyle colors a checks cell by its rollup outcome.
func checksCellStyle(pr *models.PRInfo, base lipgloss.Style) lipgloss.Style {
	if pr == nil || pr.Checks.Total == 0 {
		return base
	}

	switch pr.Checks.Summary() {
	case models.StatusPassing:
		return styles.CleanStyle
	case models.StatusFailing:
		return styles.ErrorStyle
	default:
		return styles.WarningStyle
	}
}

func renderBranchRow(row branchRow) string {
	cursor := "  "
	if row.selected {
		cursor = "> "
	}

	name := truncate(row.branch.Name, branchNameTruncLen)
	if row.branch.IsCurrent {
		name = "* " + name
	}

	checkout := emDash
	if row.branch.IsCurrent {
		checkout = "here"
	} else if row.checkout != "" {
		checkout = "⧉ " + truncate(row.checkout, checkoutColWidth-2) //nolint:mnd // reserves room for the "⧉ " prefix
	}

	style := styles.TableRowStyle
	if row.selected {
		style = styles.SelectedRowStyle
	}

	nameStyle := styles.BranchStyle
	if row.branch.IsCurrent {
		nameStyle = styles.PROpenStyle
	}
	nameStyle = withSelection(nameStyle, row.selected)

	prStyle := style
	if row.pr != nil {
		prStyle = withSelection(styles.PROpenStyle, row.selected)
	}

	checkoutStyle := style
	if row.checkout != "" && !row.branch.IsCurrent {
		checkoutStyle = withSelection(styles.WarningStyle, row.selected)
	}

	cells := fmt.Sprintf("%-*s", branchNameTruncLen, name)
	rendered := fmt.Sprintf("%s%s  %s  %s  %s  %s  %s  %s",
		cursor,
		nameStyle.Render(cells),
		style.Render(fmt.Sprintf("%-*s", upstreamTruncLen, truncate(row.branch.Upstream, upstreamTruncLen))),
		style.Render(fmt.Sprintf("%-*s", branchStatusColWidth, branchAheadBehindStatus(row.branch))),
		prStyle.Render(fmt.Sprintf("%-*s", branchPRColWidth, formatBranchPRCell(row.pr))),
		checksCellStyle(row.pr, style).Render(fmt.Sprintf("%-*s", branchChecksColWidth, formatChecksCell(row.pr))),
		checkoutStyle.Render(fmt.Sprintf("%-*s", checkoutColWidth, checkout)),
		style.Render(row.branch.RelativeLastCommit()),
	)
	if row.deletable {
		rendered += "  " + styles.Badge("merged", styles.PROpenStyle)
	}

	return rendered
}

func (m Model) renderStashList() string {
	if len(m.stashes) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		noStashesMsg := "No stashes found\n\nStashes are only available for git repositories.\n" +
			"JJ repositories use the working copy change instead."

		return emptyStyle.Render(noStashesMsg)
	}

	var rows []string
	header := fmt.Sprintf("  %-8s  %-40s  %s",
		"INDEX", "MESSAGE", "DATE")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, stash := range m.stashes {
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		index := fmt.Sprintf("stash@{%d}", stash.Index)
		message := truncate(stash.Message, messageTruncLen)
		date := stash.RelativeDate()

		var style lipgloss.Style
		if i == m.detailCursor {
			style = styles.SelectedRowStyle
		} else {
			style = styles.TableRowStyle
		}

		formattedIndex := fmt.Sprintf("%-8s", index)
		formattedMessage := fmt.Sprintf("%-40s", message)

		row := fmt.Sprintf("%s%s  %s  %s",
			cursor,
			style.Render(formattedIndex),
			style.Render(formattedMessage),
			style.Render(date),
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// renderNotesTab shows the full content of each of the repo's detected notes
// files, each clearly delineated by its filename, or an empty state naming
// the filenames that are detected.
func (m Model) renderNotesTab() string {
	if len(m.notesFiles) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		noNotesMsg := "No notes file found\n\n" +
			"Add a .doing, doing.md, doing.txt, or TODO.md file to the repo root."

		return emptyStyle.Render(noNotesMsg)
	}

	var b strings.Builder

	for i, nf := range m.notesFiles {
		if i > 0 {
			b.WriteString("\n\n")
			b.WriteString(styles.SubtitleStyle.Render(strings.Repeat("─", notesSeparatorWidth)))
			b.WriteString("\n\n")
		}

		b.WriteString(styles.HeaderStyle.Render(nf.Name))
		b.WriteString("\n\n")

		content := nf.Content
		if content == "" {
			content = "(empty file)"
		}
		b.WriteString(styles.TableRowStyle.Render(content))
	}

	return b.String()
}

func (m Model) renderWorktreeList() string {
	summary := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	if len(m.worktrees) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		emptyMsg := "No worktrees found\n\nWorktrees allow working on multiple branches simultaneously."
		if isJJ {
			emptyMsg = "No workspaces found\n\nWorkspaces (jj's version of worktrees) allow working on multiple\n" +
				"changes simultaneously in separate working directories."
		}

		return emptyStyle.Render(emptyMsg)
	}

	var rows []string
	header := fmt.Sprintf("  %-30s  %-20s  %s",
		"PATH", "BRANCH", "STATUS")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i, wt := range m.worktrees {
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		path := truncate(filepath.Base(wt.Path), worktreePathTruncLen)
		branch := truncate(wt.Branch, branchNameTruncLen)
		status := ""
		if wt.IsBare {
			status = "bare"
		}
		if wt.IsLocked {
			if status != "" {
				status += ", "
			}
			status += "locked"
		}
		if status == "" {
			status = "active"
		}

		var style lipgloss.Style
		if i == m.detailCursor {
			style = styles.SelectedRowStyle
		} else {
			style = styles.TableRowStyle
		}

		formattedPath := fmt.Sprintf("%-30s", path)
		formattedBranch := fmt.Sprintf("%-20s", branch)

		branchStyleLocal := withSelection(styles.BranchStyle, i == m.detailCursor)

		row := fmt.Sprintf("%s%s  %s  %s",
			cursor,
			style.Render(formattedPath),
			branchStyleLocal.Render(formattedBranch),
			style.Render(status),
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderPRList() string {
	if len(m.prs) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.Surface1).
			Padding(emptyStateVPad, emptyStateHPad).
			Foreground(styles.Subtext0)

		return emptyStyle.Render("No open pull requests")
	}

	var rows []string
	header := fmt.Sprintf("  %-8s  %-40s  %-10s  %-18s  %s",
		"NUMBER", "TITLE", "STATE", "REVIEW", "BRANCH")
	rows = append(rows, styles.HeaderStyle.Render(header))

	for i := range m.prs {
		pr := &m.prs[i]
		cursor := "  "
		if i == m.detailCursor {
			cursor = "> "
		}

		number := fmt.Sprintf("#%d", pr.Number)
		title := truncate(pr.Title, messageTruncLen)
		state := pr.StatusDisplay()
		review := pr.ReviewStatus()
		branch := truncate(pr.HeadRef, branchNameTruncLen)

		var rowStyle lipgloss.Style
		if i == m.detailCursor {
			rowStyle = styles.SelectedRowStyle
		} else {
			rowStyle = styles.TableRowStyle
		}

		stateStyle := styles.PROpenStyle
		switch {
		case pr.IsDraft:
			stateStyle = styles.PRDraftStyle
		case state == models.PRStatusMerged:
			stateStyle = styles.PRMergedStyle
		case state == models.PRStatusClosed:
			stateStyle = styles.ErrorStyle
		}
		stateStyle = withSelection(stateStyle, i == m.detailCursor)

		reviewStyle := styles.SubtitleStyle
		switch review {
		case models.ReviewApproved:
			reviewStyle = styles.CleanStyle
		case models.ReviewChangesRequested:
			reviewStyle = styles.ErrorStyle
		}
		reviewStyle = withSelection(reviewStyle, i == m.detailCursor)

		branchStyleLocal := withSelection(styles.BranchStyle, i == m.detailCursor)

		row := fmt.Sprintf("%s%-8s  %-40s  %s  %-18s  %s",
			cursor,
			rowStyle.Render(number),
			rowStyle.Render(title),
			stateStyle.Render(fmt.Sprintf("%-10s", state)),
			reviewStyle.Render(review),
			branchStyleLocal.Render(branch),
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}
