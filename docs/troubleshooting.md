# Troubleshooting

PR and workflow columns are blank because the `gh` CLI is missing or
unauthenticated. Run `gh auth status`, then `gh auth login` if needed.
Everything else works without `gh`.

Rows for jj repositories show errors because the `jj` CLI is not installed.
Install it, or scan paths without jj repositories. The dashboard reads colocated
repos, holding both `.git` and `.jj`, through jj.

Nothing is listed on launch because with no arguments and no config file the
dashboard scans only the repository enclosing the current directory. Pass parent
directories as arguments (`gh repo-dashboard ~/projects`) or set `scan_paths` in
the [config file](./configuration.md).

PR data looks stale because cached GitHub data lives for `cache_ttl_minutes`,
defaulting to 5. Press `r` to refresh, or pass `--fresh` with `--cli`.
