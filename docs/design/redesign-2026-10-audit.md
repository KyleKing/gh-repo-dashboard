# gh-repo-dashboard design audit

Read-only audit against the `tui-critique` and `hyperskills:tui-design` rubrics.
Evidence is VHS stills (not kept; `docs/images/` holds the post-redesign set) (52 real repos under
`~/Developer/kyleking`, wide is about 200x50 and narrow is 84x28), the golden
frames in `internal/app/testdata/`, and the source at `703a499` (v1.11.2,
aragonite v0.20.0). `NO_COLOR` and 16-color terminals were not captured, so
portability is unscored.

Three stills did not reach their intended state, and nothing below critiques them
as if they had:

- `*-07-palette.png` shows the list with 31 marked repos and a "Selected 31 repos"
  toast. The `;` palette never opened, so universal find is unreviewed
- `wide-09-branches.png` is identical to `wide-08-repo.png`, because the focused
  view already opens with Branches focused
- `narrow-11-prs.png` is a shell prompt, because the app had exited

## 1. Verdict

The tool is good at the plumbing a fleet view needs. It loads progressively with
honest placeholders (`·` pending against `—` absent), the shared column engine
collapses columns by priority and says so (`…+2`), the 84-column compact layout
is a real two-line record rather than a squeezed table, the `:filter` predicate
grammar and `!` operators compose, and every write that touches a remote is
gated behind a confirm. None of that is the problem.

It does not earn a daily slot for three reasons:

1. Nothing on the first screen points at work. The list sorts by name, so the 3
   dirty repos the header announces sit below the fold at 200x50, while the
   color budget goes to 44 green `✓` cells and 45 mauve `main` cells. The 8
   repos with failing default-branch CI show it as one small red digit at the
   far right of a `2/0/0/7` cell
2. The facts that answer "what should I do here" disagree with each other and
   live in different places. The PR column prints `— +2`, the filter dock
   says "Has PR 0" for the same fleet, and the focused repo view has no PR
   section and shows CI and template drift only after focusing Status. Finding
   one repo's state costs more hops than opening GitHub
3. The chrome is heavy and some of it is broken. Five rows of header before the
   column titles, a status toast that never fades and covers the footer in every
   later view, a help page that cannot scroll (16 of about 45 rows reachable at
   84x28), and a focused view of bordered boxes that resizes its own columns
   when focus moves. second-look's Ink shell already solved most of this in the
   same library

Rubric scores (0-4): status visibility 3, domain language 2, control 3,
consistency 2, error prevention 4, recognition 2, efficiency 3, minimalist
design 1, error recovery 3 (light evidence), portability not tested. That is
23/36 on the nine scored heuristics, which lands in the "acceptable, real
friction" band.

## 2. Findings

Severity follows the rubric: P0 blocks the task, P1 causes real difficulty, P2
is an annoyance with a workaround, P3 is polish. Tags say where the fix belongs:
`app` (this repo), `aragonite` (the shared TUI library, so second-look gains
too), `template` (my_go_template).

### Repo list (01, 02, 03, 07)

