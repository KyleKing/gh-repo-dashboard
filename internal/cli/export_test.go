package cli

import (
	"context"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// GitHubClient exposes the unexported githubClient type to black-box tests.
type GitHubClient = githubClient

// NewGitHubClient builds a githubClient from injected fetchers for black-box tests.
func NewGitHubClient(
	prForBranch func(ctx context.Context, repoPath, branch, upstream string) (*models.PRInfo, error),
	prsForRepo func(ctx context.Context, repoPath, upstream string) ([]models.PRInfo, error),
) githubClient {
	return githubClient{prForBranch: prForBranch, prsForRepo: prsForRepo}
}

// NewGitHubClientWithCI builds a githubClient that also answers CI lookups.
func NewGitHubClientWithCI(
	defaultCI func(ctx context.Context, repoPath string) (*models.DefaultBranchCI, error),
) githubClient {
	return githubClient{defaultCI: defaultCI}
}

// LookupCI exposes the unexported lookupCI helper to black-box tests.
var LookupCI = lookupCI

// LookupPR exposes the unexported lookupPR helper to black-box tests.
var LookupPR = lookupPR

// LookupPRCount exposes the unexported lookupPRCount helper to black-box tests.
var LookupPRCount = lookupPRCount

// NewRepo exposes the unexported newRepo helper to black-box tests.
var NewRepo = newRepo

// WriteOutput exposes the unexported writeOutput helper to black-box tests.
var WriteOutput = writeOutput
