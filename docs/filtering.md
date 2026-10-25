# Filtering and sorting

Filtering composes in one direction: filter mode, then search text, then sort.
The `DIRTY` filter plus an `api` search yields dirty repos whose name contains
"api".

## Filter modes

`all`, `dirty`, `ahead`, `behind`, `has_pr`, `has_stash`, and `has_notes`. The
filter modal (`f`) toggles them, and several combine with AND logic.

## Predicate expressions

`:filter <expression>` takes a boolean expression instead of a single mode. The
same grammar works in `--filter` on the command line and in `:select where`.

```
:filter dirty and has_pr
:filter behind or ahead
:filter not clean
:filter (dirty or behind) and has_pr
```

Precedence runs `not`, then `and`, then `or`, and parentheses group.

The atoms are `ahead`, `behind`, `branch_pr`, `clean`, `config_override`, `dirty`,
`error`, `git`, `has_notes`, `has_pr`, `has_stash`, `has_upstream`, `https`, `jj`,
`ssh`, and `template_drift`. A malformed expression reports a parse error rather than
silently matching nothing.

`has_pr` matches a repo with any open pull request, and `branch_pr` one with a
pull request on the checked-out branch.

`template_drift` matches a copier-generated repo that is behind its template's
latest tag, or pinned to a commit or branch rather than a tag, where currency
cannot be judged at all.

Three comparisons and one call go beyond bare atoms:

```
git_size > 1G
template ~ calcipy
last_run(gc) > 30d
due(gc)
```

- `git_size` takes `>`, `>=`, `<`, `<=`, or `=` against a size with an optional
  binary `K`, `M`, `G`, or `T` suffix. It sums the files under `.git` (a
  non-colocated jj repo's backing store otherwise) and memoizes the answer for a
  minute
- `template ~ text` matches the copier template's source, ignoring case
- `last_run(task)` compares how long ago the [task](./configuration.md#tasks)
  last ran on the repo, using the same comparisons against an age such as `30d`.
  A repo that never ran it counts as infinitely old
- `due(task)` holds when the task's `every` schedule calls for a run. It ignores
  the task's own `where`

`--cli` output reports the same size as `git_size_bytes`.

`:filter all` clears both the mode and any predicate.

## Selection

`:select where <predicate>` marks repos as selected without changing what is
visible. The `sr` text object then scopes a batch operation to that selection.

## Sorting

Sort by `attention`, `name`, `modified`, `status`, or `branch` from the sort modal
(`s`) or `:sort <mode>`. `R` reverses the current direction, and sorts combine by
priority.

`attention` is the default. Its sort key is how urgently a repo needs you (failing
CI, then unpushed commits, then uncommitted work, then a shared branch or a pull
request waiting on you, then everything quiet), with the most recent change
breaking ties, and it draws the NEEDS YOU and QUIET groups. Any other sort
leading the priority list draws the list ungrouped, in the order that sort gives.

## Search

`/` searches repo directory names as you type, case-insensitively. Substring
matches take priority, and fuzzy matching runs only when no name contains the
text. Search runs after the filter mode and before the sort.
