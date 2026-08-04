# Focused repo view

Redesign of the single-repo experience: the screen shown when discovery finds
one repo (launching from inside a checkout) and when drilling into a repo from
the list. Scope: ROADMAP M15. Layout primitives come from
[layout-and-density.md](layout-and-density.md).

## Problem

The richest question the tool can answer is "what is the state of the repo I am
standing in", and today the answer is one branch row under a tab bar. Peers,
notes, template drift, stash age, CI, and PR activity all exist in the data
model and none of them render on arrival.

## Inspiration, and what to take

- lazygit: persistent panels build spatial memory; every pane is actionable in
  place. Take the always-visible status pane and per-pane keybindings
- k9s: breadcrumb drill-down plus `:` jumps; header carries live context
  (cluster, counts). Take the dense header strip and the idea that the list
  IS the interface
- mactop / btop: mode toggles that re-purpose the same screen for different
  questions. Take single-key scenario presets rather than more tabs
- gh-dash: sectioned lists defined by queries, each section with its own
  actions. Take the "sections are saved questions" framing for the PR pane

## Structure

Three stacked zones. The overview pane is the same component the wide-layout
preview panel mounts, rendered here at full width.

```
 gh-repo-dashboard   git · ssh · ⧉ 0 peers                            main ↑1 · clean
 ~/Developer/kyleking/gh-repo-dashboard          template v0.9.1 → v0.10.0   CI ✓ main
┌─ Overview ──────────────────────────────────────────────────────────────────────────┐
│ Sync      main ↑1 vs origin/main · last commit 9 mins ago (feat: expand and center) │
│ Files     clean                                                                     │
│ Stashes   2 · latest "wip: spike" 3 days ago                                        │
│ Notes     none                                                                      │
│ PRs       none open · last merged #42 "fix(app): surface status" 2 days ago         │
└─────────────────────────────────────────────────────────────────────────────────────┘
  Branches (1)  │  Stashes (2)  │  Worktrees (1)  │  PRs (0)  │  Notes (0)
  BRANCH        UPSTREAM   STATUS   PR   CHECKS   CHECKED OUT   LAST COMMIT
> * main        ·          ↑1       —    —        here          9 mins ago
 tab panes  1-4 scenes  enter drill  c switch  p push  N new PR  esc back  ? help
```

Every overview line is a summary of a tab, so the pane doubles as a table of
contents: the cursor can move into the overview and Enter jumps to the
corresponding tab. Rows with nothing to say render dim and short (`Notes
none`), keeping quiet repos quiet.

## Scenes (mactop-style toggles)

Number keys swap the lower zone between presets tuned to a scenario. Tabs
remain the raw material; scenes are curated arrangements of them, so the
implementation is a mapping from scene to (tab, sort, columns), not a new view
system.

| Key | Scene | Lower zone shows |
|-----|-------|------------------|
| 1 | work (default) | Branches tab, current branch first |
| 2 | review | PRs with checks, review state, and latest-comment age; Enter opens PR detail |
| 3 | sync | Peers and worktrees: every checkout of this remote, its branch, dirty state, ahead/behind, with same-branch conflicts flagged ([fleet-navigation.md](fleet-navigation.md)) |
| 4 | maintain | Template drift, notes files, stash list, and `:cleanup --dry-run` preview counts |

The active scene renders in the footer (`[1 work] 2 review 3 sync 4 maintain`)
so modal state is never ambiguous. Scenes only exist in the focused view;
the fleet list keeps its single layout.

## Compact behavior

At compact width the overview pane keeps only Sync and Files lines and the
header drops the path line. Scenes still work; their tables render in the
compact two-line record style.

## Performance

Nothing on this screen may block on the network. The overview renders from
`RepoSummary` and cache-resident data immediately; CI and PR-activity lines
show `…` placeholders filled by the same Tea message flow the list uses.
Scene switches reuse already-loaded tab data; the review scene triggers at
most one gh call per repo visit (see the API budget in
[fleet-navigation.md](fleet-navigation.md)).

## Open questions

- Whether the overview cursor (Enter jumps to tab) is worth its complexity in
  v1, or the pane starts static with scenes carrying all navigation
- Whether scene state persists per repo or resets to work on every entry
- jj repos: the sync scene maps worktrees to workspaces cleanly, but stash
  lines render `—`; decide whether the row hides or shows the mapping note
