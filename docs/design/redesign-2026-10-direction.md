# Direction

What gh-repo-dashboard has to become to earn a daily slot again. The screen-level
findings live in [redesign-2026-10-audit.md](redesign-2026-10-audit.md); this file is the product call they feed.

## Why it fell out of use

All four of these, per Kyle (2026-10-25): it shows state but not what needs him,
it is slow and noisy to start, he still leaves it for mani or a shell to act, and
Claude sessions, second-look, and Watch Doggo took over the PR review side.

## The job it keeps

One tool that owns the repo list, answers "what needs me" across the fleet, and
runs the chores across it. Four moments a day:

| Moment | Screen | What it must answer |
|---|---|---|
| Morning triage | Repo list | Which repos need me and why, on the first screen with no keys |
| After parallel agents | Repo list | Which checkouts and worktrees are dirty, unpushed, or sharing a branch |
| Entering a repo | Focused view | Where I left it: PRs, CI, branches, notes, on one surface |
| Fleet maintenance | Tasks | Run a named chore over the repos a predicate picks, serially when it has to be |

The review queue leaves for second-look and the PRs tab goes away. Open PRs
stay visible through the list's PRs column and the focused view's PRs section.

## Decisions (2026-10-25)

- Triage list as mocked in the audit (D1a, D2a, D3a, D4a)
- Focused view as one surface (D5a)
- Ink skin with a Sapphire accent (D6a)
- PRs tab removed (D7c); a Tasks tab takes its place
- Status messages fade at the footer's right end (D8a)
- Tasks live in `config.toml` as `[[tasks]]`, and a task is due by `every`
  per repo since its last run, gated by `where`
- aragonite and the task engine are built in parallel, then the look

## Feature set

### Triage list (replaces the name-sorted table)

A NEEDS column carries every attention signal as colored tokens and is blank for
a quiet repo. Rows split into NEEDS YOU and QUIET groups. Color goes only to what
needs action (audit L1 to L6, D1 to D3). Signals: dirty, unpushed, behind,
failing default-branch CI, a PR whose turn is his, a branch shared with a peer or
worktree, notes, and a task that is due. Template drift and due tasks show but
never promote a row, so chores do not bury real work.

### Tasks (replaces mani)

Tasks are named shell commands in `config.toml`, the same type the built-in
fetch, prune, and cleanup become:

```toml
[[tasks]]
name = "gc"
desc = "aggressive gc for bloated repos"
run = "git -c pack.threads=3 -c pack.windowMemory=1024m gc --prune=now --aggressive"
where = "git_size > 1G"
concurrency = 1        # serial; default runs in parallel
confirm = true
every = "30d"          # due once per repo per 30 days, shown in NEEDS when where also holds

[[tasks]]
name = "copier-update"
run = "copier update --UNSAFE --conflict=rej --defaults && git status --short"
where = "template ~ calcipy"
```

mani's tags map onto predicates the dashboard already reads (`template ~
calcipy` covers `calcipy-template`, a name prefix covers `mdformat-plugin`), so
there is no second repo list or tag list to keep in sync.

The Tasks tab lists every task with its due count and last run, and keeps run
history. `:run gc` (or `enter` on a task) runs it over marks or its `where`; a
run opens as one row per repo with status and duration, `enter` for the full log. The
same task runs headless as `gh repo-dashboard run gc [--where …] [--dry-run]`,
exiting nonzero if any repo failed. Each run writes its log and a last-run stamp
under the cache dir, which is what `every` and a `last_run(gc)` predicate read.

New predicates and fields this needs: `git_size` (a `.git` size column the
fleet sorts by), `last_run(task)`, and `template ~ name`.

### Focused view

One surface with labeled section rules, a PRs section and a CI section ahead of
branches, a fixed split, and a header line carrying the same NEEDS tokens (audit
R1 to R7, D5). A one-shot shell into the repo (suspend, subshell, restore) moves
off the ROADMAP's deferred list because "can't act from it" was one of the four
reasons.

### Hand-offs to other tools

Decided 2026-10-25. The protocol is "argv in, exit back": the dashboard suspends,
hands the terminal to another tool started with a target on its argv, and
re-reads that repo when the tool exits. A hand-off is a task with
`interactive = true`, run for the one row under the cursor, with `{path}`,
`{repo}`, `{pr}`, and `{branch}` filled in shell-quoted:

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

They show in the `a` verbs menu only where their placeholders have values, so
`review` appears on a PR and not on a repo with none. The one-shot shell into a
repo is the built-in `$SHELL` hand-off. The reverse direction needs nothing from
the dashboard: `gh-repo-dashboard <path>` already opens that repo's focused
view, so second-look only needs a key that hands off to it. The suspend, run,
and return mechanics live in aragonite so both tools share them.

### Fast start

Measured 2026-10-25 on 52 repos with `--cli -depth 1`: local summaries take
2.2s (5.5s of it system time, from per-repo git subprocesses), and `--fresh`
GitHub PR and CI data takes 31.6s. PR and CI answers already persist to disk,
but only within the TTL, so a morning start shows a spinner on every row until
the fan-out lands. The fix is stale-while-revalidate: render the last persisted
answer at once whatever its age, mark it stale quietly, and refresh behind it.
That is a read in aragonite's `cache` (a `PersistedStale` beside `Persisted`).
Ship a `scan_paths` config so `gh repo-dashboard` opens the fleet from anywhere
(Kyle has no config file today).

## Shared work

| Where | Change | Who benefits |
|---|---|---|
| aragonite | `keyhint.Fit` (priority footer, dimmed inapplicable keys), `tui/frame` (header, tabs, body, footer, status slot), `table.Row` (gapless selection), `region` height as a ceiling | gh-repo-dashboard now, second-look next, which deletes its own footer engine |
| my_go_template | `demo:stills` task (VHS `Wait` before each `Screenshot`, fixture fleet), orphan-golden check | Every Go TUI rendered from the template |
| gh-repo-dashboard | aragonite v0.20.0 → latest, Ink skin, everything above | |

## Status

Shipped 2026-10-25 (gh-repo-dashboard v1.12 through v1.16, aragonite v0.23 and
v0.24, my_go_template v0.16): everything above, mani's tasks ported into
`config.toml`, and stills recorded against a fixture fleet with
`mise run demo:stills`.

Left:

- second-look adopts `keyhint.Fit`, `tui/frame`, and `tui/handoff`, and gains a
  key that hands off to `gh-repo-dashboard <path>`
- my_go_template: `test:golden-orphans` honors a golden build tag, the golden
  tasks become template content, and `gci.sections` renders multi-line
- Retire `mani.yaml` and `mani_manager.py` once the Tasks tab has carried a few
  maintenance sweeps

## Parked

Cloning repos listed in config but missing on disk (mani's `projects.url`),
Claude session badges, watch mode.
