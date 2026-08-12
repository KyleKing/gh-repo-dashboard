# Interface

## Keys

Press `?` for the help overlay. It reads the live keymap, so it always matches
the binary you are running. Trust it over this page, which lists only enough to
start.

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `enter` | Open the repo under the cursor |
| `esc` | Go back |
| `/` | Fuzzy search by name |
| `f` / `s` | Filter and sort modals |
| `:` | Command mode |
| `r` | Refresh, clearing caches |
| `!` | Verbs for whatever the focused panel has selected |
| `?` | Help overlay |
| `q` | Quit |

Batch operations use a vim-style operator plus a text object, so `F` alone waits
for a scope. See [batch operations](./batch-operations.md).

Everything that writes lives behind `!` rather than on a key of its own, which
keeps the single-key namespace for moving around and means the verbs on offer
always match what is selected. `!` then a letter runs one:

| Panel | Verbs |
|---|---|
| Status | `f` fetch, `p` prune remote, `c` cleanup merged, `o` open on remote, `y` copy path |
| Branches | `s` switch, `p` push, `n` new PR, `d` delete, `o` open on remote, `y` copy name |
| PRs | `c` check out here, `m` squash-merge, `o` open in browser, `u` copy URL, `y` copy number |
| Peers | `y` copy path |
| Stashes | `a` apply, `d` drop, `o` toggle the full diff |
| Notes | `e` edit in `$EDITOR`, `y` copy path |

Anything that reaches the remote or destroys work asks first: push, new PR,
squash-merge, PR checkout, prune, cleanup, branch delete, and stash drop. Answer
with `y` or `enter` to go ahead, `n` or `esc` to back out. Switching, fetching,
and applying a stash run straight away, because each is recoverable: a failed
switch changes nothing and refuses branches already checked out here or in a
parallel checkout, and applying a stash leaves the stash in place.

## Views

The repository list is the starting view. `enter` opens the focused repo view: a
column of panels for Status, Branches, PRs, Peers, Stashes, and Notes beside a
detail pane that renders whatever the cursor sits on. Only the panels with
something to show are drawn, so a jj repo has no Stashes panel and a repo with no
open pull requests has no PRs panel; the Status panel names what is missing
instead ("no PRs, peers, or notes"). Each panel's border brackets its own jump
key, so `[B]ranches` is one `b` away. `tab` and `l` move to the next panel, `h`
to the previous, and `j`/`k` walk the whole column as one list, crossing from the
last row of one panel into the first of the next. Every panel gets an equal share
of the height before the busier ones get more, so a long list is never squeezed
out and a busy repo still arrives with its problems open.

`enter` hands the keyboard to the detail pane, where `j`/`k` scroll its text and
`esc` hands it back to the panel column. The blue border marks whichever region
is live. `O` opens the selected branch or pull request in its own full-screen
view.

`space` opens the universal find, scoped to the repo you are in (`;` opens it
fleet-wide from the list, and `*` widens a repo-scoped query). Type `#12` for a
pull request number, `b`, `s`, `n`, or `r` plus a space to narrow to branches,
stashes, notes, or repos, or plain text to search everything. `enter` opens the
highlighted result where it lives, `tab` marks rows, and `!` offers verbs for the
whole set, including committing its repos to the selected-repos text object so
batch operators compose with the find.

Launched from inside a repository, the scan finds only that one repo and the
detail view opens straight away. `esc` still steps back to the one-row list.

Filter (`f`), sort (`s`), and help (`?`) are modals over the current view. A
batch run replaces the view with a progress screen until it finishes.

## Repo list columns

NAME, BRANCH, STATUS, PEERS, PR, PRs, TEMPLATE, and MODIFIED.

PEERS counts the other checkouts of the same remote, whether sibling clones
picked up by the same scan or worktrees of this repo, so `⧉2` means two other
directories hold this project.

TEMPLATE reads a repo's `.copier-answers.yml` and shows the installed template
tag, `tag→latest` when the template has a newer tag, or the installed ref plus a
warning icon when the recorded value is a commit SHA or branch rather than a tag.

## Branch list columns

BRANCH, UPSTREAM, STATUS, PR, CHECKS, CHECKED OUT, and LAST COMMIT.

PR shows the open pull request's number with `✓` for approved, `✗` for changes
requested, or `draft`. CHECKS rolls its CI up as `passing 3/3`. CHECKED OUT reads
`here` for the branch this working copy is on and `⧉ folder` for a branch another
checkout holds, and `c` refuses to switch to the latter until that folder frees
it.

## Status symbols

The STATUS column shows the working tree as symbols:

- `+N` for N staged changes
- `~N` for N unstaged changes
- `?N` for N untracked files
- `!N` for N conflicted files
- `↑N` for N commits ahead of the tracking branch
- `↓N` for N commits behind
- `✓` when the working tree is clean

Alongside it, `N` marks a repo with one notes file and `N3` a repo with three.
Press `v` to open the expanded region under the table, which reads those files in
full for the repo under the cursor alongside its peers, local branches, and open
pull requests. `esc` closes the region, then clears the search, then the filters.

The list has no symbols for stash counts, worktree counts, or workflow runs.
Those appear in the focused view's panels, and the branch detail pane spells
workflow status out as words, such as `passing (3/4 passing)`. Selecting a pull
request names its failing and still-running checks, tallying the settled ones, so
a red rollup can be read without leaving the panel; `O` opens the full-screen PR
view, which adds each check's run time and the most recent comment.

## Notes files

The dashboard collects every configured notes filename it finds at a repo root,
so a repo holding three of them shows all three. The defaults are `.doing`,
`doing.md`, `doing.txt`, and `TODO.md`, and
[configuration](./configuration.md) overrides the list. The dashboard only reads
them: the Notes panel's `!e` verb hands the file to `$EDITOR` and reloads the
repo when the editor exits, so any change is one your editor made.
