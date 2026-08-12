# Layout engine and responsive density

Why the column engine and the density rules are shaped the way they are. What
they render is in [interface.md](../interface.md); how they are implemented is
in `internal/ui/table` and `internal/app/layout.go`.

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
`panels.go`). On the repo list that means the current branch's own PR goes
first, since it is nearly always `—` on main, while PEERS and TEMPLATE are the
actionable fleet signals and survive longest. Name, branch, and status carry no
priority and never hide.

## Density

Two layouts, chosen by width alone and re-derived on every render, so a resize
needs no state to migrate.

| Layout | Width | Why |
|--------|-------|-----|
| compact | < 100 | eight columns cannot be read at 80 cells, so compact is its own two-line-record layout rather than a shrunken table |
| standard | >= 100 | the engine sizes the single table |

Past 100 the width is continuous rather than stepped, and past roughly 160 cells
the table hides nothing. There is no third breakpoint, because with the side
panel gone a wide terminal renders what a standard one does.

The table takes the wider of two rules: the single-column content width, and the
proportional one the focused grid uses (90% of the terminal, capped at 200).
Below about 156 cells the first wins, so a narrow terminal keeps every cell it
has. Above it the second wins, and the list and the focused view frame alike.

## Expanded region

`v` opens one region below the table for the repo under the cursor, answering
the questions the table has no column for: peers, local branches with unpushed
commits, open pull requests, and the repo's notes in full.

Its height is a function of the body height alone, never of the selected repo or
of what is still loading, so moving the cursor swaps text in place instead of
resizing the table underneath. It takes 60% of the body, bounded so the table
keeps six rows and the region keeps its head plus a note line. Sections still
loading say `reading…` rather than `—`, and a note taller than the space has its
middle elided so both ends survive.

Branches and pull requests come from the same per-repo read the `:prs` fleet map
makes, cached per repo for the session, so opening the region costs one fetch per
repo and reopening it costs none. A terminal too short to seat the region says so
in the status line instead of swallowing the keypress.

## Consequences

- When collapse hides columns the header says so (`…+2`), and `s`/`f` still sort
  and filter on the hidden fields
- The footer collapses by the same priority idea, keeping `enter / f s ? q` and
  never clipping mid-hint
- Resize is stateless re-layout, so the cursor, filters, and view stack all
  survive a density change
