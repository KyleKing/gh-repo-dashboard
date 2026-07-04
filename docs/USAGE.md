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

## Filter by predicate expression

| Input | Result |
|---|---|
| `:filter dirty and has_pr` | shows dirty-pr; predicate: dirty and has_pr |
| `:filter behind or has_pr` | shows behind, dirty-pr |
| `:filter all` | shows behind, clean, dirty, dirty-pr; predicate: (cleared) |

## Navigate the repo list with vim keys

| Input | Result |
|---|---|
| press `j` `j` | cursor on row 2 |
| press `G` | cursor on row 3 |
| press `g` | cursor on row 0 |
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

## Select repos by predicate, then fetch the selection

| Input | Result |
|---|---|
| `:select where dirty` | selects dirty, dirty-pr |
| press `F` `s` `r` | opens the batch view; starts batch Fetch All (selected); over 2 repos |
