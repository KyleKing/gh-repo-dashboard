# Focused repo view: panel grid and universal find

Second-generation design for the single-repo experience, replacing both the
tab bar and the scene presets that shipped with M15. Scope: ROADMAP M20.
Layout primitives come from [layout-and-density.md](layout-and-density.md).

## Problem with the shipped version

Tabs hide four of the five data sets behind a keypress, and scenes paper over
that by re-mapping tabs to scenarios. Both are modes: the user has to remember
where they are and what `1`-`4` mean. The 2026-08-04 review also found the
scene layer half-invisible (absent from help) and its per-scene tables
diverging from their designs. The fix is structural: show everything at once
and spend interaction on acting, not on revealing.

## Inspiration, and what to take

- lazygit: a grid of always-visible panels, each showing real content even
  when unfocused, with the focused panel expanded and a large detail pane
  rendering the selected item. Take the whole shape
- Telescope: typed queries reach any object; results render with a preview
  and Enter acts. Take the prefix grammar and the act-on-result model,
  extended to act on the whole result set, fleet-wide
- mini.files and magit informed earlier drafts; both lost to the panel grid
  because folds and column drilling cost an interaction per reveal

## Panel grid

Every data set is a bordered panel, all visible at once. Focus moves between
panels; the focused panel grows and the others shrink but never collapse
below a content line, so nothing is ever hidden behind a mode.

```
 gh-repo-dashboard   git · ssh   main ↑1 · clean   template ✓   CI ✓ main
┌─1 Status ───────────────────────┐┌─ main ──────────────────────────────────────┐
│ ↑1 vs origin/main               ││ 9 mins ago   feat: expand and center tables │
│ clean · 2 stashes · notes: none ││ upstream origin/main (↑1) · PR — · CI ✓     │
└─────────────────────────────────┘│                                             │
┌─2 Branches (2) ─────────────────┐│ recent commits                              │
│ > * main    ↑1    here    9m    ││   d2c794a feat: expand and center tables    │
│     fix/ci  ✓     —       5d    ││   7aa6be7 bump: version 1.3.3 → 1.4.0       │
└─────────────────────────────────┘│   91b30a1 docs: cover the focused repo view │
┌─3 PRs (1) ──────────────────────┐│                                             │
│ #11 bump mise-action      OPEN  ││                                             │
└─────────────────────────────────┘│                                             │
┌─4 Peers (0) ─┬─5 Stashes (2) ───┐│                                             │
│ none         │ wip: spike    3d ││                                             │
└──────────────┴──────────────────┘└─────────────────────────────────────────────┘
 1-5 panels  j/k nav  esc back  space find  enter detail
```

- The left column stacks the side panels; the right pane always renders the
  detail of the selected item in the focused panel (branch → commits and PR
  state, PR → description and latest comment, stash → diffstat, note → file
  body). This replaces per-tab detail screens
- `1`-`5` jump straight to a panel (the keys freed by retiring scenes);
  `h`/`l` or `[`/`]` cycle. `j`/`k` stay within the focused panel
- `enter` hands the keyboard to the detail pane, where `j`/`k` scroll its text
  and `esc` returns to the panel column; the border marks which region is live.
  `O` opens the selected branch or PR full screen
- Each panel's actions are its existing tab actions unchanged (`c` switch,
  `p` push, `N` new PR, `M` merge, `g` checkout PR); the footer shows the
  focused panel's keys
- Panels render for git and jj alike; jj drops the Stashes panel entirely
  instead of showing an n/a hint

## Relevance-driven sizing

Default space follows actionable state, not a fixed split. Each panel gets a
relevance score from cached data: dirty files or conflicted peers score high,
open PRs with recent activity score high, clean or empty sets score minimum.
Height is distributed by score through the existing column-engine weight
logic applied vertically. Focus overrides: the focused panel always gets its
full content up to the available height, and whatever no panel claims is
shared out again so the column reaches the bottom of the pane beside it. A
quiet repo therefore reads as a small stack of one-liners plus a large detail
pane; a busy repo arrives with its problems already expanded.

A note scores as high as a failing check. It is what the last session left for
this one, and unlike every other panel it holds something the repo cannot be
asked for, so the Notes panel spells out the note's text below the file list
and stays readable from whichever panel the view opened on.

## Universal find (the palette)

`space` opens a query line. A one-letter prefix narrows the type; bare text
searches everything. The same palette works in the fleet list, where scope
defaults to all repos; in the focused view scope defaults to this repo and
`*` widens to the fleet.

| Query | Matches |
|-------|---------|
| `#12` | PR number 12 (any state) |
| `b fix` | branches matching "fix" |
| `s wip` | stash messages |
| `n todo` | notes file names and first lines |
| `r dash` | repos (fleet scope) |
| `escape user` | everything: titles, branches, messages |

```
 find: #12                                      scope: all repos · 3 matches
 > mdit-py-plugins    #124  fix: inline double dollar to be rende…  OPEN
   calcipy            #12   feat: composable extras                 MERGED
   gh-sweep           #12   chore: bump mise-action                 OPEN
┌─ #124 ─────────────────────────────────────────────────────────────────┐
│ author camoz · updated 2 months ago · +12 -3                           │
│ head: fork camoz/master → master · no local branch                     │
└────────────────────────────────────────────────────────────────────────┘
 enter open  tab mark  ! act on set  esc close
```

Acting on the result set is the point, not just previewing it:

- Enter runs the type's default action on the highlighted row (branch →
  focus it in its panel, PR → open detail, repo → drill in)
- `tab` marks rows; with no marks, `!` targets every match. `!` opens the
  action menu for the set: type-appropriate verbs (checkout, open in
  browser, fetch) when the set is homogeneous, batch operators when the set
  is repos
- A repo result set commits to the existing selected-repos text object, so
  `F`/`P`/`C`/`R` compose with it exactly like `sr` today. "Which repos have
  PR 12" becomes: `space`, `#12`, `!`, pick the verb

The palette is the discoverability answer the `:` prompt never was: every
object is reachable by typing what you remember about it.

## Compact behavior

Below the compact breakpoint the grid becomes a single stack: the detail pane
moves inline under the focused panel and unfocused panels compress to their
title line plus one content line. `1`-`5` and the palette work unchanged.

## Performance

Unchanged budget from [fleet-navigation.md](fleet-navigation.md): panels
render from cached summaries immediately, network-backed cells (`CI`, PR
activity) fill in via the existing message flow, and the palette queries only
cache-resident data (no fetch on keystroke, fleet-wide included).

## Migration

Tabs and scenes are removed in the same milestone; their tables become the
panels and the freed number keys become panel jumps. Fixtures covering scenes
are rewritten against panels, and the wide fleet list's preview pane keeps
mounting the overview component, which becomes the Status panel here.

## What shipped

The open questions resolved as follows:

- The detail pane has no focus state of its own. It truncates to the height it
  is given, and long PR descriptions are cut on a word boundary. A scroll key
  can be added when a real description outgrows the pane
- `space` opens the palette in the focused view and `;` opens it fleet-side,
  because the list already spends `space` on selection
- Relevance scores ship fixed. They are three tiers (idle, present, urgent)
  rather than a tunable weight, which is enough to separate a busy panel from
  a quiet one without a config surface to explain
