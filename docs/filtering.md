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

The atoms are `ahead`, `behind`, `clean`, `config_override`, `dirty`, `error`,
`git`, `has_notes`, `has_pr`, `has_stash`, `has_upstream`, `https`, `jj`, `ssh`,
and `template_drift`. A malformed expression reports a parse error rather than
silently matching nothing.

`template_drift` matches a copier-generated repo that is behind its template's
latest tag, or pinned to a commit or branch rather than a tag, where currency
cannot be judged at all.

`:filter all` clears both the mode and any predicate.

## Selection

`:select where <predicate>` marks repos as selected without changing what is
visible. The `sr` text object then scopes a batch operation to that selection.

## Sorting

Sort by `name`, `modified`, `status`, or `branch` from the sort modal (`s`) or
`:sort <mode>`. `R` reverses the current direction, and sorts combine by
priority.

## Search

`/` searches repo directory names as you type, case-insensitively. Substring
matches take priority, and fuzzy matching runs only when no name contains the
text. Search runs after the filter mode and before the sort.
