package app

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/tui/frame"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/ui/styles"
)

type freshness int

const (
	freshnessNone freshness = iota
	freshnessMiss
	freshnessStale
	freshnessFresh
)

const (
	rankFailingCI = iota
	rankUnpushed
	rankDirty
	rankOther
	rankQuiet
)

// needs is a repo's attention signals. Its promotes method is the NEEDS YOU rule.
type needs struct {
	changed, untracked int
	ahead, behind      int
	ciFailing, ciTotal int
	ciRunning          bool
	ciPending          bool
	ciStale            bool
	shared             bool
	note               bool
	stashes            int
	drift              string
	due                []string
	turn               int
	prStale            bool
	prPending          bool
	open               int
}

func (n needs) promotes() bool {
	return n.changed+n.untracked+n.ahead > 0 || n.shared || n.ciFailing > 0 || n.turn > 0
}

func (n needs) rank() int {
	switch {
	case n.ciFailing > 0:
		return rankFailingCI
	case n.ahead > 0:
		return rankUnpushed
	case n.changed+n.untracked > 0:
		return rankDirty
	case n.promotes():
		return rankOther
	default:
		return rankQuiet
	}
}

func (m Model) needsOf(s models.RepoSummary) needs {
	if cached, ok := m.needsCache[s.Path]; ok {
		return cached
	}

	return m.computeNeeds(s)
}

// withNeedsCache holds each listed repo's needs so one render computes them once. Never keep its result.
func (m Model) withNeedsCache() Model {
	m.needsCache = make(map[string]needs, len(m.filteredPaths))

	for _, path := range m.filteredPaths {
		if s, ok := m.summaries[path]; ok {
			m.needsCache[path] = m.computeNeeds(s)
		}
	}

	return m
}

func (m Model) computeNeeds(s models.RepoSummary) needs {
	failing, total := s.CIFailing()
	n := needs{
		changed:   s.Staged + s.Unstaged + s.Conflicted,
		untracked: s.Untracked,
		ahead:     s.Ahead,
		behind:    s.Behind,
		ciFailing: failing,
		ciTotal:   total,
		ciRunning: failing == 0 && s.CIRunning() > 0,
		ciPending: m.awaiting(s.Path, fetchCI),
		ciStale:   m.freshness[fetchKey{s.Path, fetchCI}] == freshnessStale,
		shared:    m.shared[s.Path],
		note:      s.HasNotes(),
		stashes:   s.StashCount,
		due:       m.due[s.Path],
		turn:      prTurn(s.OpenPRs, m.viewer),
		prStale:   m.freshness[fetchKey{s.Path, fetchOpenPRs}] == freshnessStale,
		prPending: m.awaiting(s.Path, fetchOpenPRs),
		open:      len(s.OpenPRs),
	}

	if info := s.TemplateInfo; info != nil && info.Behind && info.LatestTag != "" {
		n.drift = info.LatestTag
	}

	return n
}

// awaiting is true once the cache came up empty, so a spinner never flashes over a value about to be drawn.
func (m Model) awaiting(path string, kind fetchKind) bool {
	return m.fetchPending(path, kind) && m.freshness[fetchKey{path, kind}] == freshnessMiss
}

// prTurn finds the first pull request waiting on viewer. The list read has no
// mergeability, so approved counts as mergeable unless a draft or failing check.
func prTurn(prs []forge.PullRequest, viewer string) int {
	if viewer == "" {
		return 0
	}

	for i := range prs {
		pr := &prs[i]
		mine := pr.Author == viewer

		switch {
		case mine && display.PRReviewStatus(*pr) == forge.ReviewChangesRequested:
			return pr.Number
		case mine && display.PRReviewStatus(*pr) == forge.ReviewApproved && !pr.IsDraft &&
			display.ChecksSummary(pr.Checks) != forge.StatusFailing:
			return pr.Number
		case !mine && containsString(pr.Reviewers, viewer):
			return pr.Number
		}
	}

	return 0
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}

	return false
}

type tone int

const (
	toneMuted tone = iota
	toneAccent
	toneWarning
	toneError
)

func (t tone) style() lipgloss.Style {
	switch t {
	case toneAccent:
		return styles.Skin.Accent
	case toneWarning:
		return styles.Skin.Warning
	case toneError:
		return styles.Skin.Error
	default:
		return styles.Skin.Muted
	}
}

func (t tone) frameTone() frame.Tone {
	switch t {
	case toneAccent:
		return frame.Accent
	case toneWarning:
		return frame.Warning
	case toneError:
		return frame.Error
	default:
		return frame.Muted
	}
}

type needsToken struct {
	text string
	tone tone
}

func (n needs) tokens() []needsToken {
	var tokens []needsToken
	add := func(text string, t tone) { tokens = append(tokens, needsToken{text, t}) }

	if work := workToken(n.changed, n.untracked); work != "" {
		add(work, toneWarning)
	}

	if n.ahead > 0 {
		add("↑"+strconv.Itoa(n.ahead), toneWarning)
	}

	if n.behind > 0 {
		add("↓"+strconv.Itoa(n.behind), toneMuted)
	}

	switch {
	case n.ciFailing > 0 && n.ciStale:
		add(fmt.Sprintf("✗ ci %d/%d", n.ciFailing, n.ciTotal), toneMuted)
	case n.ciFailing > 0:
		add(fmt.Sprintf("✗ ci %d/%d", n.ciFailing, n.ciTotal), toneError)
	case n.ciRunning:
		add("… ci", toneMuted)
	}

	if n.shared {
		add(conflictMark+" shared", toneWarning)
	}

	if n.note {
		add("✎ note", toneAccent)
	}

	if n.stashes > 0 {
		add("≡"+strconv.Itoa(n.stashes), toneMuted)
	}

	if n.drift != "" {
		add("→ "+n.drift, toneMuted)
	}

	if len(n.due) > 0 {
		add("⟳ "+strings.Join(n.due, ","), toneMuted)
	}

	return tokens
}

