# gh-repo-dashboard TUI critique

Produced with the `tui-critique` skill (`~/.claude/skills/tui-critique/`), which
pairs `hyperskills:tui-design`'s TUI vocabulary with impeccable's critique
structure, adapted for terminals.

**Method**: binary built from `e653d1b`, run over `~/Developer/kyleking` (63 real
repos, `--depth 2`). Captured repo list, filter modal, sort modal, search, command
mode, repo detail, help overlay, `NO_COLOR=1`, and an 80-column-equivalent narrow
terminal, via VHS `Screenshot`. Findings cross-checked against
`internal/app/view_repolist.go`, `internal/app/view.go`, and `internal/config/`.

## Design Specificity Verdict

Authored for this domain, not generic. The status-code vocabulary
(`+1 ~10 ?2 ↑1N`), the filter/sort modals keyed to git-specific facets
(Dirty/Ahead/Behind/Has Stash), and the vim operator+text-object batch pattern
(`F` + `ar`) wouldn't transplant to an unrelated list-of-rows tool. This is the
part working best.

## Health Score

| # | Heuristic | Score | Key finding |
|---|-----------|-------|-------------|
| 1 | Visibility of System Status | 3 | Loading/dirty/PR counts always visible in the breadcrumb; no per-row loading indicator visible in this capture |
| 2 | Match System/Real World | 4 | Git/jj terminology throughout, no leaked internal types |
| 3 | User Control and Freedom | 3 | Esc backs out everywhere tested; command-mode has no visible cancel hint |
| 4 | Consistency and Standards | 1 | Status column overflows its fixed width and desyncs every column to its right |
| 5 | Error Prevention | n/a this pass | destructive batch ops (`C` cleanup) weren't exercised |
| 6 | Recognition Rather Than Recall | 2 | Command mode (`:`) is a bare prompt with no hint visible and isn't listed on the `?` help screen at all |
| 7 | Flexibility and Efficiency | 4 | Vim motions, batch operators, command history all present |
| 8 | Aesthetic and Minimalist Design | 2 | Clean at rest, but the column desync reads as broken, not minimal |
| 9 | Error Recovery | n/a this pass | no error state captured |
| 10 | Terminal Portability | 1 | `NO_COLOR=1` produces zero visible difference from full color |

**24/28** applicable (heuristics 5 and 9 n/a this pass). Good, dragged down hard by
two P1s.

## Priority Issues

**[P1] Status column overflows and desyncs every row after it.**
`statusColWidth` is a hardcoded `12` (`internal/app/view.go:25`), and
`renderTableRow` pads with `fmt.Sprintf("%-*s", statusTextWidth, status)`
(`view_repolist.go:332`), which pads a short status but never truncates a long
one. Name and branch cells do call `truncate()` (`view_repolist.go:282-283`);
status does not, so it's the one column in the row without the safety net its
neighbors already have. Any repo whose status string exceeds 12 characters
(`+1 ~10 ?2 ↑1N` is 14) pushes PR/PRs/Template/Modified out of alignment for that
row only, seen on `gh-lazydispatch`, `gh-repo-dashboard`, and `gh-star-search`.
Fix: truncate with an ellipsis at `statusColWidth`, or size the column from the
actual max status width the way name/branch effectively are.

**[P1] `NO_COLOR` is not implemented.** No reference to `NO_COLOR` anywhere in
`internal/`. `NO_COLOR=1` output is pixel-identical in coloring to the default
run. `tui-design`'s anti-pattern #7 and DESIGN.md's own "minimal color"
philosophy both point the same direction; right now dirty/ahead/behind status is
color-only in practice since there's no fallback path to verify against.

**[P2] Command mode has no discoverability.** A bare `:` prompt shows no
completion list, no argument hint, and no footer change. The `?` help screen
documents Navigation, Filtering & Sorting, Batch Actions, and General, but never
mentions `:` command mode, `@:` repeat, or text objects, despite
`command.go`/`textobject.go` existing as first-class systems. A first-timer has
no path to discovering this.

**[P3] Narrow-terminal layout has no responsive strategy.** At a narrow width the
table silently clips PRs/Template/Modified columns and part of the footer's own
keybinding hints (`? help q quit` gets cut off), with no truncation indicator or
minimum-size gate. `tui-design`'s Responsive Terminal Design section names three
fixes (priority collapse, breakpoint mode, minimum-size gate); none appear
implemented.

## Persona Red Flags

**Alex (power user)**: mostly satisfied, vim motions and batch operators are
there. One gap: command mode's lack of an autocomplete overlay (tab-completion
exists functionally per the project's own demo tape, but nothing in the UI
signals it before you try).

**Jordan (first-timer)**: would never find `:` command mode from the help
screen. Status codes (`+1 ~10 ?2 ↑1N`) have no legend outside the filter modal's
spelled-out labels (Dirty/Ahead/Behind); a first-timer reading the repo list cold
is decoding shorthand with no key.

**Sam (portability-dependent)**: `NO_COLOR` is a straight fail. Nothing in this
capture was unusable in a screen-reader sense (text-based, no icon-only
controls), but color is currently load-bearing for status meaning with no
honored opt-out.

## Specialist Reviewer Analysis

### Tone (Refine: animate, bolder, colorize, delight, layout, overdrive, quieter, typeset)

