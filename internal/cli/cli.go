// Package cli implements the non-interactive --cli mode that prints repo
// summaries as JSON instead of launching the TUI.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"time"

	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/github"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
	"github.com/kyleking/gh-repo-dashboard/internal/vcs"
)

const maxConcurrentRepos = 8

// Output is the top-level JSON document printed by --cli mode.
type Output struct {
	GeneratedAt time.Time `json:"generated_at"`
	ScanPaths   []string  `json:"scan_paths"`
	Repos       []Repo    `json:"repos"`
}

// Repo is the stable JSON shape of one repo summary, mirroring the columns of
// the TUI's repo list view.
type Repo struct {
	Path           string         `json:"path"`
	Name           string         `json:"name"`
	VCS            string         `json:"vcs"`
	Branch         string         `json:"branch"`
	Upstream       string         `json:"upstream,omitempty"`
	Ahead          int            `json:"ahead"`
	Behind         int            `json:"behind"`
	Staged         int            `json:"staged"`
	Unstaged       int            `json:"unstaged"`
	Untracked      int            `json:"untracked"`
	Conflicted     int            `json:"conflicted"`
	Dirty          bool           `json:"dirty"`
	Status         string         `json:"status"`
	StashCount     int            `json:"stash_count"`
	WorktreeCount  int            `json:"worktree_count"`
	NotesFile      string         `json:"notes_file,omitempty"`
	NotesFirstLine string         `json:"notes_first_line,omitempty"`
	LastModified   *time.Time     `json:"last_modified,omitempty"`
	PR             *models.PRInfo `json:"pr,omitempty"`
	PRCount        *int           `json:"pr_count,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// githubClient holds the gh-backed fetchers used only when fresh retrieval is
// requested, injectable so tests can assert on cache-only gating.
type githubClient struct {
	prForBranch func(ctx context.Context, repoPath, branch, upstream string) (*models.PRInfo, error)
	prsForRepo  func(ctx context.Context, repoPath, upstream string) ([]models.PRInfo, error)
}

func defaultGitHubClient() githubClient {
	return githubClient{
		prForBranch: github.GetPRForBranch,
		prsForRepo:  github.GetPRsForRepo,
	}
}

// Run discovers repos under scanPaths and writes their summaries as JSON to w.
// GitHub data is served from the cache only, unless fresh is set.
func Run(ctx context.Context, w io.Writer, scanPaths []string, maxDepth int, fresh bool) error {
	paths := discovery.DiscoverRepos(scanPaths, maxDepth)
	out := Output{
		GeneratedAt: time.Now().UTC(),
		ScanPaths:   scanPaths,
		Repos:       collectRepos(ctx, defaultGitHubClient(), paths, fresh),
	}

	return writeOutput(w, out)
}

func collectRepos(ctx context.Context, client githubClient, paths []string, fresh bool) []Repo {
	repos := make([]Repo, len(paths))
	sem := make(chan struct{}, maxConcurrentRepos)

	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			repos[i] = loadRepo(ctx, client, path, fresh)
		}()
	}
	wg.Wait()

	return repos
}

func loadRepo(ctx context.Context, client githubClient, path string, fresh bool) Repo {
	ops := vcs.GetOperations(path)
	summary, err := ops.GetRepoSummary(ctx, path)
	if err != nil {
		return Repo{
			Path:  path,
			Name:  filepath.Base(path),
			VCS:   vcs.DetectVCSType(path).String(),
			Error: err.Error(),
		}
	}

	// Worktrees are a best-effort extra column: a failure just reports zero.
	worktrees, _ := ops.GetWorktreeList(ctx, path) //nolint:errcheck // best-effort, see comment above
	summary.NotesFile, summary.NotesFirstLine = models.DetectNotes(path)
	pr := lookupPR(ctx, client, path, summary.Branch, summary.Upstream, fresh)
	prCount := lookupPRCount(ctx, client, path, summary.Upstream, fresh)

	return newRepo(&summary, len(worktrees), pr, prCount)
}

func newRepo(summary *models.RepoSummary, worktreeCount int, pr *models.PRInfo, prCount *int) Repo {
	repo := Repo{
		Path:           summary.Path,
		Name:           summary.Name(),
		VCS:            summary.VCSType.String(),
		Branch:         summary.Branch,
		Upstream:       summary.Upstream,
		Ahead:          summary.Ahead,
		Behind:         summary.Behind,
		Staged:         summary.Staged,
		Unstaged:       summary.Unstaged,
		Untracked:      summary.Untracked,
		Conflicted:     summary.Conflicted,
		Dirty:          summary.IsDirty(),
		Status:         summary.Status().String(),
		StashCount:     summary.StashCount,
		WorktreeCount:  worktreeCount,
		NotesFile:      summary.NotesFile,
		NotesFirstLine: summary.NotesFirstLine,
		PR:             pr,
		PRCount:        prCount,
	}

	if !summary.LastModified.IsZero() {
		lastModified := summary.LastModified
		repo.LastModified = &lastModified
	}

	return repo
}

// lookupPR returns the pull request for branch from the cache, fetching via gh
// only when fresh is set. A miss (or fetch failure) yields nil.
func lookupPR(ctx context.Context, client githubClient, repoPath, branch, upstream string, fresh bool) *models.PRInfo {
	if upstream == "" {
		return nil
	}

	if cached, ok := github.CachedPRForBranch(branch, upstream); ok {
		return cached
	}
	if !fresh {
		return nil
	}

	pr, err := client.prForBranch(ctx, repoPath, branch, upstream)
	if err != nil {
		return nil
	}

	return pr
}

// lookupPRCount returns the repo's open PR count from the cache, fetching via
// gh only when fresh is set. A miss (or fetch failure) yields nil.
func lookupPRCount(ctx context.Context, client githubClient, repoPath, upstream string, fresh bool) *int {
	if upstream == "" {
		return nil
	}

	if cached, ok := github.CachedPRs(upstream); ok {
		count := len(cached)
		return &count
	}
	if !fresh {
		return nil
	}

	prs, err := client.prsForRepo(ctx, repoPath, upstream)
	if err != nil {
		return nil
	}
	count := len(prs)

	return &count
}

func writeOutput(w io.Writer, out Output) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encoding JSON output: %w", err)
	}

	return nil
}
