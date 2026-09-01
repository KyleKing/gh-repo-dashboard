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

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/forge/github"
	"github.com/kyleking/aragonite/vcs"

	"github.com/kyleking/gh-repo-dashboard/internal/copier"
	"github.com/kyleking/gh-repo-dashboard/internal/discovery"
	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
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
	Path          string             `json:"path"`
	Name          string             `json:"name"`
	VCS           string             `json:"vcs"`
	Branch        string             `json:"branch"`
	Upstream      string             `json:"upstream,omitempty"`
	Remote        string             `json:"remote,omitempty"`
	RemoteID      string             `json:"remote_id,omitempty"`
	Ahead         int                `json:"ahead"`
	Behind        int                `json:"behind"`
	Staged        int                `json:"staged"`
	Unstaged      int                `json:"unstaged"`
	Untracked     int                `json:"untracked"`
	Conflicted    int                `json:"conflicted"`
	Dirty         bool               `json:"dirty"`
	Status        string             `json:"status"`
	StashCount    int                `json:"stash_count"`
	WorktreeCount int                `json:"worktree_count"`
	Worktrees     []Worktree         `json:"worktrees,omitempty"`
	NotesFiles    []models.NoteFile  `json:"notes_files,omitempty"`
	LastModified  *time.Time         `json:"last_modified,omitempty"`
	PR            *forge.PullRequest `json:"pr,omitempty"`
	PRCount       *int               `json:"pr_count,omitempty"`

	TemplateSrc      string                 `json:"template_src,omitempty"`
	TemplateVersion  string                 `json:"template_version,omitempty"`
	TemplateLatest   string                 `json:"template_latest,omitempty"`
	TemplateDrift    bool                   `json:"template_drift"`
	CI               *forge.DefaultBranchCI `json:"ci,omitempty"`
	DependabotAlerts map[string]int         `json:"dependabot_alerts,omitempty"`

	Error string `json:"error,omitempty"`
}

// Worktree is one checkout of a repo, naming where a branch already lives.
type Worktree struct {
	Path   string `json:"path"`
	Branch string `json:"branch,omitempty"`
	Locked bool   `json:"locked,omitempty"`
}

// githubClient holds the gh-backed fetchers used only when fresh retrieval is
// requested, injectable so tests can assert on cache-only gating.
type githubClient struct {
	prForBranch func(ctx context.Context, repoPath, remoteID, branch, upstream string) (*forge.PullRequest, error)
	prsForRepo  func(ctx context.Context, repoPath, remoteID, upstream string) ([]forge.PullRequest, error)
	defaultCI   func(ctx context.Context, repoPath, remoteID string) (*forge.DefaultBranchCI, error)
	alerts      func(ctx context.Context, repoPath, remoteRepo string) map[string]int
}

func defaultGitHubClient() githubClient {
	return githubClient{
		prForBranch: github.GetPRForBranch,
		prsForRepo:  github.GetPRsForRepo,
		defaultCI:   github.GetDefaultBranchCI,
		alerts:      github.DependabotAlerts,
	}
}

// Options configures one --cli run.
//
// Fresh permits network reads (pull requests, CI); without it the output is
// whatever the cache already holds, which with `cache_to_disk` on survives a
// cold start for every GitHub-derived field. Fetch runs a git fetch first,
// so ahead/behind counts compare against the remote rather than the last
// local fetch.
type Options struct {
	MaxDepth  int
	Fresh     bool
	Fetch     bool
	Predicate string
}

// Run discovers repos under scanPaths and writes their summaries as JSON to w.
func Run(ctx context.Context, w io.Writer, scanPaths []string, opts Options) error {
	var pred filters.Predicate[models.RepoSummary]
	if opts.Predicate != "" {
		parsed, err := filters.ParsePredicate(opts.Predicate, filters.RepoAtoms())
		if err != nil {
			return fmt.Errorf("invalid --filter predicate: %w", err)
		}
		pred = parsed
	}

	paths := discovery.DiscoverRepos(scanPaths, opts.MaxDepth)
	out := Output{
		GeneratedAt: time.Now().UTC(),
		ScanPaths:   scanPaths,
		Repos:       collectRepos(ctx, defaultGitHubClient(), paths, opts, pred),
	}

	return writeOutput(w, out)
}

func collectRepos(
	ctx context.Context, client githubClient, paths []string, opts Options, pred filters.Predicate[models.RepoSummary],
) []Repo {
	repos := make([]*Repo, len(paths))
	sem := make(chan struct{}, maxConcurrentRepos)

	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			repos[i] = loadRepo(ctx, client, path, opts, pred)
		}()
	}
	wg.Wait()

	kept := make([]Repo, 0, len(repos))
	for _, repo := range repos {
		if repo != nil {
			kept = append(kept, *repo)
		}
	}

	return kept
}

