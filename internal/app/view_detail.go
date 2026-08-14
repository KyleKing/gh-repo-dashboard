package app

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/table"
)

// renderRepoDetailBreadcrumbs names the repo alone. Its identity facts (vcs,
// protocol, detached/dirty state, pull request, parallel checkouts, config
// drift) live in the Status panel's full preview instead.
func (m Model) renderRepoDetailBreadcrumbs() string {
	summary, ok := m.summaries[m.selectedRepo]
	if !ok {
		return styles.TitleStyle.Render("repo-dashboard")
	}

	home := styles.SubtitleStyle.Render("Repos")
	sep := styles.SubtitleStyle.Render(" > ")
	repo := styles.TitleStyle.Render(summary.Name())

	return home + sep + repo
}

func (m Model) renderRepoDetail() string {
	return m.renderPanelGrid()
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

// stripRemotePrefix drops the leading "<remote>/" off a tracking ref. The
// remote name is boilerplate in a narrow column, where the branch name it
// prefixes is what a reader is scanning for.
func stripRemotePrefix(ref string) string {
	if _, rest, ok := strings.Cut(ref, "/"); ok {
		return rest
	}

	return ref
}

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
		colUpstream:    stripRemotePrefix(row.branch.Upstream),
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

func checkoutState(c models.PeerCheckout) string {
	parts := []string{}
	if c.Dirty {
		parts = append(parts, "dirty")
	}

	parts = append(parts, c.TrackingSummary())

	if c.IsLocked {
		parts = append(parts, "locked")
	}

	return strings.Join(parts, " ")
}

func relativeOrDash(t time.Time) string {
	if t.IsZero() {
		return emDash
	}

	return models.RelativeTime(t)
}

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

// prStateCell renders the pull request's state with its review decision
// folded in as a glyph, since ACTIVITY took the review column's width.
func prStateCell(pr *models.PRInfo) string {
	if glyph := pr.ReviewGlyph(); glyph != "" {
		return pr.StatusDisplay() + " " + glyph
	}

	return pr.StatusDisplay()
}
