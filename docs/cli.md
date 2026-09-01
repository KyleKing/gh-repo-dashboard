# Command line

```bash
# Scan the config file's scan_paths, or with no config the enclosing repo
# (walking up from the current directory), or the current directory itself
gh repo-dashboard

# Scan specific directories
gh repo-dashboard ~/projects ~/work

# Limit scan depth (default 1)
gh repo-dashboard -depth 2 ~/Developer
```

Flags and positional paths override the [config file](./configuration.md).

## Flags

| Flag | Effect |
|------|--------|
| `-depth N` | Maximum directory depth to scan, default 1 |
| `--cli` | Print repo summaries as JSON instead of launching the TUI |
| `--fetch` | With `--cli`, git fetch each repo first so ahead/behind compares against the remote |
| `--filter <expr>` | Narrow `--cli` output by a [predicate expression](./filtering.md) |
| `--fresh` | With `--cli`, fetch fresh GitHub PR, CI, and Dependabot data instead of reading the cache |
| `--mani <path>` | Read the repo roster from a mani.yaml instead of scanning directories |
| `--script <path>` | Run `:command` lines headlessly from a file, or `-` for stdin |
| `--version` | Print version information |

## JSON output

```bash
# Cached GitHub data only, so PR and CI fields may be missing
gh repo-dashboard --cli ~/projects

# Invoke gh per repo for current PR, CI, and Dependabot data
gh repo-dashboard --cli --fresh ~/projects

# Only the repos matching a predicate
gh repo-dashboard --cli --filter 'dirty and has_notes' ~/projects

# A full fleet assessment from a mani roster, with remote-accurate counts
gh repo-dashboard --cli --fresh --fetch --mani ~/Developer/mani.yaml
```

### Fields

Local fields (branch, ahead/behind, file counts, stashes, notes, template) cost
no API calls and are always present. `remote` is the `owner/name` the checkout
pushes to and `remote_id` prefixes it with the host, which is what lets another
tool match a directory to a pull request. `worktrees` lists each worktree's path
and branch, so a consumer can find where a branch is already checked out rather
than shelling out to git itself. `--fetch` runs `git fetch` first, so
ahead/behind reflects the remote rather than the last local fetch.

`pr`, `pr_count`, `ci`, and `dependabot_alerts` need the network and appear only
with `--fresh`. Each repo costs one `gh` call for its pull requests, one for the
default branch's CI runs, one for its alerts, and one more per failing workflow
to name the jobs and steps that broke.

```jsonc
{
  "path": "/Users/me/Developer/acme/app",
  "remote": "acme/app",
  "remote_id": "github.com/acme/app",
  "branch": "feature-a",
  "worktrees": [
    {"path": "/Users/me/Developer/acme/app", "branch": "feature-a"},
    {"path": "/Users/me/Developer/acme/app-main", "branch": "main"}
  ],
  "template_src": "gh:KyleKing/calcipy_template",
  "template_version": "5.0.3",
  "template_latest": "5.1.1",
  "template_drift": true,      // behind the latest tag, or pinned to a non-tag
  "ci": {
    "branch": "main",
    "sha": "8553711...",
    "workflows": [
      {
        "workflow": "CI",
        "status": "completed",
        "conclusion": "failure",
        "failing_jobs": ["lint", "test"]
      }
    ]
  },
  "dependabot_alerts": {"high": 2, "low": 1}
}
```

A repo whose alerts endpoint denies access (archived repos, or a token without
the scope) reports no `dependabot_alerts` key rather than failing.

### Roster from mani.yaml

`--mani` reads the project names out of a [mani](https://github.com/alajmo/mani)
roster and uses those directories as the repo set, skipping any that are not
cloned locally. Project paths resolve relative to the roster file, and an
explicit `path:` on a project wins over its name.

## Script files

A script holds one `:command` per line. The leading `:` is optional, and the
runner skips blank lines and `#` comments.

```text
# fetch everything that is behind, then report
:fetch behind
:filter dirty
```

```bash
gh repo-dashboard --script maintenance.txt ~/projects
```

## Commands

The same commands work in the TUI command bar (`:`) and in scripts, with tab
completion on names and arguments: `cleanup`, `fetch`, `filter`, `help`,
`history`, `prune`, `quit`, `refresh`, `select`, and `sort`. A unique prefix
resolves, so `:fil` reaches `:filter`.

`:history` lists the recognized commands you have run, capped at 50, shared
across the command bar, the `@:` repeat key, and script runs.

The test fixtures generate worked examples in [USAGE.md](./USAGE.md).
