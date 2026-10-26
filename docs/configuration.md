# Configuration

An optional TOML file at `$XDG_CONFIG_HOME/gh-repo-dashboard/config.toml`
(defaulting to `~/.config/gh-repo-dashboard/config.toml`) sets defaults. Every
key is optional, and a missing file means the built-in defaults.

```toml
# The fleet: what -cli, -script, and run scan when no paths are passed.
# The TUI scans where it is launched and switches to it from the filter panel.
scan_paths = ["~/Developer", "~/work"]

# Default for -depth
depth = 2

# Per-repo notes filenames checked at each repo root, every match is shown
notes_filenames = [".doing", "doing.md", "doing.txt", "TODO.md"]

# Lifetime of cached GitHub data (PRs, workflow runs), in minutes
cache_ttl_minutes = 5

# Keep pull request data between runs, one file per remote (default true)
cache_to_disk = true

# The TUI's fleet scope keeps repos with a commit by one of fleet_authors
# within fleet_active (default "180d", the syntax of a task's every)
fleet_active = "180d"
fleet_authors = ["me@example.com"]

# Diff viewer, in the form git's diff.external takes; unset means git decides
[diff]
external = "difft --color=always --display=inline --syntax-highlight=off"

```

Flags and positional paths on the command line take precedence over the file.
Task definitions are covered in [Tasks](#tasks).

## Fleet scope

The filter panel (`f`, then `f` again) switches the list from the directories
the TUI was launched in to the fleet: every repo under `scan_paths`, kept to those
where one of `fleet_authors` committed within `fleet_active`. `fleet_active` takes
`180d`, `26w`, `12h`, or `90m`, and must be positive. `fleet_authors` is a list of
author emails, matched case-insensitively against every ref with `git log --all`,
and defaults to the global `git config user.email`. With neither `scan_paths` nor an
author the filter panel's entry is dimmed and names what is missing.

Repos the filter drops are counted in the `fleet · 71 active of 85` chip above the
table. The filter applies only to the TUI's fleet scope: `-cli`, `-script`, and `run`
scan all of `scan_paths`, so a tool looking up a checkout finds repos you review
without committing to. A repo with no `.git` entry (jj without a colocated git
repo) cannot be read by `git log` and is always kept.

## Diff viewer

A stash's full patch (`!o` in the Stashes panel) renders through the same
external diff command git itself would run, so a repo or global
`diff.external = difft` is picked up with no further setup. `[diff] external`
overrides it for this tool alone.

The viewer runs without a terminal, which is how a tool decides to drop its
color and assume eighty columns, so the pane's width arrives in `COLUMNS` and
`DFT_WIDTH`, and color is forced through `CLICOLOR_FORCE`, `DFT_COLOR`, and
`DFT_DISPLAY=inline`. A flag written into the command itself still wins over
all of them, which is the way to configure a viewer that reads neither.

## Tasks

A `[[tasks]]` entry is a named shell command the dashboard runs in each repo it
selects, replacing a mani task list. Run one with
[`gh repo-dashboard run`](./cli.md#running-tasks), or from the
[Tasks tab](./interface.md#tasks-tab).

```toml
[[tasks]]
name = "gc"
desc = "aggressive gc for bloated repos"
run = "git -c pack.threads=3 -c pack.windowMemory=1024m gc --prune=now --aggressive"
where = "git_size > 1G"
concurrency = 1        # serial; default runs up to 8 repos in parallel
confirm = true
every = "30d"          # due once per repo per 30 days, when where also holds

[[tasks]]
name = "copier-update"
run = "copier update --UNSAFE --conflict=rej --defaults && git status --short"
where = "template ~ calcipy"
```

| Key | Meaning |
|-----|---------|
| `name` | Required, unique, no whitespace |
| `desc` | One-line description |
| `run` | Required. Run with `sh -c` in the repo directory, with `REPO_NAME` and `REPO_PATH` set |
| `where` | A [predicate](./filtering.md#predicate-expressions) choosing the repos the task applies to. Empty means every repo |
| `concurrency` | Repos run at once. `1` is serial, and unset or `0` runs up to 8 in parallel |
| `confirm` | Ask before running. `run --yes` skips the prompt |
| `every` | How long a successful run keeps a repo from being due, as `30d`, `2w`, `12h`, or `90m`. Without it the task is never due |
| `interactive` | Hand the terminal to the command for the one row under the cursor, as described under [Hand-offs](#hand-offs) |

A malformed task, including a `where` that does not parse, stops the program at
startup with the entry and key named. The built-in names `cleanup`,
`cleanup-preview`, `fetch`, `prune`, `refresh-prs`, and `shell` are reserved: a task with
one of them stops the program at startup too.

Each run writes the repo's combined output to
`<user cache dir>/gh-repo-dashboard/tasks/<task>/<repo>-<hash>/run.log` and a
last-run stamp beside it. A repo is due when `where` holds and it has never
succeeded or last succeeded longer ago than `every`. A failed run does not reset
the clock.

### Hand-offs

A task with `interactive = true` is a hand-off. It runs for exactly one row (the repo
under the cursor, or the PR, branch, or checkout selected in the focused view) with
the whole terminal, and the dashboard re-reads that repo when the command exits. A
fading status at the footer's right end reports the exit.

```toml
[[tasks]]
name = "review"
run = "second-look {pr}"
interactive = true

[[tasks]]
name = "lazygit"
run = "lazygit"
interactive = true
```

`run` may use `{path}`, `{repo}` (`owner/name` when the repo has a remote), `{pr}`, and
`{branch}`, each filled in shell-quoted. The command runs under `sh -c` in the row's
directory with `REPO_NAME` and `REPO_PATH` set. A hand-off appears in the `a` verbs menu
(repo list, focused view sections, and a find result set of exactly one) only where every
placeholder it names has a value, so `review` shows on a PR and not on a repo with none.
Its menu key is the first letter of its name that the menu has not already used, then a digit.

A hand-off may not set `where`, `every`, or `concurrency`, and may not name another
placeholder. Either stops the program at startup. Hand-offs do not appear in the Tasks tab
and `gh repo-dashboard run` does not know them. `confirm = true` asks first.

The built-in `shell` hand-off opens `$SHELL` in the repo, on `S` in the list and the focused
view. `shell` is reserved like the other built-in names.

## Caching

The dashboard caches GitHub pull request and workflow data for
`cache_ttl_minutes`, defaulting to 5. Press `r` in the TUI to clear the caches
and reload, or pass `--fresh` alongside `--cli`.

With `cache_to_disk` left on, the pull request list and the merged-PR head map
also outlive the process, in one file per remote under
`os.UserCacheDir()/gh-repo-dashboard` (`~/Library/Caches` on macOS,
`$XDG_CACHE_HOME` or `~/.cache` on Linux), so a cold start on a large fleet
renders without one `gh pr list` per repo. A file is read when the row that
needs it is drawn, never all of them at startup.

What lands there is pull request numbers, states, check counts, and titles. A
title from a private repository is the most any of it reveals, so the files are
written at mode 0600 under a 0700 directory. Pull request bodies and comment
text stay in memory and are never written. Set `cache_to_disk = false` to keep
everything in memory; `r` drops the files along with the memory caches.
