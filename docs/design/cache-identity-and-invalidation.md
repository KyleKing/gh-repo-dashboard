# Cache identity and invalidation

How caching is keyed and invalidated, and why a timer alone is the wrong
answer.

Caching was keyed by repo path and expired on a timer, which was wrong in both
directions at once. Six checkouts of one remote each paid for their own `gh pr
list`, and a PR merged thirty seconds ago kept rendering as open for the rest
of the five-minute window while the branch that carried it had already gone
from the local refs.

The answer is two identity axes and a cheap local stamp, plus a per-upstream
cache file so a cold start on a large fleet does not re-issue one network call
per repo. All three steps below have shipped.

## What is shared with what

A cache key answers "who else may read this value". Today the answer is always
"one directory", which is too narrow for anything read off the remote and too
narrow again for a worktree reading its parent's refs.

| Data | Belongs to | Shared by |
|------|-----------|-----------|
| PR list, PR detail, merged PR heads | the remote | every checkout of that remote |
| Workflow runs, dependabot alerts | the remote | every checkout of that remote |
| Copier latest tag | the template's `_src_path` | every repo generated from it |
| Branch list, commit log | the checkout that asked | nobody (see Resolved) |
| Working tree status, stash list | the checkout | nobody |

`CopierLatestTagCache` already keys on the upstream rather than the path, and
it is the only one that does. This generalizes that.

### Upstream identity

`vcs.ExtractRepoPath` already folds `git@github.com:Owner/Repo.git` and
`https://github.com/Owner/Repo` onto `Owner/Repo`. Two gaps: it keeps the
case, and GitHub treats `Owner/Repo` and `owner/repo` as one repository, and
it drops the host, so a GitHub Enterprise `acme/tools` would collide with a
github.com `acme/tools`.

The upstream key becomes `host/owner/repo`, lowercased. Deriving it costs
nothing new because the remote URL is already read for `RemoteRepo`.

### Checkout identity

`vcs.CheckoutIdentity` already resolves a worktree or jj workspace to the
parent it borrows refs from, at the cost of one `os.Stat` and a small file
read. Branch and commit caches key on it, so a worktree and its parent stop
fetching the same branch list twice.

## The stamp

A stamp is what the checkout looks like right now, gathered without spawning a
subprocess:

- the OID `HEAD` resolves to, read through `.git/HEAD` and the ref file it
  names, falling back to `packed-refs`
- the current branch's name and its upstream's name, which only `.git/config`
  records
- the OID of the remote-tracking ref for that upstream
- the mtime of `packed-refs`, `FETCH_HEAD`, `refs/heads`, and the upstream's
  `refs/remotes/<remote>`

The leaf ref directories are the ones to stat. A directory's mtime tracks its
own entries alone, so `refs/` never moves when a branch is created.

Cost is six stats and four reads, measured at 54-60 µs per checkout, which is
cheap enough to take on every render and not cheap enough to take per row per
keystroke. Everything in it changes the moment you commit, switch, push, or
fetch.

The stamp cuts both ways, and conflating the two directions is how this gets
built wrong:

**For values derived from local state it extends the entry's life.** A branch
list whose stamp is unchanged is still correct however old it is, so the TTL
stops being the thing that evicts it. That removes most of the re-fetching a
long session does today.

**For values derived from the remote it only shortens the entry's life.** A
stamp cannot prove a PR is still open, because someone else can merge it
without touching your working copy. What it does prove is that a local change
happened, and pushing a branch is exactly the case where the cached PR list is
now wrong and TTL alone keeps serving it for another five minutes. So remote
values keep their TTL as a ceiling and gain the stamp as an early eviction.

Concretely, `Get` takes the current stamp and an entry stores the stamp it was
written under:

```go
value, ok := cache.PRListCache.Get(upstream, stamp)   // stale if stamp differs
value, ok := cache.BranchCache.Fresh(identity, stamp) // ignores the TTL
```

A stamp carries the checkout it came from, and an entry remembers a
fingerprint per checkout rather than one for the whole entry. Six checkouts
share one PR list and each has its own stamp, so a single fingerprint would
make every checkout miss on the entry the last one wrote and thrash the
sharing that keying by upstream just bought. A checkout meeting an entry for
the first time is a new reader and hits. A checkout whose own fingerprint
moved since it last touched the entry made a local change and evicts.

## The cache file

One file per upstream under `os.UserCacheDir()/gh-repo-dashboard/`, holding
the remote-derived values and the stamp each was written under. A cold start
on the 12-repo fleet in `coverbasedev` currently issues 12 `gh pr list` calls,
6 of which are the same remote asked 6 times, before the first frame settles.

Decisions this needs:

- **Schema version in the file, and drop the file on a mismatch.** A struct
  that gained a field must not be read back as a zero value.
- **Write atomically**, to a temp file and rename, so two sessions running at
  once cannot leave a half-written file.
- **Mode 0600 and no bodies.** PR titles from a private repo are already more
  than the app has ever put on disk. Comment bodies and descriptions stay in
  memory. This is worth deciding deliberately rather than inheriting from
  whatever is convenient, and a `cache_to_disk = false` opt-out belongs in
  `config.toml` next to `cache_ttl_minutes`.
- **Bound the size**, and evict by last-read date rather than growing without
  limit across every repo ever scanned.
- **Load lazily.** Reading 60 small files on startup is worse than reading the
  one the cursor is on.

## Resolved

- A jj colocated repo stamps on git refs alone. The op log advances on
  operations that changed nothing a cache reads, so stamping on it would
  invalidate far more than it protects.
- `r` drops the disk cache along with the memory one. Refresh is pressed
  because something looks wrong, and a refresh that leaves a stale PR state on
  disk cannot fix it.
- The stamp lives in `internal/vcs` beside `CheckoutIdentity`, which already
  reads repository state without spawning a subprocess. `internal/cache` takes
  it as an opaque comparable value and never learns how it is gathered.
- Disk holds counts, states, numbers, and PR titles at mode 0600. Bodies and
  comments stay in memory, and `cache_to_disk = false` turns the file off.
- The branch list and the commit log key on the checkout path, not on the
  object store the checkout borrows. Both are read relative to whichever HEAD
  asked (`for-each-ref` carries the `%(HEAD)` marker, `git log` starts at
  HEAD), so one shared entry either hands a worktree its parent's answer or,
  with the stamp guarding it, misses every time the two alternate. Keying on
  the checkout caches both, and the sharing this row promised was never
  available to values that differ per checkout.

## Order of work

1. Upstream and checkout identity keys, with the existing TTL untouched. This
   alone removes the duplicate fetching across parallel checkouts, and it is
   the part the list redesign needs.
2. Stamps, extending local-derived entries and evicting remote-derived ones
   early.
3. The disk cache.

The list redesign rode on step 1. Its expanded region reads branches and open
pull requests for the repo under the cursor through the `:prs` fleet map's
per-repo path, so the upstream key is what keeps `j` held down from costing one
API call per row.
