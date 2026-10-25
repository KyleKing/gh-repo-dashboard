package models

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const gitSizeTTL = time.Minute

type gitSizeEntry struct {
	bytes int64
	at    time.Time
}

//nolint:gochecknoglobals // a scan-scoped memo, like the other caches
var gitSizes sync.Map

// GitSize returns the bytes under the repo's git directory: .git, or a
// non-colocated jj repo's backing store. An unreadable directory counts as 0.
// Results are memoized for a minute so one scan walks each repo once.
func GitSize(repoPath string) int64 {
	if entry, ok := gitSizes.Load(repoPath); ok {
		//nolint:forcetypeassert,errcheck // only gitSizeEntry is stored
		if cached := entry.(gitSizeEntry); time.Since(cached.at) < gitSizeTTL {
			return cached.bytes
		}
	}

	size := dirSize(gitDir(repoPath))
	gitSizes.Store(repoPath, gitSizeEntry{bytes: size, at: time.Now()})

	return size
}

func gitDir(repoPath string) string {
	dotGit := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(dotGit); err == nil {
		return dotGit
	}

	return filepath.Join(repoPath, ".jj", "repo", "store", "git")
}

func dirSize(dir string) int64 {
	var size int64
	//nolint:errcheck // a partial walk still gives a usable estimate
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // skip unreadable entries
		}
		if info, infoErr := d.Info(); infoErr == nil {
			size += info.Size()
		}

		return nil
	})

	return size
}
