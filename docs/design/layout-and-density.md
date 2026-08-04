# Layout engine and responsive density

Design for the shared column engine and the three-breakpoint layout system.
Findings that motivate this live in
[2026-08-03-usability-critique.md](2026-08-03-usability-critique.md). Scope and
sequencing live in [ROADMAP.md](../../ROADMAP.md) (M13, M14).

## Problem

Every table except the repo list renders through fixed character budgets
(`internal/app/view.go:22-51`) and `fmt.Sprintf("%-*s", ...)`, which pads by
bytes. Wide terminals truncate branch names to 18 characters inside empty
margins, narrow terminals hard-clip columns and the footer, and any emoji or
wide glyph desyncs the row. The repo list has its own one-off collapse logic
that nothing else shares.

## Column engine (M13)

One table renderer used by the repo list, detail tabs, PR views, and every
future table. A table is a `[]Column` plus rows of cells:

```go
type Column struct {
    Title    string
    Min      int   // never render narrower; collapse instead
    Max      int   // 0 = unbounded
    Weight   int   // share of surplus width; 0 = fixed at content width
    Priority int   // collapse order: highest number hides first
    Align    Align // numbers right, text left
}
```

Rules, in order:

1. Measure content with `lipgloss.Width` (display cells, never bytes or runes).
   All padding and truncation go through this measure; `%-*s` on user content
   is retired
2. Fit fixed columns at their content width, capped at `Max`
3. Distribute surplus to weighted columns proportionally, capped at `Max`
4. While the total exceeds the terminal width, hide the highest-`Priority`
   column and show a one-cell `…` marker in the header region so hidden data
   is announced rather than silent
5. Truncate cell overflow with `…`, never clip mid-cell

### Collapse priorities, repo list

Ordered by information value observed with the real fleet, not by column
position. PR (current-branch PR) is nearly always `—` on main, so it goes
first. PEERS and TEMPLATE are the actionable fleet signals and survive longest.

| Column | Priority (hides first = high) | Weight | Notes |
|--------|------------------------------|--------|-------|
| NAME | never | 3 | Min 16 |
| BRANCH | never | 2 | Min 10 |
| STATUS | never | 0 | fixed, content width |
| PEERS | 3 | 0 | |
| PRs | 4 | 0 | open count |
| TEMPLATE | 5 | 1 | drift arrow kept whole: `5.0.3→5.1.0` |
| MODIFIED | 2 | 0 | |
| PR | 1 | 0 | current-branch PR; first to hide |
| CI | 6 | 0 | lands in M17; stays visible at standard width |

### Collapse priorities, branch tab

UPSTREAM only renders when it differs from `origin/<branch>`; the common case
collapses to a `·` marker so BRANCH gets the width. CHECKED OUT collapses to a
glyph column (`here`, `⧉ name`) before hiding.

## Breakpoints (M14)

Three named layouts selected by width, re-evaluated on every resize:

| Name | Width | UX |
|------|-------|----|
| compact | < 100 | different UX, not a shrunken table: two-line rows, no side panel |
| standard | 100 - 159 | current single-table layout, engine-sized |
| wide | >= 160 | master-detail: table left, live preview panel right |

Height also gates: below 20 rows the preview panel is dropped even at wide
widths.

### Compact (80x24 target)

A shrunken 8-column table cannot work at 80 cells. Compact renders two-line
records: identity on line one, signals on line two, with a subtle separator by
zebra spacing rather than borders.

```
 repo-dashboard   62 repos   22 dirty              Name (ASC)
> gh-repo-dashboard              main            ↑1
    ⧉2 · 1 PR · v0.9.1→v0.10.0 · 6 mins ago
  backup-yak-shears-py           yak-shears-py   +1 ↑5 ↓80
    ⧉2 · 6 stashes · 1 month ago
  calcipy                        main            +1 ↑2
    2 PRs · 5.0.3→5.1.0 · 1 day ago
 j/k nav  enter open  f filter  / search  ? help
```

Line two shows only non-empty signals, so quiet repos stay one line tall in a
future refinement; v1 keeps uniform two-line rows for cursor simplicity.
Detail views in compact stack sections vertically (no side-by-side), and the
tab bar abbreviates counts (`Br 7 · St 0 · Wt 1 · PR 12 · No 0`).

### Standard (100-159)

The current layout, but every column sized by the engine. No other UX change.

### Wide (>= 160): preview side panel

The unused margin becomes a live preview of the selected repo, so Enter is no
longer required to answer "what state is this repo in". j/k updates the panel
from cached summaries; it never blocks on fetch (sections render placeholders
until their data arrives, same pattern as the list).

```
 repo-dashboard   62 repos   22 dirty                                      Name (ASC)
  NAME                    BRANCH          STATUS     PEERS PRs TEMPLATE   MODIFIED    │ gh-repo-dashboard          git · ssh
> gh-repo-dashboard       main            ↑1         —     —   v0.9.1→…   6 mins ago  │ main ↑1 · clean
  gh-star-search          main            ✓          —     1   v0.9.1→…   2 days ago  │
  gh-sweep                main            ✓          —     1   v0.9.1→…   2 days ago  │ Branches   main ↑1 (here)
  hk-debugging            main            ✓          —     —   —          8 months    │ Stashes    2 · latest: wip: spike (3d)
  jj-diff                 @               ✓          —     —   v0.9.1→…   2 days ago  │ Peers      none
  karabiner-actions       main            +1 ↑3  N   —     —   —          2 days ago  │ Notes      none
  KyleKing                main            ✓          —     —   —          2 days ago  │ Template   v0.9.1 → v0.10.0
  KyleKing.github.io      main            ✓          —     —   —          15 mins ago │ CI (main)  ✓ ci · ✓ release
  mdformat-admon          main            ✓          —     —   2.9.3      2 days ago  │
                                                                                      │ PR #—      no PR for main
 j/k nav  enter open  v notes  f filter  s sort  / search  r refresh  ? help  q quit
```

The panel is the same component as the focused view's overview pane
([focused-repo-view.md](focused-repo-view.md)), rendered at panel width.
Building it once and mounting it twice keeps the implementation KISS.

Bubble Tea v2 notes: the frame is already `tea.View` with alt-screen; the
split is plain lipgloss `JoinHorizontal` with the engine sizing each half.
No new dependency is required, though `bubbles/v2` viewport backs the panel
scroll if the preview grows taller than the window.

## Interaction consequences

- Hidden-column marker: when collapse hides columns, the header ends with
  `…+2`, and `s`/`f` modals still filter and sort on hidden fields
- The footer collapses by the same priority idea: keep
  `enter / f s ? q`, drop the rest, never clip mid-hint
- Resize is stateless re-layout; cursor, filters, and view stack survive
  breakpoint changes

## Testing

- Unit tests on the engine: surplus distribution, collapse order, `Max` caps,
  emoji and CJK cells measured by display width
- Golden frames per breakpoint (80x24, 120x35, 220x50) for repo list, detail
  Branches, and PR tab, replacing per-view ad hoc sizing tests
- A resize fixture: 220 → 80 → 220 asserting identical final frame