// loadRepo builds the Repo for path, or nil when pred is set and the repo's
// summary doesn't match it.
func loadRepo(
	ctx context.Context, client githubClient, path string, opts Options, pred filters.Predicate[models.RepoSummary],
) *Repo {
	ops := vcs.GetOperations(path)

	if opts.Fetch {
		if mutator, ok := ops.(vcs.Mutator); ok {
			//nolint:errcheck // a repo with no reachable remote still reports its local state
			_, _, _ = mutator.FetchAll(ctx, path)
		}
	}

	summary, err := models.ReadSummary(ctx, ops, path)
	if err != nil {
		if pred != nil && !pred(summary) {
			return nil
		}

		return &Repo{
			Path:  path,
			Name:  filepath.Base(path),
			VCS:   summary.VCSType.String(),
			Error: err.Error(),
		}
	}

	// Worktrees are a best-effort extra column: a failure just reports zero.
	worktrees, _ := ops.GetWorktreeList(ctx, path) //nolint:errcheck // best-effort, see comment above
	//nolint:errcheck // absence just leaves the template fields empty
	summary.TemplateInfo, _ = copier.GetTemplateInfo(ctx, path)
	pr := lookupPR(ctx, client, path, summary.RemoteID, summary.Branch, summary.Upstream, opts.Fresh)
	prCount := lookupPRCount(ctx, client, path, summary.RemoteID, summary.Upstream, opts.Fresh)
	summary.PRInfo = pr

	if pred != nil && !pred(summary) {
		return nil
	}

	repo := newRepo(&summary, worktrees, pr, prCount)
	repo.CI = lookupCI(ctx, client, path, summary.RemoteID, opts.Fresh)
	if opts.Fresh && client.alerts != nil {
		repo.DependabotAlerts = client.alerts(ctx, path, summary.RemoteRepo)
	}

	return &repo
}

// lookupCI returns the default branch's CI from the cache, fetching via gh only
// when fresh is set. A miss (or fetch failure) yields nil.
func lookupCI(
	ctx context.Context, client githubClient, repoPath, remoteID string, fresh bool,
) *forge.DefaultBranchCI {
	if cached, ok := github.CachedDefaultBranchCI(ctx, repoPath, remoteID); ok {
		return cached
	}
	if !fresh || client.defaultCI == nil {
		return nil
	}

	ci, err := client.defaultCI(ctx, repoPath, remoteID)
	if err != nil {
		return nil
	}

	return ci
}

func newRepo(
	summary *models.RepoSummary, worktrees []vcs.WorktreeInfo, pr *forge.PullRequest, prCount *int,
) Repo {
	repo := Repo{
		Path:          summary.Path,
		Name:          summary.Name(),
		VCS:           summary.VCSType.String(),
		Branch:        summary.Branch,
		Upstream:      summary.Upstream,
		Remote:        summary.RemoteRepo,
		RemoteID:      summary.RemoteID,
		Ahead:         summary.Ahead,
		Behind:        summary.Behind,
		Staged:        summary.Staged,
		Unstaged:      summary.Unstaged,
		Untracked:     summary.Untracked,
		Conflicted:    summary.Conflicted,
		Dirty:         summary.IsDirty(),
		Status:        summary.Status().String(),
		StashCount:    summary.StashCount,
		WorktreeCount: len(worktrees),
		Worktrees:     worktreeList(worktrees),
		NotesFiles:    summary.NotesFiles,
		PR:            pr,
		PRCount:       prCount,
	}

	if info := summary.TemplateInfo; info != nil {
		repo.TemplateSrc = info.SrcPath
		repo.TemplateVersion = info.Commit
		repo.TemplateLatest = info.LatestTag
		repo.TemplateDrift = info.Behind || !info.IsTag
	}

	if !summary.LastModified.IsZero() {
		lastModified := summary.LastModified
		repo.LastModified = &lastModified
	}

	return repo
}

// worktreeList drops bare worktrees, which hold no checkout to review.
func worktreeList(worktrees []vcs.WorktreeInfo) []Worktree {
	var out []Worktree

	for i := range worktrees {
		if worktrees[i].IsBare {
			continue
		}

		out = append(out, Worktree{
			Path:   worktrees[i].Path,
			Branch: worktrees[i].Branch,
			Locked: worktrees[i].IsLocked,
		})
	}

	return out
}

// lookupPR returns the pull request for branch from the cache, fetching via gh
// only when fresh is set. A miss (or fetch failure) yields nil.
func lookupPR(
	ctx context.Context, client githubClient, repoPath, remoteID, branch, upstream string, fresh bool,
) *forge.PullRequest {
	if upstream == "" {
		return nil
	}

	if cached, ok := github.CachedPRForBranch(repoPath, remoteID, branch, upstream); ok {
		return cached
	}
	if !fresh {
		return nil
	}

	pr, err := client.prForBranch(ctx, repoPath, remoteID, branch, upstream)
	if err != nil {
		return nil
	}

	return pr
}

// lookupPRCount returns the repo's open PR count from the cache, fetching via
// gh only when fresh is set. A miss (or fetch failure) yields nil.
func lookupPRCount(ctx context.Context, client githubClient, repoPath, remoteID, upstream string, fresh bool) *int {
	if upstream == "" {
		return nil
	}

	if cached, ok := github.CachedPRs(repoPath, remoteID, upstream); ok {
		count := len(cached)
		return &count
	}
	if !fresh {
		return nil
	}

	prs, err := client.prsForRepo(ctx, repoPath, remoteID, upstream)
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
