package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
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
	b.WriteString(styles.SubtitleStyle.Render(truncate(summary.Path, contentWidth(m.width))))
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
		if m.detailLoading {
			return m.loadingPlaceholder("Loading branches")
		}

		return emptyPlaceholder("No branches found", "")
	}

	prsByBranch := prsByHeadRef(m.prs)
	checkouts := m.RepoCheckouts()

	layout := fitDetailCols(branchColSpecs, m.width)
	rows := []string{detailHeader(layout)}

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
		rows = append(rows, renderBranchRow(row, layout))
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

// Row markers whose display width padCell must account for; padCell truncates
// the prefixed value, so the prefix never widens its column.
const (
	currentBranchPrefix = "* "
	peerPrefix          = "⧉ "
)

func renderBranchRow(row branchRow, layout table.Layout) string {
	name := row.branch.Name
	if row.branch.IsCurrent {
		name = currentBranchPrefix + name
	}

	checkout := emDash
	if row.branch.IsCurrent {
		checkout = "here"
	} else if row.checkout != "" {
		checkout = peerPrefix + row.checkout
	}

	style := rowStyleFor(row.selected)

	nameStyle := styles.BranchStyle
	if row.branch.IsCurrent {
		nameStyle = styles.PROpenStyle
	}

	prStyle := style
	if row.pr != nil {
		prStyle = styles.PROpenStyle
	}

	checkoutStyle := style
	if row.checkout != "" && !row.branch.IsCurrent {
		checkoutStyle = styles.WarningStyle
	}

	values := map[string]string{
		colBranchName:  name,
		colUpstream:    row.branch.Upstream,
		colBranchState: branchAheadBehindStatus(row.branch),
		colBranchPR:    formatBranchPRCell(row.pr),
		colChecks:      formatChecksCell(row.pr),
		colCheckedOut:  checkout,
		colLastCommit:  row.branch.RelativeLastCommit(),
	}
	cellStyles := map[string]lipgloss.Style{
		colBranchName: withSelection(nameStyle, row.selected),
		colBranchPR:   withSelection(prStyle, row.selected),
		colChecks:     withSelection(checksCellStyle(row.pr, style), row.selected),
		colCheckedOut: withSelection(checkoutStyle, row.selected),
	}

	cells := renderCells(layout, values, cellStyles, &style)

	rendered := detailRow(rowCursorFor(row.selected), layout, cells)
	if row.deletable {
		rendered += "  " + styles.Badge("merged", styles.PROpenStyle)
	}

	return rendered
}

func (m Model) renderStashList() string {
	if len(m.stashes) == 0 {
		if m.detailLoading {
			return m.loadingPlaceholder("Loading stashes")
		}

		return emptyPlaceholder("No stashes found",
			"Stashes are only available for git repositories.\n"+
				"JJ repositories use the working copy change instead.")
	}

	layout := fitDetailCols(stashColSpecs, m.width)
	rows := []string{detailHeader(layout)}

	for i, stash := range m.stashes {
		selected := i == m.detailCursor
		style := rowStyleFor(selected)

		values := map[string]string{
			colStashIndex:   fmt.Sprintf("stash@{%d}", stash.Index),
			colStashMessage: stash.Message,
			colStashDate:    stash.RelativeDate(),
		}

		cells := renderCells(layout, values, nil, &style)
		rows = append(rows, detailRow(rowCursorFor(selected), layout, cells))
	}

	return strings.Join(rows, "\n")
}

