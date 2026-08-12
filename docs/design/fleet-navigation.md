# Fleet navigation: peers, the PR-to-local map, and the API budget

Why the fleet-wide views exist and what constrains them. The keys and columns
are in [interface.md](../interface.md); layout primitives come from
[layout-and-density.md](layout-and-density.md).

## Peers

The data was already there (`models.FindPeerCheckouts`, `WorktreeCheckouts`) and
only surfaced as a count badge, so the gap was presentation and movement rather
than fetching. The peer table is reachable from both the focused view and the
fleet list, and `enter` re-roots the dashboard on a peer by pushing it onto the
existing drill-down stack, so `esc` returns.

Two checkouts on one branch is the state that loses local commits, so it is
flagged as `⚠ same branch` and propagates upward: the repo list's PEERS cell
renders `⧉2⚠` and the fleet header counts the conflicts. The hazard is visible
without drilling in.

All of this is local git data, so it costs no API calls and never waits on
network code paths.

## PR-to-local map

One fleet-level table (`:prs`) answering three questions at once: which open PRs
have a local branch, where that branch lives, and which local branches have no
PR. LOCAL resolves each PR's head ref against every peer checkout's cached
branch list and names the checkout holding it. Rows with no PR are local
branches with an upstream or commits ahead and no matching open PR, which is the
"did I forget to open a PR" case.

ACTIVITY carries the age of the latest comment or review event plus its author,
because "waiting on whom" is the signal a state column cannot give. gh-dash's
lesson applies as sections-as-saved-queries, and v1 ships fixed sections rather
than a config-defined list, on the theory that a config surface should wait
until the fixed ones prove insufficient.

## API budget

Constraints worth designing around rather than discovering in production. REST
allows 5,000 requests/hour per authenticated user and GraphQL a separate
5,000-point/hour budget, so a naive per-branch `gh pr view` across 62 repos with
several branches each burns hundreds of calls per refresh. Search endpoints have
a much lower secondary limit of roughly 30 requests/minute, so "one search query
per repo" is capped too.

Rules for every feature here:

1. One call per repo, never one per branch. `gh pr list --json` returns all open
   PRs with head refs at once, and the PR-to-local join happens locally against
   cached branch lists
2. Prefer one GraphQL query batching many repos (aliased `repository(...)`
   blocks, roughly 1 point per 10 repos), falling back to per-repo REST when the
   query fails
3. Everything network-derived goes through the cache registry with per-kind
   TTLs, and `r` stays the explicit escape hatch
4. Fetch on demand and render from cache: the fleet map fetches only for
   filtered repos, the CI column populates lazily as rows become visible, and
   nothing fetches for repos scrolled past
5. Show staleness rather than hiding it, so a cell served from cache past its
   TTL refresh point renders dim with an age suffix
6. Concurrency stays bounded through the summary loader's worker pattern, so a
   refresh never opens 62 simultaneous gh processes