| id | sev | finding | evidence | proposed fix | tag |
|---|---|---|---|---|---|
| L1 | P1 | The rows that need action are not on screen. Default sort is Name ASC, and none of the 45 visible rows is dirty while the header says "3 dirty" | `wide-01`: 45 of 52 rows visible, every STATUS cell `✓` except `my_lua_template ↓1`. `internal/models/enums.go:99` (`SortModeName` is the zero value) | Default to an attention-first order, split into a NEEDS YOU group and a receding QUIET group (section 4). Add `SortModeAttention` as the zero value | app |
| L2 | P1 | Color is spent on healthy state. Clean renders green and every branch renders mauve, so the list reads as uniformly colorful and the one `↓1` does not stand out | `wide-01`: green `✓` on 44 rows, mauve `main` on 45. `internal/ui/styles/styles.go:83` (`CleanStyle` = Green), `:92` (`BranchStyle` = Mauve), `view_repolist.go:1032`, `:1102` | Clean renders blank. Branch renders in Body, and in Muted when it is the default branch. Reserve color for dirty, ahead, failing, and "your turn" | app |
| L3 | P1 | The CI column is hard to read. Four slash-separated counts under a glyph legend in the header, and the failing count, the only part that needs action, is the last digit in the rightmost data column | `wide-02`: `0/0/0/7`, `2/0/0/7`, `3/0/0/7` on 8 rows. `wide-01`: braille spinner on every row while 48 fetches run. `view_repolist.go:845` (`ciColumnTitle`), `ciListCell` | Collapse to one signal: blank when passing, `✗ 7/9` in Error when failing, `…2` while running. Move the per-outcome breakdown into the `v` region and the focused view. Let failing CI promote the row into NEEDS YOU | app |
| L4 | P1 | PR semantics disagree. The PR column shows `— +2` (no PR on this branch, two elsewhere), while the `p` filter and `has_pr` only count a PR on the checked-out branch, so the dock says "Has PR 0" beside 11 visible rows with `+N` | `wide-03`: "Has PR 0". `wide-01`: `— +1`, `— +2` rows. `internal/filters/predicate.go:47` (`s.PRInfo != nil`, described as "has an open pull request" at `:71`), `view_repolist.go:704` (`prCell`) | Show the repo's open PR count, and mark the ones whose turn is yours. Make `has_pr` mean any open PR and add `branch_pr` for the current-branch case | app |
| L5 | P2 | PEERS is `—` on all 52 rows yet survives collapse longer than MODIFIED | `wide-01`. `layout.go:117` (Priority 3) | Drop the column. A peer conflict becomes a `⚠` in the attention cell | app |
| L6 | P2 | The notes marker `N` is right-aligned inside STATUS, so it reads as a sixth column with no header | `wide-01` x≈1008. `view_repolist.go:1060` (`statusCell`) | Fold notes into the attention cell as a glyph | app |
| L7 | P2 | Five rows of chrome above the column titles (tab chips, rule, title chips, a sort chip alone on its row, blank). The sort chip uses the same blue fill as the active tab, so the eye cannot tell a mode from a setting | `wide-01` rows 1-5. `styles.go` `SortBadgeStyle` and `skin.TabActive` both fill Blue | One header line: identity, counts with problems in their status color, tabs at the right. Sort shows as `▾` on its column title | app, aragonite |
| L8 | P2 | At 220 columns NAME takes 62 cells and BRANCH 44 for names under 25 characters | `TestGoldenRepoListBreakpoints/220x50.golden`. `layout.go:114-115` (no `Max`) | Give NAME and BRANCH a `Max` (about 28 and 24) so surplus goes to the attention cell | app |
| L9 | P2 | Template drift is Warning yellow on 15 rows, which competes with real failures for the same attention | `wide-01` TEMPLATE column. `templateCellStyle` in `view_repolist.go` | Treat drift as a chore: Muted `→ v0.15.2`, a header count, and the existing `template_drift` predicate. See decision D2 | app |
| L12 | P1 | Below 100 columns CI disappears entirely. The compact second line lists peers, PRs, template, stashes, notes, and age, never CI | `narrow-01`: calcipy reads `2 PRs · 5.5.2 · 1 month ago` with 7 failing runs. `view_compact.go:69-98` (`compactSignals`) | Make the second line the NEEDS cell (section 3), so failing CI leads it | app |
| L10 | P3 | The mark glyph collides with the name (`•app-template`, `>•calcipy`) | `wide-07`, `narrow-07`. `view_repolist.go:1017-1022` | Draw cursor and mark in a fixed two-cell gutter with `skin.CursorBar` and a mark glyph | app |
| L11 | P3 | Cursor is `> ` on a Surface0 row, the pre-Ink look | `wide-01`. `layout.go:99`, `view_repolist.go:211` | Adopt `skin.Cursor` with `skin.CursorBar` and `skin.Selection` from aragonite v0.22 | app |

### `v` expanded region (02)

