# Cache identity and invalidation

How caching is keyed and invalidated, and why a timer alone is the wrong answer.
Implementation is `internal/cache` and `internal/vcs/stamp.go`.

Caching used to key on the repo path and expire on a timer, which was wrong in
both directions at once. Six checkouts of one remote each paid for their own `gh
pr list`, and a PR merged thirty seconds ago kept rendering as open for the rest
of the five-minute window, while the branch that carried it had already gone
from the local refs.

The answer is two identity axes, a cheap local stamp, and a per-upstream cache
file. All of it has shipped.

## Two identity axes

A cache key answers "who else may read this value", and the old answer was
always "one directory", which was too narrow for anything read off the remote
and too narrow again for a worktree reading its parent's refs.

| Data | Belongs to | Shared by |
|------|-----------|-----------|
| PR list, PR detail, merged PR heads | the remote | every checkout of that remote |
| Workflow runs, dependabot alerts | the remote | every checkout of that remote |
| Copier latest tag | the template's `_src_path` | every repo generated from it |
| Branch list, commit log | the checkout that asked | nobody |
| Working tree status, stash list | the checkout | nobody |

The upstream key is `host/owner/repo`, lowercased. `vcs.ExtractRepoPath` already
folded the SSH and HTTPS forms together, but it kept the case (GitHub treats
`Owner/Repo` and `owner/repo` as one repository) and dropped the host (a GitHub
Enterprise `acme/tools` would collide with a github.com one). Deriving it costs
nothing new, since the remote URL is already read for `RemoteRepo`.

The checkout key is `vcs.CheckoutIdentity`, which resolves a worktree or jj
workspace to the parent it borrows refs from. Branch and commit caches key on
it, so a worktree and its parent stop fetching the same branch list twice.

Branch lists and commit logs key on the checkout rather than on the object store
the checkout borrows, even though the store is what they read. Both are relative
to whichever HEAD asked (`for-each-ref` carries `%(HEAD)`, `git log` starts at
HEAD), so one shared entry would either hand a worktree its parent's answer or,
with the stamp guarding it, miss every time the two alternate.

## The stamp

A stamp is what a checkout looks like right now, gathered without spawning a
subprocess: the OID HEAD resolves to, the current branch and its upstream's
name, the OID of that upstream's remote-tracking ref, and the mtimes of
`packed-refs`, `FETCH_HEAD`, `refs/heads`, and `refs/remotes/<remote>`. The leaf
ref directories are the ones to stat, because a directory's mtime tracks its own
entries alone, so `refs/` never moves when a branch is created.

That costs six stats and four reads, measured at 54-60 µs per checkout, which is
cheap enough to take on every render and not cheap enough to take per row per
keystroke. Everything in it changes the moment you commit, switch, push, or
fetch.

The stamp cuts both ways, and conflating the two directions is how this gets
built wrong.

For values derived from local state it extends the entry's life. A branch list
whose stamp is unchanged is still correct however old it is, so the TTL stops
being what evicts it, which removes most of the re-fetching a long session used
to do.

For values derived from the remote it only shortens the entry's life. A stamp
cannot prove a PR is still open, because someone else can merge it without
touching your working copy. What it does prove is that a local change happened,
and pushing a branch is exactly the case where the cached PR list is now wrong
and the TTL alone would keep serving it. So remote values keep the TTL as a
ceiling and gain the stamp as an early eviction.

An entry remembers a fingerprint per checkout rather than one for the whole
entry. Six checkouts share one PR list and each has its own stamp, so a single
fingerprint would make every checkout miss on whatever the last one wrote and
thrash the sharing that keying by upstream just bought. A checkout meeting an
entry for the first time is a new reader and hits; one whose own fingerprint
moved since it last touched the entry made a local change and evicts.

A jj colocated repo stamps on git refs alone. Its op log advances on operations
that changed nothing a cache reads, so stamping on it would invalidate far more
than it protects.

## The disk cache

One file per upstream under `os.UserCacheDir()`, holding the remote-derived
values and the stamp each was written under. A cold start on a 12-repo fleet
used to issue 12 `gh pr list` calls, 6 of them the same remote asked six times,
before the first frame settled.

The decisions worth keeping in view:

- The file carries a schema version and is dropped on a mismatch, so a struct
  that gained a field is never read back as zero values
- Writes go to a temp file and rename, so two sessions at once cannot leave a
  half-written file
- Mode 0600, and no bodies. PR titles from a private repo are already more than
  this app had ever put on disk, so comment bodies and descriptions stay in
  memory and `cache_to_disk = false` turns the file off entirely
- Eviction is by last read against a byte budget, rather than growing without
  limit across every repo ever scanned
- Loading is lazy, because reading 60 small files on startup is worse than
  reading the one the cursor is on
- `r` drops the disk cache along with the memory one. Refresh is pressed because
  something looks wrong, and a refresh that leaves a stale PR state on disk
  cannot fix it
