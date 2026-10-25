# Interface

## Keys

Press `?` for the legend of the view you are in. It is a scrolling box (`j`/`k`,
`pgup`/`pgdn`), and a key that waits on a second press, like `a` or `!`, opens its
own page that `esc` leaves. Trust it over this page, which lists only enough to
start.

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `enter` | Open the repo under the cursor |
| `esc` | Go back |
| `/` | Fuzzy search by name |
| `f` / `s` | Filter and sort docks |
| `:` | Command mode |
| `r` | Refresh, clearing caches |
| `x` / `V` | Mark the row under the cursor / open a line-wise range motions extend |
| `a` | Verbs for what is marked, or for whatever the current view has selected |
| `!` | Batch operator, then a text object |
| `?` | Help legend |
| `q` | Quit |

Batch operations use a vim-style operator plus a text object, so `!f` alone
waits for a scope. `x` marks rows and `sr` is the text object naming them, so
`x x x !fsr` fetches three repos you pointed at. See
[batch operations](./batch-operations.md).

Everything that writes lives behind `a` rather than on a key of its own, which
keeps the single-key namespace for moving around and means the verbs on offer
always match what is marked or selected. `a` then a letter runs one:

| Where | Verbs |
|---|---|
| Repo list | `f` fetch, `p` prune remote, `c` cleanup merged, `r` refresh PRs, each then a text object |
| Status | `f` fetch, `p` prune remote, `c` cleanup merged, `o` open on remote, `y` copy path |
| Branches | `s` switch, `p` push, `n` new PR, `d` delete, `o` open on remote, `y` copy name |
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

The header line names the screen, carries its counts (`3 dirty` in the warning
color, `8 failing CI` in the error color, `2 your turn` in the accent, `15 behind
template` and `1 task due` muted, with zero counts left out; active filters and
marks in the accent), and shows the tab strip at its right. The footer is pinned to the last row with the keys on offer bracketed
inside their words, and a status message fades at its right end after three
seconds. A sort shows as `▴` or `▾` on its column title.

The repository list is the starting view. `enter` opens the focused repo view: a
column of panels for Status, Branches, Peers, Stashes, and Notes beside a
detail pane that renders whatever the cursor sits on. Only the panels with
something to show are drawn, so a jj repo has no Stashes panel; the Status panel
names what is missing instead ("no PRs, peers, or notes"). Each panel's border brackets its own jump
key, so `[b]ranches` is one `b` away. `tab` and `l` move to the next panel, `h`
to the previous, and `j`/`k` walk the whole column as one list, crossing from the
last row of one panel into the first of the next. Every panel gets an equal share
of the height before the busier ones get more, so a long list is never squeezed
out and a busy repo still arrives with its problems open.

`enter` hands the keyboard to the detail pane, where `j`/`k` scroll its text and
`esc` hands it back to the panel column. The accent border marks whichever region
is live, and the panel column narrows while the detail pane holds focus, since
nothing is being selected in it then. `O` opens the selected branch in its own full-screen view.

`space` opens the universal find, scoped to the repo you are in (`;` opens it
fleet-wide from the list, and `*` widens a repo-scoped query). Type `#12` for a
pull request number, `b`, `s`, `n`, or `r` plus a space to narrow to branches,
stashes, notes, or repos, or plain text to search everything. `enter` opens the
highlighted result where it lives, `tab` marks rows, and `a` offers verbs for the
whole set, including committing its repos to the marked-repos text object so
batch operators compose with the find.

Launched from inside a repository, the scan finds only that one repo and the
detail view opens straight away. `esc` still steps back to the one-row list.

Filter (`f`) and sort (`s`) dock above the table, and help (`?`) floats over the
current view. A batch run replaces the view with a progress screen until it
finishes.

## Repo list columns

NAME, BRANCH, NEEDS, PRs, and MODIFIED. NEEDS is the one place the list says
what a repo asks of you, and it is blank for a quiet repo. Each token has its own
color, and the order never changes:

| Token | Meaning | Color |
|---|---|---|
| `~3 ?1` | 3 changed files and 1 untracked | warning |
| `↑2` | 2 commits not pushed | warning |
| `↓1` | 1 commit behind the upstream | muted |
| `✗ ci 7/9` | 7 of 9 default-branch workflows failing | error |
| `… ci` | default-branch CI still running | muted |
| `⚠ shared` | the branch is also checked out in a sibling clone or worktree | warning |
| `✎ note` | a notes file is present | accent |
| `≡2` | 2 stashes | muted |
| `→ v0.16.0` | the copier template has a newer tag | muted |
| `⟳ gc` | a [task](./configuration.md#tasks) is due on the repo | muted |

PRs counts the repo's open pull requests, whichever branch is checked out. When
one waits on you it adds `#31 ←you` in the accent: your pull request has changes
requested, or is approved with no failing check, or a review was requested of
you. `has_pr` matches any open pull request, and `branch_pr` matches a pull
request on the checked-out branch.

Rows fall into two groups under labeled rules. NEEDS YOU holds a repo with
uncommitted or unpushed work, a branch shared with another checkout, failing
default-branch CI, or a pull request waiting on you. It sorts failing CI first,
then unpushed, then dirty, then the rest, newest first within each. QUIET holds
the others by most recent change, with the names muted. Behind, notes, stashes,
template drift, and due tasks show in NEEDS but never move a repo up, so chores
do not bury real work. Sorting by anything else (`s`) turns the groups off.

Below 100 columns each repo is a two-line record. The first line holds the name,
branch, and PRs, and the second holds the NEEDS tokens followed by the age, so
failing CI still shows.

Pull requests and CI draw from the last cache the moment a repo's summary loads,
however old, and fresh reads replace them as they land. A value still waiting on a
fresh read draws muted, and a spinner shows only where nothing was cached.

## Branch list columns

BRANCH, UPSTREAM, STATUS, PR, CHECKS, CHECKED OUT, and LAST COMMIT.

PR shows the open pull request's number with `✓` for approved, `✗` for changes
requested, or `draft`. CHECKS rolls its CI up as `passing 3/3`. CHECKED OUT reads
`here` for the branch this working copy is on and `⧉ folder` for a branch another
checkout holds, and `c` refuses to switch to the latter until that folder frees
it.

## Loading marks

`·` marks a value still being read and `—` one the dashboard has read and found
empty, so a quiet repo can be told from an unread one. The header counts the same
thing twice over: `Loading 8/54` while the repos are being read, then `Fetching 31`
counting down the pull request, template, and CI reads that follow.

Press `v` to open the expanded region under the table. It is as tall as what it
holds, up to 60% of the body, and answers why a row asks for you: the changed
paths, CI per workflow with the failing ones first and the jobs that broke, the
template's installed and latest tags, peer checkouts, local branches with
unpushed commits, open pull requests, and the repo's notes in full. A fact with
nothing to say is left out. `esc` closes the region, then clears the search, then
the filters.

The list has no column for stash counts beyond `≡N`, worktree counts, or the
per-workflow CI breakdown. Those appear in the focused view's panels, and the
branch detail pane spells workflow status out as words, such as
`passing (3/4 passing)`. Selecting a pull request names its failing and
still-running checks, tallying the settled ones, so a red rollup can be read
without leaving the panel.

## Notes files

The dashboard collects every configured notes filename it finds at a repo root,
so a repo holding three of them shows all three. The defaults are `.doing`,
`doing.md`, `doing.txt`, and `TODO.md`, and
[configuration](./configuration.md) overrides the list. The dashboard only reads
them: the Notes panel's `!e` verb hands the file to `$EDITOR` and reloads the
repo when the editor exits, so any change is one your editor made.