| id | sev | finding | evidence | proposed fix | tag |
|---|---|---|---|---|---|
| E1 | P2 | The region takes 60% of the body whatever it holds. For calcipy (three facts, no notes) about 20 rows are blank and the table shrinks from 45 rows to 18 | `wide-02`. `view_repolist.go:445` (`expandPercent = 60`), aragonite `region.padBottom` | Size to content, capped at 60%. In aragonite, let `region.Region.Render` treat height as a ceiling (add `Region.Lines(s, width, maxHeight)` returning only what it drew) so each tool picks fixed or fitted. See D4 | aragonite, app |
| E2 | P3 | The repo identity prints twice (opening rule `kyleking/calcipy · git · ssh`, closing rule `kyleking/calcipy · no notes`) and `Peers —` spends a row on nothing | `wide-02`, `narrow-02` | Omit empty facts. Make the closing caption optional in `region` (it already handles an empty label) and pass none here | app |
| E3 | P2 | The region is where the compressed list cells should unfold, yet it holds no CI breakdown, no template detail, and no dirty-file list | `wide-02` | Add CI per workflow (failing names first), template `current → latest`, and the first few changed paths | app |

### Filter, sort, command, help (03, 04, 05, 06)

| id | sev | finding | evidence | proposed fix | tag |
|---|---|---|---|---|---|
| M1 | P1 | Help is one page that cannot scroll and is not scoped to the view. At 84x28 it shows 16 of about 45 rows with every description clipped. At 200x50 the close hint falls off the bottom. `a` and `f` each appear twice with different meanings | `narrow-06`, `wide-06`. `view_modals.go:16-48` (pads to height, no scroll, no wrap) | Build a `keyhint.Hint` tree per view and render it with `keyhint.Column` in a scrolling `overlay.Box`, chord pages through `keyhint.At`, as second-look does at `internal/tui/view.go:479-523` | app |
| M2 | P2 | Command completions render as bare prose over the table's last rows. Descriptions run past 150 columns, and at 84 columns they cut to about 12 characters, which says nothing | `wide-05`, `narrow-05`. `renderCompletionCandidates` via `view.go:73-78` | Render candidates with `keyhint.Column` (prose wraps at the description column) inside `overlay.Box` | app |
| M3 | P3 | Filter dock header "If on" does not say it is a match count, and filter keys share mauve with branch names | `wide-03` | Header "Matches". Keys in `skin.Key` | app |
| M4 | P3 | Sort offers Name, Modified, Status, and Branch, with no order by attention, CI, or PR | `wide-04`. `enums.go:99-103` | Add Attention (default, see L1) | app |

### Focused repo view (08, 09, 10)

