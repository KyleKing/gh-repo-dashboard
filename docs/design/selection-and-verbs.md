# Selection and verbs: finishing the vim paradigm

The Vision in [ROADMAP.md](../../ROADMAP.md) promises text objects, operators, and
composition. Two of the three shipped. This is the design for the third, and for
the leader key that got stuck carrying both grammars at once.

Status: proposed. Nothing here is implemented.

## Problem

Three symptoms, one cause.

**Nothing selects a row.** `selectedPaths` exists on the model, the repo list draws
a marker for it, and `sr` is a text object meaning "the selected repos". No key
fills it. The only writers are `:select` and the universal find's result set, so
`!fsr` is a sentence the keyboard cannot say. Vim's Visual mode is the missing
half: mark rows, then run an operator over the marks.

**`!` carries two grammars.** Pressing it over the repo list opens a modal verb
menu. Pressing it and then typing `f` starts an operator waiting for a text object.
Those are different things — a menu is a list you read, a grammar is a sentence you
compose — and one key introduces both depending on what you type next.

**`! act` teaches neither.** The footer's job is to name what a key does. "act" is
a category, not a verb, and `!` is not a letter anyone guesses. The verbs behind it
(check out, open in browser, copy URL) are useful and invisible, which is why the
PRs tab grew a second, top-level `o` binding for one of them rather than pointing
at the menu that already had it.

## What vim actually does

Worth being precise, because the analogy is doing real work here.

| vim | today | gap |
| --- | --- | --- |
| `v` enters Visual, motions extend the selection | no selection key | the whole mode |
| `d`, `y`, `>` operate on a selection or a motion | `!` + verb + object | operators only take objects |
| `dap`, `ci"` — operator then text object | `!fdr` | works, and is the good part |
| `5j`, `}` — counted and structural motions | `j`, `k`, `gg`, `G` | no counts |

The composition already works. What is missing is the second way to name a target:
vim lets an operator take either a motion or a visual selection, and the dashboard
only offers the former.

## Options

**Rename the label, keep the keys.** Footer reads `[!] actions`, help spells out the
menu. Costs nothing, breaks nothing, and leaves `!` unguessable and selection
unreachable. This is the floor, not the answer.

**Split the two grammars.** `a` opens the verb menu (`[a]ctions` in the footer's own
bracket notation), `!` stays the operator prefix it already is. Cheap, mnemonic, and
it stops one key meaning two things. Says nothing about selection.

**Add Visual mode and let operators take a selection.** `x` (or `space`) toggles the
row under the cursor, `V` enters a line-wise mode where motions extend the marks, and
any operator run with marks live acts on them instead of prompting for a text object.
`sr` stops being unreachable. The verb menu becomes the discoverable front door to
the same operators the grammar composes, so a user who never learns `!fdr` can still
fetch four marked repos.

The three are cumulative, not exclusive. The third is the one that finishes the
paradigm; the second is the prerequisite that stops `!` from being ambiguous once
operators can be reached two ways.

## Proposed shape

```
x            toggle the row under the cursor
V            line-wise visual: motions extend the marks
esc          clear the marks
a            verb menu for whatever is targeted
!            operator, then a text object (unchanged)
```

The verb menu's title states its target, which is what makes selection legible
without a second indicator:

```
  Repos · 4 selected            a over marked rows
  Repos · dev/bravo             a with nothing marked
  PRs   · #594 ci: Pin GitHub…  a on the PRs tab
```

An operator with marks live skips the object prompt: `!f` fetches the marks rather
than waiting for `dr`. With nothing marked it behaves as it does today. That keeps
one rule — an operator acts on the marks if there are any, otherwise on what you
name — instead of two modes to remember.

## What breaks

`!` no longer opens the verb menu. It is a single key with a year of muscle memory
behind it, and the fix is a one-line status message on the first `!` press that
names `a`, removed a release later. `:select` and the palette keep writing the same
`selectedPaths` map, so both reach the new marks for free.

`x` is unbound today. `V` is unbound. `esc` currently backs out of the expanded
region and the docks, so clearing marks has to sit below those in the same handler
rather than in front of them.

## Open questions

- `x` or `space` for the toggle. `space` is the universal find today (`;` is its
  synonym), so taking it means moving find to `;` alone
- Whether the PRs tab gets marks too, or whether selection stays a repo-list idea
  until a batch verb over pull requests exists to justify it
- Whether `a` should also be the operator prefix, retiring `!` entirely. One key is
  simpler to teach; two keep the menu and the grammar visibly separate
