# Fleet navigation: peers, PR-to-local map, and PR flows

Design for the navigation and GitHub-intelligence features: knowing which
parallel checkouts hold the same branch, mapping open PRs to local branches,
and PR-wide management inspired by gh-dash but scoped to this tool's flows.
Scope: ROADMAP M16 (peers, map) and M17 (PR flows, CI). Layout primitives come
from [layout-and-density.md](layout-and-density.md).

## Peers panel and jump (M16)

The data already exists (`models.FindPeerCheckouts`, `WorktreeCheckouts`); the
gap is presentation and movement. Today peers surface as a count badge and a
CHECKED OUT column, and there is no way to see the set or move to a member.

The sync scene of the focused view ([focused-repo-view.md](focused-repo-view.md))
renders the peer table; the same table is reachable from the fleet list with
`]` on any repo with peers.

```
 Peers of backup-yak-shears-py   remote: KyleKing/yak-shears-py
  CHECKOUT                    KIND       BRANCH          STATUS      LAST COMMIT
> backup-yak-shears-py        clone      yak-shears-py   +1 ↑5 ↓80   1 month ago   ⚠ same branch
  yak-shears                  clone      yak-shears-py   ✓           2 days ago    ⚠ same branch
  yak-shears-wt-fix           worktree   fix/ci          +2          5 days ago
 enter jump  o open in editor  esc back
```

- Enter re-roots the dashboard on that checkout (pushes it onto the existing
  drill-down stack, so esc returns)
- The `⚠ same branch` flag marks two checkouts on one branch, the state that
  loses local commits. It also propagates upward: the repo list PEERS cell
  renders `⧉2⚠` and the fleet header counts conflicts
  (`⚠ 2 branch conflicts`), so the hazard is visible without drilling
- All of this is local git data: no API cost, no new caches

## PR-to-local map (M16)

A fleet-level view answering three questions in one table: which open PRs have
a local branch, where that branch lives, and which local branches have no PR.
Reached with `:prs` (and a repo-list keybinding once it proves out).

```
 Open PRs across 62 repos          14 open · 3 with local branch · 2 local-only branches
  REPO               PR     TITLE                        CI      REVIEW      ACTIVITY   LOCAL
> mdit-py-plugins    #151   TEST(#143): add category…     ✓ 9/10  —           1w  you    here: kyle/xss-and-attrs
  mdit-py-plugins    #150   FIX: escape user content…     ✓ 11/12 —           1w  cjs    peer: yak-shears
  calcipy            #88    feat: new lint rules         ✗ 2/9   changes     3d  bot    —
  (no PR)            —      local branch fix/texmath     —       —           —          here: fix/texmath-redos
 enter open PR  g checkout locally  o browser  s sections  esc back
```

- LOCAL resolves the PR head ref against every peer checkout's branch list
  (cache-resident), naming the checkout that holds it
- Rows with no PR are local branches with an upstream or commits ahead and no
  matching open PR, the "did I forget to open a PR" case
- ACTIVITY is age of the latest comment or review event plus its author,
  the "waiting on whom" signal
- gh-dash's general lesson applies as sections-as-saved-queries: v1 ships
  fixed sections (needs my review, mine awaiting review, local-only), and a
  config-defined section list only if the fixed three prove insufficient

## PR activity and flows (M17)

Extends PR detail and the review scene rather than adding screens:

- Latest-comment age and author in the PR tab's ACTIVITY column (replacing
  the mostly empty REVIEW width; review state folds into ACTIVITY as a glyph)
- `g` on any PR row checks the PR branch out locally (fetch + switch, gated by
  `ViewModeConfirm`, refused when a peer holds the branch, consistent with `c`)
- Batch refresh of PR data for the filtered set behind the existing `F+obj`
  operator pattern

## CI on the default branch (M17)

`internal/github/workflow.go` already fetches workflow conclusions for the
TUI. Surface it as a CI column (repo list, expanded region, focused
header): `✓`, `✗ lint`, or `…` while loading. The `--cli` fleet-assessment
proposal in ROADMAP.md consumes the same fetch path, so the TUI column and the
JSON field ship from one implementation.

## API budget and performance

Constraints worth designing around, not discovering in production:

- REST is 5,000 requests/hour for an authenticated user, and GraphQL is a
  separate 5,000-point/hour budget. A naive per-branch `gh pr view` across 62
  repos with several branches each burns hundreds of calls per refresh
- Search endpoints have a much lower secondary limit (roughly 30
  requests/minute), so "one search query per repo" patterns are also capped

Rules for every feature in this document:

1. One call per repo, not per branch: `gh pr list --json ...` returns all open
   PRs with head refs in a single call, and the PR-to-local join happens
   locally against cached branch lists
2. Prefer one GraphQL query batching many repos (aliased `repository(...)`
   blocks, roughly 1 point per ~10 repos) for the fleet map and CI columns;
   fall back to per-repo REST when the query fails
3. Everything network-derived goes through the existing TTL cache registry
   with per-kind TTLs (PR list ~5 min, CI ~5 min, activity ~10 min); `r`
   refresh stays the explicit escape hatch
4. Fetch on demand, render from cache: the fleet map fetches for filtered
   repos only, the CI column populates lazily as rows become visible, and
   nothing fetches for repos scrolled past
5. Show staleness instead of hiding it: cells render dim with an age suffix
   (`✓ 2h`) when served from cache older than its TTL refresh point
6. Concurrency stays bounded (reuse the summary loader's worker pattern) so a
   refresh never opens 62 simultaneous gh processes

Local-only features (peers, same-branch conflicts, local-only branch
detection) cost zero API and should never wait on network code paths.