| id | sev | finding | evidence | proposed fix | tag |
|---|---|---|---|---|---|
| R1 | P1 | A status message never fades and replaces the footer. The `StatusMsg` handler sets the text and returns no clear, and the status line is overlaid on the bottom row, so after one message the footer is gone from every later view. The "Marks cleared" Kyle saw on entering a repo came from `esc` clearing the 31 marks the script made, then sticking | `wide-08` to `wide-11`, `narrow-08`, `narrow-10`: "Marks cleared" where the footer belongs. `update.go:171-172` (no `clearStatusAfterDelay`), `update.go:952` (`statusCmd("Marks cleared")`), `view.go:82-85` (`overlayBottomLine`) | Route `StatusMsg` through `withFadingStatus`. Give status a slot that never covers keys (the footer's right end, D8) | app |
| R2 | P1 | The view hides the repo's CI, template drift, and open PRs. Status shows only sync and files, CI and template appear only when Status is focused, and there is no PRs panel at all, although the list counts `+1` and the PRs tab lists #20 for this repo | `wide-08` vs `wide-11`. `view_panels.go:100-126` (`statusPanel`), `panels.go:19-25` (no PR panel id) | A header facts line that carries every attention item (section 4), and a PRs section in the left column | app |
| R3 | P2 | Panels are stacked bordered boxes. About 60% of the left column and 55% of the detail pane are blank on a quiet repo, and four borders frame three facts | `wide-08` | One surface. Sections in the left column split by labeled rules, the detail pane separated by one vertical rule, no boxes | app, aragonite |
| R4 | P2 | Focus changes the layout. The panel column narrows from about 25% to 15% when the detail pane takes focus, which drops the branch date and stash date columns | `wide-09` vs `wide-10`. `view_panels.go:446-455` (`sideColumnFocusedPercent`) | Fixed split. Show focus with the accent rule and cursor only | app |
| R5 | P2 | The detail pane opens with facts the Status panel already gave (`upstream origin/main`, `tracking ✓` beside "in sync vs origin/main") | `wide-08` | Lead the detail with what is not elsewhere: commits, or the PR's failing checks | app |
| R6 | P2 | At 84x28 the detail box shows a "recent commits" heading with zero commits under it, and Status loses its second line | `narrow-08`, `narrow-10` | In the stacked layout, guarantee the detail pane a minimum of rows or omit a heading with nothing under it | app |
| R7 | P3 | Selection highlight has gaps between cells, here and on the PRs tab | `wide-08` branch row, `wide-11` PR row | `table.Row(layout, cells, rowStyle)` in aragonite that pads and paints the gutters in the row style, used by every table | aragonite |

### PRs tab (11)

| id | sev | finding | evidence | proposed fix | tag |
|---|---|---|---|---|---|
| P1 | P2 | The footer renders directly under the table instead of at the bottom, so its position differs from the repo list | `wide-11` row 13. `view_prlist.go:72` | A shared frame that pins the footer to the last row (X1) | aragonite, app |
| P2 | P2 | AUTHOR and CHECKS truncate (`app/depen…`, `passing 6…`) while ACTIVITY holds `—` in about 30 cells | `wide-11`. `view_prlist.go:35-43` (ACTIVITY has Weight 1, no Max) | `Max` on ACTIVITY, CHECKS as glyph and count, bot authors shortened | app |
| P3 | P3 | Two rows of query chrome (`Open this repo 1 open`, `is:open sort:updated-desc`) | `wide-11` | One line, with the raw query behind `?` or `:pr-query` | app |

### Cross-cutting and tooling

| id | sev | finding | evidence | proposed fix | tag |
|---|---|---|---|---|---|
| X1 | P2 | Footer notation is mixed (`[a]ctions`, `v expand`, `pre[v]iew`, `[/] cycle views`, `* everywhere`) and the list footer shows 12 hints. The app keeps its own priority-collapse footer engine while second-look keeps its own dim-inapplicable one | `wide-01`, `wide-11`. `view_repolist.go:1143-1290` (`footerHint`, `fittingHints`, `renderHints`), second-look `internal/tui/hints.go:63-80` (`dimLine`) | aragonite `keyhint.Fit(s Styles, hints []Hint, width int) string`, with `Hint.Priority` and `Hint.Off`. It brackets keys one way, dims keys that do not apply, and drops lowest priority first. Both tools delete their copies | aragonite |
| X2 | P2 | Header, tab strip, footer, and status slot are hand-assembled per view, which is how the PRs footer floats and the toast covers keys | `view.go:61-87`, `view_prlist.go:72`, second-look draws the same shell in `internal/tui/view.go` | aragonite `tui/frame`: `frame.Render(s skin.Skin, f Frame, width, height int) string` where `Frame{Title, Facts []Fact, Tabs []Tab, Body []string, Hints []keyhint.Hint, Status string}`. `Fact{Text, Tone}` with Tone in Muted/Warning/Error. This is the second consumer, which meets aragonite's "what earns a package" rule | aragonite |
| X3 | P2 | Pinned to aragonite v0.20.0. v0.22.0 adds the Ink flavor, `skin.Selection` tint, `skin.Card` and `skin.Edge`, `tui/card`, and `theme.Tint`/`theme.Mix` | `go.mod`. Module cache `aragonite@v0.22.0/tui/skin/skin.go` | Bump, then build styles from `skin.New(theme.Ink(), accent)` like second-look `internal/tui/styles.go:61-110`. See D6 | app |
| X4 | P2 | The style file names about 40 styles, and Blue serves the title, footer keys, help headings, sort chip, active tab, and focus border at once | `internal/ui/styles/styles.go:52-200` | Map onto skin roles (Title, Subtitle, Heading, Body, Muted, Accent, Key, Success, Warning, Error) and keep only domain styles (diff, PR state) | app |
| T1 | P2 | Stills were scripted blind against a live fleet, so three states were missed and the first frame caught 48 CI fetches mid-flight | `wide-07`, `wide-09`, `narrow-11`, `wide-01` CI spinners | Template `demo:stills` task alongside `demo`: a `stills.tape` per breakpoint that uses VHS `Wait /regex/` before each `Screenshot`, run against a fixture fleet replayed through `ghcassette`. second-look's `demo/stills.tape` is the reference | template |
| T2 | P3 | Nine orphan goldens. No test writes `TestGoldenDetailTabBreakpoints/*` (6 files, the retired tabbed view) or `TestGoldenPanelGridBreakpoints/prs-*` (3 files, a PRs panel that no longer exists), and `-update` never deletes | `golden_test.go:105-111` runs only `branches`. `grep -rn DetailTab --include='*.go'` finds nothing | Template `test:golden-orphans` task: regenerate into an empty copy of `testdata` and fail on any file left only in the original | template |
| T3 | P3 | Golden breakpoints use a three-repo fixture, so no golden shows a fleet where most rows are quiet, which is the case the hierarchy has to get right | `TestGoldenRepoListBreakpoints/*` | Add a 20-repo fixture with 3 needing attention to the breakpoint set | app |

## 3. Proposed information hierarchy

### Repo list

The always-visible surface answers one question: which repos need me, and why.

Keep as columns: NAME, BRANCH (Muted when it is the default branch), a NEEDS cell,
PRs, and MODIFIED (Muted). NEEDS is the single attention signal. It concatenates
glyph-and-count tokens in a fixed order, each in its own status color, and it is
empty for a quiet repo:

- `~3 ?1` uncommitted work (Warning), `↑2` unpushed (Warning), `↓1` behind (Muted)
- `✗ ci 7/9` failing default-branch CI (Error), `… ci` running (Muted)
- `⚠ shared` branch also checked out in a peer (Warning)
- `✎ note` a notes file present (Accent)
- `≡1` stashes (Muted)

PRs shows the open count, and `#18 ←you` in Accent when one is waiting on you.
Template drift leaves the row and becomes a header count plus the existing
`template_drift` predicate (decision D2). PEERS and the CI breakdown leave the
table and move into `v`.

Rows group under two labeled rules. NEEDS YOU holds any repo whose NEEDS cell is
non-empty or has a PR waiting on you, sorted by severity (failing CI, then
unpushed, then dirty, then the rest). QUIET holds the rest by most recently
modified, with names in Muted and every other cell blank. Repo names below are
from the real fleet, and the attention values are illustrative.

```text
gh-repo-dashboard  ~/Developer/kyleking  52 repos  3 dirty  8 failing CI  2 your turn  15 behind template   Repos  PRs
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
  NAME                        BRANCH                  NEEDS                                PRs           MODIFIED ▾
── NEEDS YOU · 11 ──────────────────────────────────────────────────────────────────────────────────────────────────────
▌ calcipy                     main                    ✗ ci 7/9                             2             1 month ago
  corallium                   main                    ✗ ci 7/10                                          1 month ago
  second-look                 @ +3                    ~4 ?2  ↑1                            1 #31 ←you    7 mins ago
  my_lua_template             main                    ↓1  ✗ ci 2/2                                       2 weeks ago
  djot-fmt                    main                    ✎ note                               1             1 month ago
  gh-lazydispatch             main                    ~1                                   2 #9 ←you     3 days ago
── QUIET · 41 ──────────────────────────────────────────────────────────────────────────────────────────────────────────
  aragonite                   main                                                         1             1 hour ago
  my_go_template              main                                                                       2 hours ago
  gh-repo-dashboard           main                                                         1             3 days ago
  chartui                     main                                                                       1 month ago

[j/k] move  [enter] open  [v] details  [f] filter  [:] command  [?] help                           Marks cleared
```

At 84 columns the compact record keeps its shape, with NEEDS as the second line
in place of today's template-and-age line, so failing CI survives the narrow
layout (today it does not appear at all below 100 columns).

`v` then answers "why" for the row: changed paths, CI per workflow with failing
names first, template `current → latest`, relevant peers, and notes, sized to its
content.

### Focused repo view

One surface. The header line carries identity and every NEEDS token. The left
column lists sections split by labeled rules with no boxes, each sized to its
content, and empty sections collapse to their rule. The detail pane sits behind
one vertical rule and shows the selected row in full. Focus moves the cursor and
the accent on the section rule, never the split. The PR titles come from
`wide-02`, and the checks and description are illustrative.

```text
Repos › calcipy   git · main · in sync · clean   ✗ ci 7/9   template 5.5.2   2 PRs                        Repos  PRs
────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
── PULL REQUESTS · 2 ──────────────────── │ #142 [WIP] Improve README and documentation content
▌ #142 Improve README and docs   draft    │ draft · head docs/readme → main · updated 3 days ago
  #141 Composable extras         ✗ 2      │
── CI · main ──────────────────────────── │ checks  ✗ lint  ✗ test (3.12)  · 6 passed
  ✗ test         7 of 9 failing           │
  ✓ docs         2 passing                │ Rewrites the README around the composable extras and
── BRANCHES · 1 ───────────────────────── │ moves the tool reference into docs/.
  * main         ✓        1 month ago     │
── STASHES · 0 ────────────────────────── │
── NOTES · 0 ──────────────────────────── │
                                          │
                                          │

[j/k] move  [tab] section  [enter] focus detail  [a] actions  [space] find  [esc] back  [?] help
```

Section order follows the attention order of the list (PRs and CI before
branches), and the section holding a NEEDS item opens with the cursor in it.
The breadcrumb's full path moves into the Status detail, where it already
appears as `path`.

## 4. Open decisions

D1. Default order of the repo list.
(a) Attention groups: NEEDS YOU then QUIET, as mocked.
(b) Keep name order and add an "attention" filter preset bound to a key.
(c) Attention sort with no group rules.
Recommendation: (a). The groups are the hierarchy, and (b) keeps the morning
question one keypress away every morning.

D2. What counts as "needs attention."
(a) Local work (dirty, unpushed, shared branch), failing default-branch CI, and
PRs whose turn is yours. Template drift, stashes, and notes show in NEEDS but do
not promote a row.
(b) Everything in (a) plus template drift and notes promote.
(c) A config predicate (`attention = "dirty or ahead or failing or turn"`)
parsed by `filters.ParsePredicate`, defaulting to (a).
Recommendation: (a). Fifteen repos behind the template would fill NEEDS YOU with
chores. (c) adds a config surface with no second caller yet.

D3. The CI signal.
(a) One token: `✗ ci 7/9` when failing, blank when passing.
(b) A run-history strip per repo (`▁▁█▁`) for the last N runs.
(c) Keep the four-way breakdown with a better header.
Recommendation: (a), with the breakdown moved into `v` and the focused view.

D4. `v` region height.
(a) Content height, capped at 60% of the body.
(b) Fixed 60% (today), which keeps the table still while the cursor moves.
(c) Replace `v` with a side preview pane at 160 columns and up.
Recommendation: (a). The table moves when the region grows, but a short region
leaves 20 more rows of the fleet visible, and stability matters less once NEEDS
YOU is at the top.

D5. Focused view shape.
(a) One surface with labeled section rules and a fixed split, as mocked.
(b) Keep the boxed grid and fix only the blank space and the focus shift.
(c) A single column like second-look's outline, with the detail opening inline
under the selected row.
Recommendation: (a). It keeps lazygit's side-by-side detail, which suits
branches and PRs, and drops the boxes.

D6. Theme and accent.
(a) Ink always, as second-look does, with a gh-repo-dashboard accent other than
second-look's Lavender. Sapphire is unused in the app today.
(b) Keep `theme.Detect` and Macchiato.
(c) Ink with Blue, which collides with `skin.Heading`.
Recommendation: (a) with Sapphire. The accent then tells the two tools apart at
a glance, which is the skin's stated purpose.

D7. The PRs tab now that second-look owns the review queue.
(a) Keep it as is.
(b) Narrow it to PRs you authored, joined to local branches and checkouts, and
leave review requests to second-look.
(c) Remove it and rely on the PRs column and section.
Recommendation: (b). The branch-to-PR join is the part second-look cannot do,
and the review queue is the part it does better.

D8. Where status messages go.
(a) The footer's right end, fading after the delay.
(b) Their own line above the footer, which costs a row in every view.
(c) The header line.
Recommendation: (a). It never covers a key and needs no extra row.
