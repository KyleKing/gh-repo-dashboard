package discovery

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Activity selects the repos where one of Authors committed since Since.
type Activity struct {
	Since   time.Time
	Authors []string
}

const activityTimeout = 10 * time.Second

// FilterActive returns the paths of repos a.Authors committed to since a.Since
// and the number it dropped, reading at most workers repos at once. Paths keep
// their input order. A repo with no .git entry (a jj-only repo) cannot be read
// with git log and is kept.
func FilterActive(ctx context.Context, paths []string, a Activity, workers int) ([]string, int) {
	active := make([]bool, len(paths))
	jobs := make(chan int)

	var wg sync.WaitGroup

	for range min(max(workers, 1), max(len(paths), 1)) {
		wg.Go(func() {
			for i := range jobs {
				active[i] = a.committed(ctx, paths[i])
			}
		})
	}

	for i := range paths {
		jobs <- i
	}

	close(jobs)
	wg.Wait()

	kept := make([]string, 0, len(paths))

	for i, path := range paths {
		if active[i] {
			kept = append(kept, path)
		}
	}

	return kept, len(paths) - len(kept)
}

func (a Activity) committed(ctx context.Context, path string) bool {
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return true
	}

	ctx, cancel := context.WithTimeout(ctx, activityTimeout)
	defer cancel()

	//nolint:gosec // #nosec G204 -- fixed binary, the author pattern is regexp-quoted
	out, err := exec.CommandContext(ctx, "git", "-C", path, "log", "--all", "-1", "--format=%H",
		"--extended-regexp", "--regexp-ignore-case",
		"--since="+a.Since.UTC().Format(time.RFC3339), "--author="+authorPattern(a.Authors),
	).Output()

	return err == nil && strings.TrimSpace(string(out)) != ""
}

// authorPattern matches any of emails inside the angle brackets of git's
// "Name <email>" author line, so one address never matches as a substring of another.
func authorPattern(emails []string) string {
	quoted := make([]string, len(emails))
	for i, email := range emails {
		quoted[i] = regexp.QuoteMeta(email)
	}

	return "<(" + strings.Join(quoted, "|") + ")>"
}
