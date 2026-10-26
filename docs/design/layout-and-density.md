# Layout engine and responsive density

Why the column engine and the density rules are shaped the way they are. What
they render is in [interface.md](../interface.md); how they are implemented is
in aragonite's `tui/table` and `internal/app/layout.go`.

## Problem

Every table except the repo list used to render through fixed character budgets
and `fmt.Sprintf("%-*s", ...)`, which pads by bytes. Wide terminals truncated
branch names to 18 characters inside empty margins, narrow terminals hard-clipped
columns and the footer, and any emoji or wide glyph desynced the row. The repo
list had its own one-off collapse logic that nothing else shared.

## Column engine

One renderer for every table. A column declares a minimum, a maximum, a weight
in the surplus, a hide priority, and an alignment. Fitting runs in this order,
and the order is the design:

1. Measure with `lipgloss.Width`, in display cells rather than bytes or runes.
   All padding and truncation go through that measure
2. While the row cannot fit even with every column at its minimum, hide the
   lowest-priority one and mark it in the header, so hidden data is announced
   rather than silent
3. Seat what survives at its minimum, then distribute the surplus to weighted
   columns proportionally, capped at each column's maximum
4. Truncate cell overflow with `…`, never clip mid-cell

Testing the fit against minimums rather than content widths is what keeps a
column from vanishing at a width where shrinking it would have been enough.

### Collapse priorities

Each table's priorities are ordered by information value observed against the
real fleet rather than by column position, and every set lives next to its
columns (`repoColSpecs` in `layout.go`, the rest in `columns.go` and
`panels.go`). On the repo list that means MODIFIED goes first and PRs second,
while NEEDS is the actionable fleet signal and never hides, with name and branch.
NAME, BRANCH, and NEEDS stop growing at 28, 24, and 32 cells, so a wide terminal
leaves the table narrow rather than stretching NEEDS across empty cells.

## Density

Two layouts, chosen by width alone and re-derived on every render, so a resize
needs no state to migrate.

| Layout | Width | Why |
|--------|-------|-----|
| compact | < 100 | five columns cannot be read at 80 cells, so compact is its own two-line-record layout (identity, then the NEEDS tokens) rather than a shrunken table |
| standard | >= 100 | the engine sizes the single table |

Past 100 the width is continuous rather than stepped, and past roughly 160 cells
the table hides nothing. There is no third breakpoint, because with the side
panel gone a wide terminal renders what a standard one does.

The table takes the wider of two rules: the single-column content width, and the
proportional one the focused grid uses (90% of the terminal, capped at 200).
Below about 156 cells the first wins, so a narrow terminal keeps every cell it
has. Above it the second wins, and the list and the focused view frame alike.

## Detail region

One detail region replaces the earlier preview pane and expanded region, and
aragonite's `tui/split` decides where it goes. From 140 columns (`detailSideMinWidth`)
the table caps at 100 cells (`tableMaxWidth`) and the region sits beside it behind a
one-cell rule, always shown. From 100 to 139 columns `v` opens it below the table at
60% of the body, and a terminal under 15 body rows says it is too short instead of
swallowing the key. Under 100 columns there is none, and `enter` opens the focused view.

The renderer draws the repo's name as `owner/name` (the origin remote's owner,
lowercased, else the parent directory), its NEEDS items spelled out, the facts the
sections below lack (changed paths, template, branches with unpushed commits), then
the focused view's PRs and CI sections on a copy of the model pointed at the cursor's
repo, the last five commits, and the notes. A fact with nothing to say is left out,
sections still loading say `reading…` rather than `none`, and a note taller than its
share has its middle elided so both ends survive.

Reads happen once per repo as the cursor lands on it: the local commit log, the
notes, the uncommitted paths, and the branch list and pull requests the `:prs`
fleet map also makes, so the map and the region share one cache.

## Consequences

- When collapse hides columns the header says so (`…+2`), and `s`/`f` still sort
  and filter on the hidden fields
- The footer collapses by the same priority idea, keeping `enter / f s ? q` and
  never clipping mid-hint
- Resize is stateless re-layout, so the cursor, filters, and view stack all
  survive a density change