func workToken(changed, untracked int) string {
	var parts []string
	if changed > 0 {
		parts = append(parts, "~"+strconv.Itoa(changed))
	}

	if untracked > 0 {
		parts = append(parts, "?"+strconv.Itoa(untracked))
	}

	return strings.Join(parts, " ")
}

const needsGap = "  "

// renderNeeds drops tokens that do not fit from the end, the least urgent side, and marks the cut.
func (m Model) renderNeeds(n needs, width int, base lipgloss.Style, selected bool) string {
	tokens := n.tokens()
	if n.ciPending {
		tokens = append(tokens, needsToken{text: m.spinner.View(), tone: toneAccent})
	}

	var b strings.Builder

	used := 0

	for i, token := range tokens {
		gap := ""
		if i > 0 {
			gap = needsGap
		}

		cost := lipgloss.Width(gap) + lipgloss.Width(token.text)
		if used+cost > width {
			if used < width {
				b.WriteString(withSelection(styles.Skin.Muted, selected).Render(table.Ellipsis))
				used++
			}

			break
		}

		b.WriteString(base.Render(gap))
		b.WriteString(withSelection(token.tone.style(), selected).Render(token.text))

		used += cost
	}

	return b.String() + base.Render(strings.Repeat(" ", max(width-used, 0)))
}

func (m Model) prsCell(n needs, width int, base lipgloss.Style, selected bool) string {
	if n.prPending {
		return base.Render(padCell(m.spinner.View(), width))
	}

	if n.open == 0 {
		return base.Render(padCell("", width))
	}

	countStyle := base
	if !n.promotes() {
		countStyle = withSelection(styles.Skin.Muted, selected)
	}

	count := strconv.Itoa(n.open)
	text := countStyle.Render(count)
	used := lipgloss.Width(count)

	if n.turn > 0 {
		turnTone := toneAccent
		if n.prStale {
			turnTone = toneMuted
		}

		turn := table.Truncate(" #"+strconv.Itoa(n.turn)+" ←you", max(width-used, 0))
		text += withSelection(turnTone.style(), selected).Render(turn)
		used += lipgloss.Width(turn)
	}

	return text + base.Render(strings.Repeat(" ", max(width-used, 0)))
}

type attentionCounts struct {
	failing, turn, drift, due int
}

func (m Model) attentionCounts() attentionCounts {
	var counts attentionCounts

	for _, path := range m.filteredPaths {
		s, ok := m.summaries[path]
		if !ok {
			continue
		}

		n := m.needsOf(s)
		if n.ciFailing > 0 {
			counts.failing++
		}

		if n.turn > 0 {
			counts.turn++
		}

		if n.drift != "" {
			counts.drift++
		}

		counts.due += len(n.due)
	}

	return counts
}

func (m Model) attentionRank(path string) int {
	s, ok := m.summaries[path]
	if !ok {
		return rankQuiet
	}

	return m.needsOf(s).rank()
}

// groupedByAttention reports whether attention leads the sort, since any other
// order interleaves the NEEDS YOU and QUIET groups.
func (m Model) groupedByAttention() bool {
	enabled := m.enabledSorts()

	return len(enabled) > 0 && enabled[0].Mode == models.SortModeAttention
}

// promotedAt reports whether the listed repo at index i belongs in NEEDS YOU.
func (m Model) promotedAt(i int) bool {
	s, ok := m.summaries[m.filteredPaths[i]]

	return ok && m.needsOf(s).promotes()
}

// straddlesGroups reports whether the window shows rows from both groups.
func (m Model) straddlesGroups(w repoWindow) bool {
	for i := w.start + 1; i < w.end; i++ {
		if m.promotedAt(i) != m.promotedAt(i-1) {
			return true
		}
	}

	return false
}

// attentionGroups counts the listed repos in each group, and says which
// promoted. It is empty unless the list is grouped.
type attentionGroups struct {
	grouped  bool
	promoted []bool
	needsYou int
	quiet    int
}

func (m Model) attentionGroups() attentionGroups {
	if !m.groupedByAttention() {
		return attentionGroups{}
	}

	g := attentionGroups{grouped: true, promoted: make([]bool, len(m.filteredPaths))}

	for i := range m.filteredPaths {
		if m.promotedAt(i) {
			g.promoted[i] = true
			g.needsYou++
		} else {
			g.quiet++
		}
	}

	return g
}

// rule is the labeled rule that opens a group.
func (g attentionGroups) rule(promoted bool, width int) string {
	label := "QUIET" + compactSignalSep + strconv.Itoa(g.quiet)
	if promoted {
		label = "NEEDS YOU" + compactSignalSep + strconv.Itoa(g.needsYou)
	}

	return sectionRule(label, width)
}
