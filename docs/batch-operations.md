# Batch operations

Fetch, prune, merged-branch cleanup, and PR refresh are built-in
[tasks](./configuration.md#tasks) that run across many repositories at once.
They share the engine, the run view, the logs, and the last-run stamps with the
tasks in `config.toml`, so `gh repo-dashboard run fetch` runs the same code as
`!ff`. A run keeps going when one repo fails.

| Task | Does | Asks first |
|------|------|------------|
| `fetch` | Updates remote refs | no |
| `prune` | Clears stale remote-tracking refs | yes |
| `cleanup` | Deletes merged and squash-merged branches | yes |
| `cleanup-preview` | Lists what `cleanup` would delete | no |
| `refresh-prs` | Re-reads open PRs from GitHub, one repo at a time | no |

Repos run in parallel, up to 8 at a time, except `refresh-prs`, which runs one at
a time to stay inside the GitHub API budget.

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
| `sr` | Marked repos (`x` marks one, `V` opens a range, `:select` fills the same set) |

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

`:cleanup --dry-run [predicate]` runs `cleanup-preview`, which reports what would
be deleted and deletes nothing. It over-reports the checked-out branch, which `git branch -d` then
refuses to delete. The dry run never under-reports, so it names every branch
before anything disappears.

## From the Tasks tab

The [Tasks tab](./interface.md#tasks-tab) runs the same tasks, built-in or
configured, over the marked rows or a task's own `where`, previewing the repos
first. `:run <task>` opens that preview from any screen.

## The run view

A run opens a view with one row per repo: status, exit code, duration, and what
the task reported. Rows fill in as repos finish. `enter` on a row opens that
repo's log, `ctrl-c` cancels the run (which stops running repos and marks the
queued ones canceled), and `esc` returns once the run is over. Each repo's output
lands in the log file a configured task writes, and each run adds a line to the
task's history, which keeps the last 10.

## Safety

- Operator keys and `:commands` touch only the filtered or marked repositories
- `prune` and `cleanup` ask for confirmation, naming the repos and their count
- Each repo reports success or failure with a message, and one failure does not
  stop the run
