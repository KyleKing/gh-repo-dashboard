# Usability critique, 2026-08-03

A tui-critique pass over the binary built from `d2c794a`, driven in real tmux PTYs
at 220x50, 120x35, and 80x24, plus a live resize to 70x20, `NO_COLOR=1`, and the
single-repo focused launch. Data set: the real 62 repos under
`~/Developer/kyleking`. Findings below are verified against source, with file
references.

## Design specificity verdict

The domain thinking is authored: peers, template drift, notes files, and the
branch-to-PR mapping exist in no generic list-of-rows TUI. The rendering
underneath has not kept up. Detail views use fixed character budgets
(`internal/app/view.go:22-51`) that ignore terminal width in both directions:
at 220 columns branch names truncate to 18 characters inside a sea of margin,
and at 80 columns the same views hard-clip off the right edge. The repo list
got responsive treatment (columns collapse on resize); nothing else did.

## What is working

- The repo list's progressive load, filter counts in the modal, and the
  compositional filter → search → sort model all hold up with 62 repos
- Column collapse on the repo list reflows cleanly during a live resize; no
  frame corruption, no crash
- `NO_COLOR=1` now emits only bold/underline SGR, so the roadmap's NO_COLOR P1
  is fixed and can be closed
- The empty state now names a next step ("Press f to change filters, / to clear
  the search"), closing half of the roadmap's empty-state P2
- Write actions consistently confirm; blast radius of batch ops is the filtered
  set and the UI says so

## Priority issues

### P0: the Stashes tab renders garbage

Every stash row shows `stash@{0}`, the literal text `%(reflog:subject)`, and
"56 years ago". `GetStashList` passes for-each-ref atoms to `git stash list`
(`internal/vcs/git.go:315`), but stash list formats route through `git log`,
which needs `%gd` / `%gs` / `%ct`. The format string is emitted verbatim, the
index regex never matches (so every row is index 0), and the date parses to
epoch zero. `internal/vcs/git_exec_test.go:542` pins the broken format, so the
suite is green while the screen is wrong.

### P1: fixed column budgets ignore terminal width

`branchNameTruncLen = 20`, `upstreamTruncLen = 20`, `commitSubjectLen = 50`,
`checkNameColWidth = 34`, `messageTruncLen = 40` (`internal/app/view.go`).
Consequences observed:

- At 220 columns, `claude/practical-...` and five indistinguishable
  `continuous-integration / tests ...` check rows, with over half the screen
  empty
- At 80 columns, the detail view clips the tab bar mid-word ("Note"), amputates
  the CHECKED OUT and LAST COMMIT columns, and cuts the footer, because nothing
  in the detail path reads `m.width`

The repo list solved this once already; the fix is one shared width-aware
column layout used by every table.

### P1: fmt padding counts bytes, not display width

PR tab rows use `fmt.Sprintf("%s%-8s  %-40s  %s  %-18s  %s", ...)`
(`internal/app/view_detail.go:543`). Go pads by byte count, so any title with
an emoji or multibyte glyph underpads and the STATE/REVIEW/BRANCH columns
drift per row. At 80 columns one overflowing row wrapped and pushed a stray
character onto its own line, corrupting the frame. The `⚠` template suffix
(`view_repolist.go:251`) and `✓`/`↑` status glyphs are the same class of
hazard anywhere `fmt` does the padding instead of `lipgloss.Width`.

### P2: search state persists invisibly across navigation

Searching "yak", drilling in, pressing esc, then pressing `/` resumes editing
the old query (typing "mdit" produced "yakmdit" and a baffling zero-match
screen). The only cue is a small quoted string in the header. Either clear the
input buffer when `/` opens with a committed query, or select-all so the first
keystroke replaces it.

### P2: the power-user layer is invisible

`:` opens a bare prompt with no completion, no hint, and no footer change.
The `?` overlay never mentions `:` commands, `@:` repeat, or text objects,
even though `keymap.go` registers help text like "F+obj fetch" that the
overlay flattens to "F fetch (filtered repos)". The roadmap's
command-discoverability P2 is confirmed still open.

### P2: collapse priority keeps the wrong columns

At 70 columns the repo list keeps PR (which is "—" for nearly every repo
sitting on main) and drops PEERS and TEMPLATE, which are the actionable
signals for this fleet. Collapse order should follow information value, not
column order.

## Smaller observations

- Notes preview (`v`) shows only the first line, which in practice is a date
  heading (`doing.txt: ## 2026-08-01`), so the peek carries almost no
  information; first non-heading, non-blank line would
- UPSTREAM column is `origin/<same name>` for nearly every branch; it earns
  its width only when it diverges from the branch name
- REVIEW column reserves 18 characters to show "—" on most rows
- PR detail clips the description mid-word with no wrapping and truncates
  check names so matrix jobs are indistinguishable
- The `* ` current-branch prefix shifts that row two cells right in the branch
  table (misaligned under the BRANCH header)
- The focused single-repo launch works, but lands on a nearly empty screen
  (one branch row) for the richest question the tool can answer: "what is the
  state of the repo I am standing in"

## Health score: 26/36

| # | Heuristic | Score | Note |
|---|-----------|-------|------|
| 1 | Visibility of status | 3 | Progressive load and counts are good; search/filter state cue is too quiet |
| 2 | Match real world | 3 | git/jj vocabulary throughout; `%(reflog:subject)` on screen is the exception |
| 3 | Control and freedom | 3 | Esc always backs out; sticky search violates expectations |
| 4 | Consistency | 3 | Vim keys held; `* ` misalignment and per-view column behavior differ |
| 5 | Error prevention | 4 | Confirm gates on every write; refusal when a peer holds the branch |
| 6 | Recognition over recall | 2 | Footer good; `:` layer and text objects are memorize-or-miss |
| 7 | Flexibility | 3 | Operators and command mode exist; nothing surfaces them |
| 8 | Minimalist design | 2 | Wide-terminal waste plus dead columns (PR, REVIEW, UPSTREAM) |
| 9 | Error recovery | 3 | Errors render in-app; stash tab renders garbage without flagging it |
| 10 | Portability | 3 | NO_COLOR fixed, tmux fine; byte-width padding breaks under emoji |

## Fixed since the 2026-07 critique

- `NO_COLOR` is respected
- The empty state names a recovery action
- Repo list columns priority-collapse on narrow widths (list only)
