package cache

// RemoteScope returns the key prefix for a value that belongs to the remote,
// so every checkout of one remote reads a single entry. A checkout with no
// resolvable remote falls back to its own path, since an empty identity would
// otherwise pool every remoteless repo into one entry.
func RemoteScope(repoPath, remoteID string) string {
	if remoteID == "" {
		return repoPath
	}

	return remoteID
}
