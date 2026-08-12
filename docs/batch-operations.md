# Batch operations

Three maintenance tasks run across many repositories at once: fetch, prune, and
merged-branch cleanup. They run sequentially, report progress in a modal, and
keep going when one repo fails.

## Scoping the run

Each operation is a verb under the `!` leader that waits for a text object, so
the scope is always explicit before anything runs.

| Verb | Task |
|----------|------|
| `!f` | Fetch all |
| `!p` | Prune remote |
| `!c` | Cleanup merged branches |
| `!r` | Refresh PR data |

| Text object | Scope |
|-------------|-------|
| `ar` | All visible repos |
| `br` | Behind repos |
| `dr` | Dirty repos |
| `nr` | Repos with notes |
| `pr` | Repos with PRs |
| `sr` | Selected repos |

`!fdr` fetches every dirty repo, and repeating the verb (`!ff`) runs it over the
filtered set. `@:` repeats the last `:command`.

Command mode takes a predicate instead: `:fetch dirty and has_pr`,
`:prune behind`, or `:cleanup`. With no argument the command runs over whatever
is currently visible.

## What each task runs

Fetch all updates remote refs, running `git fetch --all --prune` or
`jj git fetch --all-remotes`.

Prune remote clears stale remote-tracking refs with `git remote prune origin`.
It is a no-op on jj, which prunes during fetch.

Cleanup merged deletes local branches and bookmarks already merged into
main or master, plus branches squash-merged through a pull request. Squash
merges are invisible to `git branch --merged`, so cleanup compares each branch
tip OID against the head OIDs of the merged PRs that `gh` reports.

- git deletes true merges with `git branch -d` and verified squash merges with
  `git branch -D`, skipping the current branch and any branch checked out in a
  worktree
- jj deletes bookmarks that are ancestors of main, plus verified squash-merged
  bookmarks, via `jj bookmark delete`

## Previewing a cleanup

`:cleanup --dry-run [predicate]` reports what would be deleted and deletes
nothing. It over-reports the checked-out branch, which `git branch -d` then
refuses to delete. The dry run never under-reports, so it names every branch
before anything disappears.

## Safety

- Batch operations only start from the repository list view
- They only touch the currently filtered or selected repositories
- Each repo reports success or failure with a message, and one failure does not
  stop the run
