package vcs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kyleking/gh-repo-dashboard/internal/cache"
)

// Stamp fingerprints what the checkout at repoPath looks like right now,
// without spawning a subprocess: the OID HEAD resolves to, the current branch
// and the upstream it tracks, that upstream's remote-tracking OID, and the
// mtimes of the ref files a commit, switch, push, or fetch touches. Cheap
// enough to take on every render.
//
// The ref trees it stats are the leaf directories, refs/heads and
// refs/remotes/<remote>, because a directory's mtime tracks only its own
// entries: refs/ never moves when a branch is created.
//
// A jj repo is stamped on its git refs alone, which a colocated repo keeps in
// step with every operation. A workspace with no .git yields cache.NoStamp,
// which proves nothing and so leaves values derived from it uncached rather
// than served stale.
func Stamp(repoPath string) cache.Stamp {
	gitDir, ok := resolveGitDir(repoPath)
	if !ok {
		return cache.NoStamp
	}

	refs := &refResolver{commonDir: commonGitDir(gitDir)}

	head := readHead(gitDir, refs)
	if head.oid == "" && head.branch == "" {
		return cache.NoStamp
	}

	upstream := upstreamRef(refs.commonDir, head.branch)

	remotesDir := filepath.Join(refs.commonDir, "refs", "remotes")
	if upstream.remote != "" {
		remotesDir = filepath.Join(remotesDir, upstream.remote)
	}

	fields := []string{
		head.oid,
		head.branch,
		upstream.ref,
		refs.resolve(upstream.ref),
		strconv.FormatInt(modNanos(filepath.Join(refs.commonDir, "packed-refs")), 10),
		strconv.FormatInt(modNanos(filepath.Join(refs.commonDir, "FETCH_HEAD")), 10),
		strconv.FormatInt(modNanos(filepath.Join(refs.commonDir, "refs", "heads")), 10),
		strconv.FormatInt(modNanos(remotesDir), 10),
	}

	return cache.Stamp{Scope: CheckoutIdentity(repoPath), Fingerprint: strings.Join(fields, "\x00")}
}

// resolveGitDir returns the directory holding repoPath's own HEAD, following
// the "gitdir:" pointer a linked worktree keeps in place of a .git directory.
func resolveGitDir(repoPath string) (string, bool) {
	path := filepath.Join(repoPath, ".git")

	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	if info.IsDir() {
		return path, true
	}

	contents, err := os.ReadFile(path) // #nosec G304 -- path is the repo's own .git pointer
	if err != nil {
		return "", false
	}

	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(contents)), "gitdir:")
	if !ok {
		return "", false
	}

	return filepath.Clean(strings.TrimSpace(gitDir)), true
}

// commonGitDir returns the directory holding the refs gitDir reads, which for
// a linked worktree at <common>/worktrees/<name> is the common directory two
// levels up.
func commonGitDir(gitDir string) string {
	if filepath.Base(filepath.Dir(gitDir)) == "worktrees" {
		return filepath.Dir(filepath.Dir(gitDir))
	}

	return gitDir
}

// headState is what HEAD names: the OID it resolves to, and the branch it
// points at, empty when HEAD is detached.
type headState struct{ oid, branch string }

func readHead(gitDir string, refs *refResolver) headState {
	contents, err := os.ReadFile(filepath.Join(gitDir, "HEAD")) // #nosec G304 -- path is the repo's own HEAD
	if err != nil {
		return headState{}
	}

	line := strings.TrimSpace(string(contents))

	ref, ok := strings.CutPrefix(line, "ref:")
	if !ok {
		return headState{oid: line}
	}

	name := strings.TrimSpace(ref)

	return headState{oid: refs.resolve(name), branch: strings.TrimPrefix(name, "refs/heads/")}
}

// tracking names the remote a branch tracks and the ref that follows it: the
// remote-tracking ref for a real remote, the local branch for a "." remote.
type tracking struct{ remote, ref string }

// upstreamRef returns what branch tracks, zero when it tracks nothing.
func upstreamRef(commonDir, branch string) tracking {
	if branch == "" {
		return tracking{}
	}

	configured := branchConfig(commonDir, branch)
	if configured.remote == "" || configured.ref == "" {
		return tracking{}
	}

	if configured.remote == "." {
		return tracking{ref: configured.ref}
	}

	return tracking{
		remote: configured.remote,
		ref:    "refs/remotes/" + configured.remote + "/" + strings.TrimPrefix(configured.ref, "refs/heads/"),
	}
}

// branchConfig reads the remote and merge ref configured for branch out of the
// repository's own config, ignoring the includes and system-wide files git
// would also consult: an upstream set anywhere but here is vanishingly rare,
// and a wrong answer costs a cache miss rather than a wrong value.
func branchConfig(commonDir, branch string) tracking {
	file, err := os.Open(filepath.Join(commonDir, "config")) // #nosec G304 -- path is the repo's own config
	if err != nil {
		return tracking{}
	}
	defer file.Close() //nolint:errcheck // read-only handle

	var configured tracking

	section := "[branch \"" + branch + "\"]"
	inSection := false
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inSection = line == section

			continue
		}

		if !inSection {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		switch strings.TrimSpace(key) {
		case "remote":
			configured.remote = strings.TrimSpace(value)
		case "merge":
			configured.ref = strings.TrimSpace(value)
		}
	}

	return configured
}

// refResolver reads ref OIDs out of a repository's common directory, loading
// packed-refs at most once.
type refResolver struct {
	commonDir string
	packed    map[string]string
}

func (r *refResolver) resolve(ref string) string {
	if ref == "" {
		return ""
	}

	refPath := filepath.Join(r.commonDir, filepath.FromSlash(ref))

	loose, err := os.ReadFile(refPath) // #nosec G304,G703 -- a ref path under the repo's own git dir
	if err == nil {
		return strings.TrimSpace(string(loose))
	}

	if r.packed == nil {
		r.packed = readPackedRefs(r.commonDir)
	}

	return r.packed[ref]
}

func readPackedRefs(commonDir string) map[string]string {
	packed := make(map[string]string)

	file, err := os.Open(filepath.Join(commonDir, "packed-refs")) // #nosec G304 -- path is the repo's own packed-refs
	if err != nil {
		return packed
	}
	defer file.Close() //nolint:errcheck // read-only handle

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' || line[0] == '^' {
			continue
		}

		oid, ref, ok := strings.Cut(line, " ")
		if ok {
			packed[strings.TrimSpace(ref)] = oid
		}
	}

	return packed
}

func modNanos(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return info.ModTime().UnixNano()
}