No motion of any kind is implemented. There's no spinner package in use; the
loading state is the static string `"Discovering repositories..."`
(`view_repolist.go:135`), and progress reporting is a plain
`"Loading %d/%d"` badge in the breadcrumb, not an animated indicator. Batch
progress (`renderBatchProgress`, `view_modals.go:312`) renders a gauge with
static `█`/`░` fill characters and no animation between frames. Under `animate`'s
standard ("purposeful motion that conveys state, not decoration"), this app has
made the maximally conservative choice: zero motion, so zero risk of decorative
noise. That's defensible for a data tool but it's also the whole reason `delight`
has nothing to point at. There isn't a single moment (a completion flash, a
satisfying fill animation, a spinner with personality) that turns "functional"
into "memorable." Visual weight is well calibrated at rest (badges pop, body text
recedes), so `bolder`/`quieter` have nothing to push. `colorize` and `typeset`
land on the same two issues already flagged above: color isn't gracefully
degrading (`NO_COLOR` fail) and column rhythm breaks under the status-overflow
bug. No net-new finding beyond what's already P1/P2, but confirms the visual
craft is otherwise close to a polished baseline.

### Reeve (Simplify: adapt, clarify, distill)

`adapt` fails the way the narrow-terminal finding already describes: no
priority-collapse or breakpoint mode, just silent clipping including the
footer's own hints. `distill` and `clarify` surface one thing the health-score
pass didn't: the status column is the *one* place truncation was skipped while
every sibling column (name, branch, even the copier/template cell via
`copierColWidth-2`) got it. That's not just a P1 bug, it's an inconsistency in
how ruthlessly the codebase already applies "cut what doesn't fit" everywhere
else. On copy: the empty state ("No repositories found") and the loading state
("Discovering repositories...") are both terse and honest, no filler, which is
exactly what `clarify` asks for. Nothing to cut on the chrome side either, the
borders and badges you see are all load-bearing; there's no decoration here that
`distill` would flag for removal.

### Priya (Harden: harden, onboard, optimize, polish)

Edge cases: the empty state exists (`"No repositories found"`) but offers no next
step, no mention of `--depth`, no hint to check the scanned path, nothing
actionable if a user's real repos didn't turn up. That's an `onboard` gap more
than a `harden` one: the state is handled, but it doesn't show a path to value.
There's no dedicated first-run experience beyond that; config is optional TOML
that silently falls back to flag defaults when absent
(`internal/config/config.go`), which is the right minimal behavior but means a
brand-new user with zero config gets the same screen as everyone else with no
"here's what to do" framing. `optimize`: progressive loading is architecturally
sound (placeholders now, incremental fill via Tea messages per DESIGN.md), so
perceived performance should be fine; not independently verified against a slow
disk or huge repo count in this pass. `polish`: the status-column desync is
exactly the kind of "meticulous final pass" defect this command exists to catch,
everything else captured (filter modal, sort modal, help overlay, repo detail)
reads as finished, no stray debug text, no obvious misalignment outside that one
bug.

## Action Items

1. **P1** Fix status-column truncation. Either call the existing `truncate()`
   helper on `status` in `renderTableRow` (`view_repolist.go:332`) the same way
   name/branch already do, or widen `statusColWidth` to the true max and drop the
   fixed constant.
2. **P1** Respect `NO_COLOR`. Wire lipgloss/termenv's color-profile detection (or
   check `os.Getenv("NO_COLOR")` directly) and confirm a monochrome pass still
   distinguishes dirty/ahead/behind status through non-color means (the
   `+`/`↑`/`↓`/`?` glyphs already carry meaning, so this may mostly be a
   lipgloss configuration fix rather than a redesign).
3. **P2** Document `:` command mode, `@:` repeat, and text objects on the `?`
   help screen; add a footer hint when the command bar is open.
4. **P2 → onboard** Give the empty state a next step ("no repos found under
   `<path>` at depth `<n>` — try `--depth` or check the scan path") instead of a
   bare "No repositories found."
5. **P3** Add a responsive strategy for narrow terminals: priority-collapse the
   PR/PRs/Template/Modified columns before the footer's own keybinding hints get
   clipped, or gate below a minimum width with an explicit message.
6. **P3 (optional, Tone/delight)** Consider one deliberate motion moment (a
   spinner during "Discovering repositories...", a brief flash on batch-task
   completion) now that the rest of the visual system is calibrated enough to
   support it without it reading as noise.

## Open Questions

1. The status-column overflow and `NO_COLOR` gap are both concrete, code-located
   fixes (#1 and #2 above). Want both addressed, or is one lower priority right
   now?
2. Should command-mode discoverability get a footer hint, a line in the `?` help
   screen, or both?
3. Is a narrow-terminal minimum-size gate in scope, or is 80x24 support not a
   goal for this tool? DESIGN.md doesn't state a minimum either way.
4. Is the current zero-motion design intentional (data-tool restraint) or just
   unaddressed? Worth deciding before treating item #6 as a real backlog item
   versus discarding it.

## Skill Notes

This run added two things to `tui-critique` based on gaps found while using it:
a requirement to verify causal claims against source before stating them (used
here for the `statusColWidth`/`truncate()` findings), and three specialist
reviewer lenses (Tone, Reeve, Priya) mapped to impeccable's Refine/Simplify/
Harden command groups for when a craft-level pass is wanted beyond the five
usability personas.