// renderNotesTab shows the full content of each of the repo's detected notes
// files, each clearly delineated by its filename, or an empty state naming
// the filenames that are detected.
func (m Model) renderNotesTab() string {
	if len(m.notesFiles) == 0 {
		if m.detailLoading {
			return m.loadingPlaceholder("Loading notes")
		}

		return emptyPlaceholder("No notes file found",
			"Add a .doing, doing.md, doing.txt, or TODO.md file to the repo root.")
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

// renderWorktreesPlaceholder renders the loading or empty state for the
// worktrees tab, using jj's "workspace" wording when the repo is a jj repo.
func (m Model) renderWorktreesPlaceholder(isJJ bool) string {
	if isJJ {
		if m.detailLoading {
			return m.loadingPlaceholder("Loading workspaces")
		}

		return emptyPlaceholder("No workspaces found",
			"Workspaces (jj's version of worktrees) allow working on multiple\n"+
				"changes simultaneously in separate working directories.")
	}

	if m.detailLoading {
		return m.loadingPlaceholder("Loading worktrees")
	}

	return emptyPlaceholder("No worktrees found",
		"Worktrees allow working on multiple branches simultaneously.")
}

func (m Model) renderWorktreeList() string {
	summary := m.summaries[m.selectedRepo]
	isJJ := summary.VCSType == models.VCSTypeJJ

	if len(m.worktrees) == 0 {
		return m.renderWorktreesPlaceholder(isJJ)
	}

	layout := fitDetailCols(worktreeColSpecs, m.width)
	rows := []string{detailHeader(layout)}

	for i, wt := range m.worktrees {
		selected := i == m.detailCursor
		style := rowStyleFor(selected)
		branchStyleLocal := withSelection(styles.BranchStyle, selected)

		values := map[string]string{
			colWorktreePath:   filepath.Base(wt.Path),
			colWorktreeBranch: wt.Branch,
			colWorktreeState:  worktreeState(wt),
		}

		cells := renderCells(layout, values, map[string]lipgloss.Style{colWorktreeBranch: branchStyleLocal}, &style)
		rows = append(rows, detailRow(rowCursorFor(selected), layout, cells))
	}

	return strings.Join(rows, "\n")
}

// worktreeState summarizes a worktree's bare and locked flags.
func worktreeState(wt models.WorktreeInfo) string {
	var flags []string
	if wt.IsBare {
		flags = append(flags, "bare")
	}
	if wt.IsLocked {
		flags = append(flags, "locked")
	}

	if len(flags) == 0 {
		return "active"
	}

	return strings.Join(flags, ", ")
}

func (m Model) renderPRList() string {
	if len(m.prs) == 0 {
		if m.detailLoading {
			return m.loadingPlaceholder("Loading pull requests")
		}

		return emptyPlaceholder("No open pull requests", "")
	}

	layout := fitDetailCols(prColSpecs, m.width)
	rows := []string{detailHeader(layout)}

	for i := range m.prs {
		pr := &m.prs[i]
		selected := i == m.detailCursor
		rowStyle := rowStyleFor(selected)

		values := map[string]string{
			colPRNumber: fmt.Sprintf("#%d", pr.Number),
			colPRTitle:  pr.Title,
			colPRState:  pr.StatusDisplay(),
			colPRReview: pr.ReviewStatus(),
			colPRBranch: pr.HeadRef,
		}
		cellStyles := map[string]lipgloss.Style{
			colPRState:  withSelection(prStateStyle(pr), selected),
			colPRReview: withSelection(prReviewStyle(pr.ReviewStatus()), selected),
			colPRBranch: withSelection(styles.BranchStyle, selected),
		}

		cells := renderCells(layout, values, cellStyles, &rowStyle)
		rows = append(rows, detailRow(rowCursorFor(selected), layout, cells))
	}

	return strings.Join(rows, "\n")
}

// prStateStyle colors a pull request's state cell by draft, merged, or closed.
func prStateStyle(pr *models.PRInfo) lipgloss.Style {
	switch {
	case pr.IsDraft:
		return styles.PRDraftStyle
	case pr.StatusDisplay() == models.PRStatusMerged:
		return styles.PRMergedStyle
	case pr.StatusDisplay() == models.PRStatusClosed:
		return styles.ErrorStyle
	default:
		return styles.PROpenStyle
	}
}

// prReviewStyle colors a review cell by its decision.
func prReviewStyle(review string) lipgloss.Style {
	switch review {
	case models.ReviewApproved:
		return styles.CleanStyle
	case models.ReviewChangesRequested:
		return styles.ErrorStyle
	default:
		return styles.SubtitleStyle
	}
}
