# Focused repo view: panel grid and universal find

Why the single-repo view is a grid of always-visible panels and why the palette
exists. What they look like and which keys drive them is in
[interface.md](../interface.md). Layout primitives come from
[layout-and-density.md](layout-and-density.md).

## Problem with tabs and scenes

The first version put five data sets behind a tab bar, then added scene presets
that re-mapped the tabs to scenarios. Both are modes, so the user has to
remember where they are and what `1`-`4` mean, and the 2026-08-04 review found
the scene layer absent from help with its per-scene tables already diverging
from their designs. The fix was structural: show everything at once and spend
interaction on acting rather than on revealing.

lazygit supplied the shape (a grid where unfocused panels still show real
content, beside one large detail pane). Telescope supplied the palette's prefix
grammar and its act-on-the-result-set model. mini.files and magit lost to the
grid because folds and column drilling each cost an interaction per reveal.

## Relevance-driven sizing

Default height follows actionable state rather than a fixed split. Each panel
scores from cached data (dirty files and conflicted peers high, clean or empty
sets minimum), and height is distributed by score through the column engine's
weight logic applied vertically. The focused panel always gets its full content
up to the available height, and whatever no panel claims is shared out again so
the column reaches the bottom of the pane beside it. A quiet repo reads as a
stack of one-liners plus a large detail pane, and a busy repo arrives with its
problems already open.

Scores ship as three fixed tiers (idle, present, urgent) rather than a tunable
weight, which separates a busy panel from a quiet one without a config surface
to explain.

A note scores as high as a failing check. It is what the last session left for
this one, and unlike every other panel it holds something the repo cannot be
asked for, so the Notes panel spells the text out below the file list.

## Universal find

The palette is the discoverability answer the `:` prompt never was: every object
is reachable by typing what you remember about it, with a one-letter prefix to
narrow the type and bare text to search everything.

Acting on the result set is the point rather than previewing it. `tab` marks
rows, `!` targets every match when nothing is marked, and a repo result set
commits to the existing selected-repos text object, so the batch operators
compose with it exactly as `sr` does. "Which repos have PR 12" becomes `space`,
`#12`, `!`, pick the verb.

Scope defaults to the repo you are in and `*` widens it, because a fleet-wide
default would make the common case the one that needs narrowing. `space` opens
it in the focused view and `;` fleet-side, since the list already spends `space`
on selection.

## Compact and performance

Below the compact breakpoint the grid becomes a single stack: the detail pane
moves inline under the focused panel, and unfocused panels compress to a title
line plus one content line.

The budget is unchanged from [fleet-navigation.md](fleet-navigation.md). Panels
render from cached summaries immediately, network-backed cells fill in through
the existing message flow, and the palette queries only cache-resident data, so
no keystroke ever fetches.
