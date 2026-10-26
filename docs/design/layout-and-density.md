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

## Preview pane

From 140 columns (`previewMinWidth`) the repo list caps its table at 100 cells
(`tableMaxWidth`) and the rest of the terminal previews the repo under the cursor
behind the focused view's split rule: its name and branch, the NEEDS items spelled
out, then the focused view's own PRs and CI sections and the last five commits.
The preview borrows the focused view's section renderers on a copy of the model
pointed at the cursor's repo, so it shows what the list already holds. The one
read it adds is the local commit log, cached per repo and requested once as the
cursor lands on a repo. Below 140 columns the preview is absent and the table
takes the width it did before.

## Expanded region

`v` opens one region below the table for the repo under the cursor, answering
the questions the table has no column for: peers, local branches with unpushed
commits, open pull requests, and the repo's notes in full.

Its height follows its content, capped at 60% of the body and bounded so the table
keeps six rows. A fact with nothing to say is left out, and the closing rule
carries no caption. Sections still loading say `reading…` rather than `—`, and a
note taller than the space has its middle elided so both ends survive. The table
takes whatever lines the region leaves, so it grows and shrinks as the cursor
moves between repos with more or less to say.

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
