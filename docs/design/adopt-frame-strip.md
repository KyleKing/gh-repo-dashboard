# Adopting aragonite's strip frame and clicks

Written 2026-10-26 from a second-look session, for a later gh-repo-dashboard session to pick up. Delete it once the work lands.

aragonite v1.3.0 gives `tui/frame` the header and footer second-look's queue settled on, so both tools can draw one shell. second-look's queue (`internal/tui/listview.go`) is the reference caller. Bump to v1.3.0 or later first.

## What to adopt

| Piece | API | What changes here |
|---|---|---|
| Strip header | `frame.Frame{Strip: true}` | Tabs move from the right to after the title, facts take the right, and a hairline row under the header underlines the active tab. Hints draw bare (`enter open`, no brackets) |
| Count chips | `frame.Tab{Count, Alert, Stale}` | A tab can carry its count. `Alert` reds an inactive tab's count, `Stale` dims one read from a snapshot a refresh has not confirmed |
| Rule note | `Frame.Note` | What narrows the body (a filter query, the section scrolled past), drawn at the rule's right end |
| Panel | `Frame.Panel []string` | Rows about the item under the cursor, pinned above the footer on `skin.Panel` (Mantle), which the footer shares |
| Geometry | `frame.Top`, `frame.Rows(f, height)` | Use `Rows` for the body height instead of `frameChrome = 2` in `screen.go`, because `Strip` and `Panel` both add rows |
| Clicks | `frame.BodyRow(f, height, y)`, `frame.TabAt(skin, f, width, x, y)` | Map a `tea.MouseClickMsg` to a body row or a tab |
| Click outside a box | `overlay.At(box, x, y).Contains(x, y)` | Record where a floating box was blitted, and close it on a click outside |

## Decisions already made

- Clicking moves the cursor to the row under it and switches to a tab clicked on. The wheel moves the cursor when no box is open and scrolls the box when one is
- Clicks need `tea.MouseModeCellMotion` on the whole time, which leaves native text selection to shift-drag (option-drag in some terminals). second-look's README says so, and this README should too
- `Strip` retires the tabs-at-the-right header and bracketed hints once this repo sets it. After that, the next aragonite change makes it the only layout and deletes the field

## Done looks like

- `screen.go` sets `Strip: true` and sizes the body with `frame.Rows`
- Tabs carry counts where the dashboard has them (repos, PRs)
- The which-key panel and help close on a click outside them
- A click on a repo row moves the cursor there, with a test that drives a `tea.MouseClickMsg`
- The repo's own check ladder passes end to end
