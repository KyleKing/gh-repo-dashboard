package filters

import (
	"testing"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

func TestFilterReposAll(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Ahead: 1},
		"/repo2": {Path: "/repo2", Behind: 2},
		"/repo3": {Path: "/repo3"},
	}

	result := FilterRepos(paths, summaries, models.FilterModeAll)
	if len(result) != 3 {
		t.Errorf("expected 3 repos, got %d", len(result))
	}
}

func TestFilterReposAhead(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Ahead: 1},
		"/repo2": {Path: "/repo2", Behind: 2},
		"/repo3": {Path: "/repo3"},
	}

	result := FilterRepos(paths, summaries, models.FilterModeAhead)
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
	if result[0] != "/repo1" {
		t.Errorf("expected /repo1, got %s", result[0])
	}
}

func TestFilterReposBehind(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Ahead: 1},
		"/repo2": {Path: "/repo2", Behind: 2},
		"/repo3": {Path: "/repo3"},
	}

	result := FilterRepos(paths, summaries, models.FilterModeBehind)
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
	if result[0] != "/repo2" {
		t.Errorf("expected /repo2, got %s", result[0])
	}
}

func TestFilterReposDirty(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Staged: 2},
		"/repo2": {Path: "/repo2", Unstaged: 1},
		"/repo3": {Path: "/repo3"},
	}

	result := FilterRepos(paths, summaries, models.FilterModeDirty)
	if len(result) != 2 {
		t.Errorf("expected 2 repos, got %d", len(result))
	}
}

func TestFilterReposHasPR(t *testing.T) {
	paths := []string{"/repo1", "/repo2"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", PRInfo: &models.PRInfo{Number: 123}},
		"/repo2": {Path: "/repo2"},
	}

	result := FilterRepos(paths, summaries, models.FilterModeHasPR)
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
	if result[0] != "/repo1" {
		t.Errorf("expected /repo1, got %s", result[0])
	}
}

func TestFilterReposHasStash(t *testing.T) {
	paths := []string{"/repo1", "/repo2"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", StashCount: 3},
		"/repo2": {Path: "/repo2"},
	}

	result := FilterRepos(paths, summaries, models.FilterModeHasStash)
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
	if result[0] != "/repo1" {
		t.Errorf("expected /repo1, got %s", result[0])
	}
}

func TestFilterReposMultiNoFilters(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Ahead: 1},
		"/repo2": {Path: "/repo2", Behind: 2},
		"/repo3": {Path: "/repo3"},
	}

	activeFilters := []models.ActiveFilter{
		{Mode: models.FilterModeAll, Enabled: true, Inverted: false},
	}

	result := FilterReposMulti(paths, summaries, activeFilters)
	if len(result) != 3 {
		t.Errorf("expected 3 repos, got %d", len(result))
	}
}

func TestFilterReposMultiSingleFilter(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Ahead: 1},
		"/repo2": {Path: "/repo2", Behind: 2},
		"/repo3": {Path: "/repo3"},
	}

	activeFilters := []models.ActiveFilter{
		{Mode: models.FilterModeAll, Enabled: false, Inverted: false},
		{Mode: models.FilterModeAhead, Enabled: true, Inverted: false},
	}

	result := FilterReposMulti(paths, summaries, activeFilters)
	if len(result) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result))
	}
	if result[0] != "/repo1" {
		t.Errorf("expected /repo1, got %s", result[0])
	}
}

func TestFilterReposMultipleFilters(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3", "/repo4"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Ahead: 1, Staged: 2},
		"/repo2": {Path: "/repo2", Ahead: 1},
		"/repo3": {Path: "/repo3", Staged: 1},
		"/repo4": {Path: "/repo4"},
	}

	activeFilters := []models.ActiveFilter{
		{Mode: models.FilterModeAll, Enabled: false, Inverted: false},
		{Mode: models.FilterModeAhead, Enabled: true, Inverted: false},
		{Mode: models.FilterModeDirty, Enabled: true, Inverted: false},
	}

	result := FilterReposMulti(paths, summaries, activeFilters)
	if len(result) != 2 {
		t.Errorf("expected 2 repos (both ahead AND dirty; note IsDirty includes Ahead), got %d", len(result))
	}
}

func TestFilterReposMultiWithPRAndDirty(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Staged: 2, PRInfo: &models.PRInfo{Number: 123}},
		"/repo2": {Path: "/repo2", PRInfo: &models.PRInfo{Number: 456}},
		"/repo3": {Path: "/repo3", Staged: 1},
	}

	activeFilters := []models.ActiveFilter{
		{Mode: models.FilterModeAll, Enabled: false, Inverted: false},
		{Mode: models.FilterModeDirty, Enabled: true, Inverted: false},
		{Mode: models.FilterModeHasPR, Enabled: true, Inverted: false},
	}

	result := FilterReposMulti(paths, summaries, activeFilters)
	if len(result) != 1 {
		t.Errorf("expected 1 repo (both dirty AND has PR), got %d", len(result))
	}
	if len(result) > 0 && result[0] != "/repo1" {
		t.Errorf("expected /repo1, got %s", result[0])
	}
}

func TestFilterReposMultiWithInverted(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3", "/repo4"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Staged: 2},
		"/repo2": {Path: "/repo2", Unstaged: 1},
		"/repo3": {Path: "/repo3"},
		"/repo4": {Path: "/repo4"},
	}

	activeFilters := []models.ActiveFilter{
		{Mode: models.FilterModeAll, Enabled: false, Inverted: false},
		{Mode: models.FilterModeDirty, Enabled: true, Inverted: true},
	}

	result := FilterReposMulti(paths, summaries, activeFilters)
	if len(result) != 2 {
		t.Errorf("expected 2 repos (NOT dirty), got %d", len(result))
	}
}

func TestFilterReposMultiMixedInverted(t *testing.T) {
	paths := []string{"/repo1", "/repo2", "/repo3", "/repo4"}
	summaries := map[string]models.RepoSummary{
		"/repo1": {Path: "/repo1", Ahead: 1, PRInfo: &models.PRInfo{Number: 123}},
		"/repo2": {Path: "/repo2", Ahead: 1},
		"/repo3": {Path: "/repo3", PRInfo: &models.PRInfo{Number: 456}},
		"/repo4": {Path: "/repo4"},
	}

	activeFilters := []models.ActiveFilter{
		{Mode: models.FilterModeAll, Enabled: false, Inverted: false},
		{Mode: models.FilterModeAhead, Enabled: true, Inverted: false},
		{Mode: models.FilterModeHasPR, Enabled: true, Inverted: true},
	}

	result := FilterReposMulti(paths, summaries, activeFilters)
	if len(result) != 1 {
		t.Errorf("expected 1 repo (ahead AND NOT has PR), got %d", len(result))
	}
	if len(result) > 0 && result[0] != "/repo2" {
		t.Errorf("expected /repo2, got %s", result[0])
	}
}
