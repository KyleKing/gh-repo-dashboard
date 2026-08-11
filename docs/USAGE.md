# Usage

<!-- Generated from internal/app/testdata/fixtures/ by `mise run docs:usage`; do not edit by hand. -->

Every example below is executed as a test (`TestFixtures`), so this page
cannot drift from the implementation. Commands (`:...`) can be typed after
pressing `:`; bare keys act on the repo list.

## Run a scoped batch from command mode

| Input | Result |
|---|---|
| `:fetch dirty and has_pr` | opens the batch view; starts batch Fetch All (dirty and has_pr); over 1 repos |

## Complete command names with tab

| Input | Result |
|---|---|
| press `:` `f` `tab` | input reads fetch |
| press `tab` | input reads filter |

## Filter with a single mode name

| Input | Result |
|---|---|
| `:filter dirty` | shows dirty, dirty-pr |
| `:filter all` | shows behind, clean, dirty, dirty-pr |

## Filter repos that have a notes file

| Input | Result |
|---|---|
| `:filter has_notes` | shows dirty-pr |
| `:filter dirty and has_notes` | shows dirty-pr |
| `:filter all` | shows behind, clean, dirty, dirty-pr |

## Filter by predicate expression

| Input | Result |
|---|---|
| `:filter dirty and has_pr` | shows dirty-pr; predicate: dirty and has_pr |
| `:filter behind or has_pr` | shows behind, dirty-pr |
| `:filter all` | shows behind, clean, dirty, dirty-pr; predicate: (cleared) |

## A busy repo shows every panel at once, and number keys move focus between them

| Input | Result |
|---|---|
| press `1` | panel: status; overview: Sync=↑2 ↓1 vs origin/feat/login Files=1 staged · 3 unstaged Peers=— Stashes=4 Notes=doing.md: wip Template=v0.9.1 → v0.10.0 PRs=#42 Add login flow CI=— |
| press `2` | panel: branches |
| press `3` | panel: prs |
| press `4` | panel: peers |
| press `5` | panel: stashes |
| press `6` | panel: notes |

## A quiet repo answers its overview without a keypress

| Input | Result |
|---|---|
| press `1` | opens the detail view; panel: status; overview: Sync=in sync vs origin/main Files=clean Peers=— Stashes=— Notes=— Template=— PRs=— CI=— |

## Review command history and repeat the last command

| Input | Result |
|---|---|
| `:filter dirty` | shows dirty, dirty-pr |
| `:history` | status: History: filter dirty |
| `:filter all` | shows behind, clean, dirty, dirty-pr |
| press `@` `:` | shows behind, clean, dirty, dirty-pr |

## Navigate the repo list with vim keys

| Input | Result |
|---|---|
| press `j` `j` | cursor on row 2 |
| press `G` | cursor on row 3 |
| press `g` `g` | cursor on row 0 |
| press `enter` | opens the detail view |
| press `esc` | opens the list view |

## Compose operators with text objects

| Input | Result |
|---|---|
| press `F` `d` `r` | opens the batch view; starts batch Fetch All (dirty); over 2 repos |

## Search repos by name

| Input | Result |
|---|---|
| press `/` `d` `i` `r` `t` `enter` | shows dirty, dirty-pr; search: dirt |

## Reopening search starts from an empty query

| Input | Result |
|---|---|
| press `/` `d` `i` `r` `t` `enter` | search: dirt; shows dirty, dirty-pr |
| press `/` | search: (cleared); shows behind, clean, dirty, dirty-pr |
| press `c` `l` `e` `a` `n` `enter` | search: clean; shows clean |

## Select repos by predicate, then fetch the selection

| Input | Result |
|---|---|
| `:select where dirty` | selects dirty, dirty-pr |
| press `F` `s` `r` | opens the batch view; starts batch Fetch All (selected); over 2 repos |

## Find across the fleet, then commit the result set to a batch operator

| Input | Result |
|---|---|
| press `;` | opens the find view |
| press `d` `i` `r` `t` `y` | find: dirty; matches: repo:dirty repo:dirty-pr |
| press `!` | opens the find view |
| press `s` | selects dirty, dirty-pr |
| press `F` `s` `r` | opens the batch view; starts batch Fetch All (selected); over 2 repos |
