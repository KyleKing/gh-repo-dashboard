# Selection and verbs: finishing the vim paradigm

The Vision in [ROADMAP.md](../../ROADMAP.md) promises text objects, operators, and
composition. Two of the three shipped. This is the design for the third, and for
the leader key that ended up the only door to both.

Status: shipped. What the build settled records the calls made along the way.

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

## Shape

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

The menu's batch verbs act on the marks when there are marks, and ask for a text
object when there are none, which is the discoverable path. The `!` grammar always
asks for its object, because `sr` already names the marked repos: consuming them
before the object would retire a text object rather than fill one. `x` is what
finally fills `sr`, so `!fsr` became a sentence the keyboard can say.

## What broke

`!` no longer opens the verb menu; `a` does. `!` became the direct operator prefix
the help text always claimed it was, so `!fdr` now works without going through the
menu first.

Ordering matters and is load-bearing. `ar` is a text object and `a` is the menu
leader, so an operator waiting for its object has to be dispatched before the
leader keys. `handleGrammarKey` is that order, and reversing two of its cases
breaks `!par` in a way no type checks.

`esc` clears the marks ahead of closing the expanded region, since a range the
user just drew is the nearest layer to back out of.

`:select` and the palette still write `selectedPaths`, so both fill the same marks.
The word in the interface is "marked" everywhere: the badge, the footer, the menu
title, and the `sr` object's own name. `:select` keeps its name as a command verb.

## What the build settled

**`x` for the toggle.** In a vim-paradigm TUI, `t` (till), `c` (change), `m`
(mark), `d`, `y`, `w`, `e`, and `b` are all motions or operators worth keeping
free. `x` is delete-char, which a read-mostly dashboard will never want, so it is
the letter with the least future value.

**Marking steps past the row.** One key held down marks a run, the way ranger and
lf behave, rather than alternating `x` and `j`.

**The range shows over the marks rather than merging into them.** Leaving visual
mode restores whatever was marked before it opened, so `V` can never silently eat
a set built with `x`.

## Still open

- Whether the PRs tab gets marks. It stays a repo-list idea until a batch verb over
  pull requests exists to justify it
- `x` also clears the predicate in the filter dock. Different mode, no collision
  today, but two meanings for one letter is worth revisiting
- Counted motions (`5j`) and structural ones (`}`), which the table above lists as
  a gap and this change did not close
