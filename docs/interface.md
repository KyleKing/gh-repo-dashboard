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
| `c` | Switch to the branch under the cursor |
| `p` | Push the branch with `--follow-tags` |
| `N` | Open a pull request for the branch |
| `M` | Squash-merge the pull request and delete its branch |
| `?` | Help overlay |
| `q` | Quit |

Batch operations use a vim-style operator plus a text object, so `F` alone waits
for a scope. See [batch operations](./batch-operations.md).

`p`, `N`, and `M` reach the remote, so each one asks for confirmation before it
runs. Answer with `y` or `enter` to go ahead, `n` or `esc` to back out. `c` runs
straight away because a failed switch changes nothing, and it refuses branches
already checked out here or in a parallel checkout.

## Views

The repository list is the starting view. `enter` opens the focused repo view: a
grid of panels for Status, Branches, PRs, Peers, Stashes, and Notes, all visible
at once, beside a detail pane that renders whatever the cursor sits on. A jj repo
has no Stashes panel. `1`-`6` jump straight to a panel, `tab` and `l` move to the
next one, `h` to the previous, and `j`/`k` move within the focused panel. Panel
height follows how much a section has to say, so a busy repo arrives with its
problems already open.

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
Press `v` to toggle a first-line preview of those files under the cursor, or open
the repo and focus its Notes panel for the full contents.

The list has no symbols for stash counts, worktree counts, or workflow runs.
Those appear in the focused view's panels, and the branch detail pane spells
workflow status out as words, such as `passing (3/4 passing)`. The PR detail view lists
each check by name with its status and run time, plus the most recent comment on
the pull request.

## Notes files

The dashboard collects every configured notes filename it finds at a repo root,
so a repo holding three of them shows all three. The defaults are `.doing`,
`doing.md`, `doing.txt`, and `TODO.md`, and
[configuration](./configuration.md) overrides the list. It reads these files and
never writes them.
