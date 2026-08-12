package app

import (
	"path/filepath"
	"strconv"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// currentPRView is the saved search the PRs tab is showing.
func (m Model) currentPRView() models.PRView {
	views := models.PRViews()
	if len(views) == 0 {
		return models.PRView{Name: "Open", Search: "is:open"}
	}

	return views[min(max(m.prViewIndex, 0), len(views)-1)]
}

// prSearchRepo is the repo a search runs from: the one the cursor last opened,
// falling back to the first of the fleet. A fleet-wide search still needs one,
// because gh reads its credentials from a working directory.
func (m Model) prSearchRepo() string {
	if m.selectedRepo != "" {
		return m.selectedRepo
	}
	if len(m.filteredPaths) > 0 {
		return m.filteredPaths[0]
	}
	if len(m.repoPaths) > 0 {
		return m.repoPaths[0]
	}

	return ""
}

// openPRList shows the PRs tab, reading the current view if its answer is not
// already on screen.
func (m Model) openPRList() (Model, tea.Cmd) {
	m.viewMode = ViewModePRList

	if len(m.prSearch) > 0 || m.prSearchLoading {
		return m, nil
	}

	return m.runPRSearch()
}

// runPRSearch reads the current view. The cache behind it answers a repeat
// within the TTL without touching the network, so cycling back to a view
// already read is free and refresh is what forces a re-read.
func (m Model) runPRSearch() (Model, tea.Cmd) {
	repo := m.prSearchRepo()
	if repo == "" {
		m.prSearchError = "No repository to search from"

		return m, nil
	}

	m.prSearch = nil
	m.prSearchCursor = 0
	m.prSearchError = ""
	m.prSearchLoading = true

	view := m.currentPRView()

	return m, loadPRSearchCmd(repo, m.summaries[repo].RemoteID, view.Search, m.prFleet)
}

// handlePRSearchLoaded takes the rows a search returned, ignoring an answer to
// a question that is no longer on screen.
func (m Model) handlePRSearchLoaded(msg PRSearchLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Query != m.currentPRView().Search || msg.Fleet != m.prFleet {
		return m, nil
	}

	m.prSearchLoading = false
	if msg.Error != nil {
		m.prSearchError = "Search failed: " + msg.Error.Error()

		return m, nil
	}

	m.prSearch = msg.PRs
	m.prSearchCursor = 0
	m.prSearchError = ""

	return m, nil
}

// selectedSearchPR is the pull request under the PRs tab cursor.
func (m Model) selectedSearchPR() (models.PRInfo, bool) {
	if m.prSearchCursor < 0 || m.prSearchCursor >= len(m.prSearch) {
		return models.PRInfo{}, false
	}

	return m.prSearch[m.prSearchCursor], true
}

// searchPRRepoPath is the local checkout a search row belongs to. A row read
// for this repo carries no repository name and is this repo by definition; a
// fleet row names one, and matching it against the scanned remotes is what
// makes a local checkout possible at all.
func (m Model) searchPRRepoPath(pr models.PRInfo) (string, bool) {
	if pr.Repo == "" {
		return m.prSearchRepo(), m.prSearchRepo() != ""
	}

	for path := range m.summaries {
		if m.summaries[path].RemoteRepo == pr.Repo {
			return path, true
		}
	}

	return "", false
}

// cyclePRView moves to the next saved view in the given direction and reads it.
func (m Model) cyclePRView(step int) (tea.Model, tea.Cmd) {
	views := models.PRViews()
	if len(views) == 0 {
		return m, nil
	}

	m.prViewIndex = (m.prViewIndex + step + len(views)) % len(views)

	return m.runPRSearch()
}

// setPRScope reads the current view against one repo or against everything the
// search reaches.
func (m Model) setPRScope(fleet bool) (tea.Model, tea.Cmd) {
	scoped := m
	scoped.prFleet = fleet

	return scoped.runPRSearch()
}

// openSearchPRDetail opens the full-screen detail for the row under the
// cursor, which needs the local checkout the pull request belongs to.
func (m Model) openSearchPRDetail() (tea.Model, tea.Cmd) {
	pr, ok := m.selectedSearchPR()
	if !ok {
		return m, nil
	}

	repo, found := m.searchPRRepoPath(pr)
	if !found {
		return m, statusCmd(pr.Repo + " is not one of the repos scanned here")
	}

	m.selectedRepo = repo
	m.selectedPR = pr
	m.prDetail = models.PRDetail{PRInfo: pr}
	m.prDetailScroll = 0
	m.prListReturn = ViewModePRList
	m.viewMode = ViewModePRDetail

	return m, loadPRDetailCmd(repo, m.summaries[repo].RemoteID, pr.Number)
}

// startCheckoutSearchPR checks the row under the cursor out into its own
// repository, which is the point of the tab: find the pull request, then work
// on it.
func (m Model) startCheckoutSearchPR() (tea.Model, tea.Cmd) {
	pr, ok := m.selectedSearchPR()
	if !ok {
		return m, nil
	}

	repo, found := m.searchPRRepoPath(pr)
	if !found {
		return m, statusCmd(pr.Repo + " is not one of the repos scanned here")
	}
	if repo == m.selectedRepo {
		if refusal, held := m.checkoutRefusal(pr.HeadRef); held {
			return m, statusCmd(refusal)
		}
	}

	detail := "#" + strconv.Itoa(pr.Number) + " " + pr.Title

	return m.confirmRun("Check the PR branch out?", detail, "in "+filepath.Base(repo),
		func(m Model) (Model, tea.Cmd) {
			return m, checkoutPRCmd(repo, pr.Number)
		})
}

// openSearchPRURL and copySearchPRURL act on the row under the cursor rather
// than on the repo grid's selection, since the PRs tab has no grid.
func (m Model) openSearchPRURL() (tea.Model, tea.Cmd) {
	pr, ok := m.selectedSearchPR()
	if !ok || pr.URL == "" {
		return m, statusCmd("No pull request URL to open")
	}

	return m, openURLCmd(pr.URL)
}

func (m Model) copySearchPRURL() (tea.Model, tea.Cmd) {
	pr, ok := m.selectedSearchPR()
	if !ok || pr.URL == "" {
		return m, statusCmd("No pull request URL to copy")
	}

	return m, copyToClipboardCmd(pr.URL)
}

// prListActions are the verbs the PRs tab offers, in the same leader-key shape
// the panels use.
func prListActions() []panelAction {
	return []panelAction{
		{key: "c", name: "check out", run: Model.startCheckoutSearchPR},
		{key: "o", name: "open in browser", run: Model.openSearchPRURL},
		{key: "u", name: descCopyURL, run: Model.copySearchPRURL},
	}
}

// handlePRListKey answers the PRs tab: navigation, which saved view is on
// screen, how wide it looks, and the verbs for the row under the cursor.
func (m Model) handlePRListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.panelActions {
		return m.handlePanelActionKey(msg)
	}
	if m.prViewMenu {
		return m.handlePRViewMenuKey(msg)
	}

	if tab, ok := tabForKey(msg.String()); ok {
		return m.openTab(tab)
	}

	last := max(len(m.prSearch)-1, 0)

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.viewMode = ViewModeRepoList

		return m, nil

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.Up):
		m.prSearchCursor = max(m.prSearchCursor-1, 0)

		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.prSearchCursor = min(m.prSearchCursor+1, last)

		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.prSearchCursor = 0

		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.prSearchCursor = last

		return m, nil

	case key.Matches(msg, m.keys.Enter):
		return m.openSearchPRDetail()
	}

	return m.handlePRViewKey(msg)
}

// handlePRViewKey answers the keys that change which view is on screen and how
// wide it looks, rather than where the cursor is.
func (m Model) handlePRViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case panelActionLeader:
		m.panelActions = true

		return m, nil

	case "f":
		m.prViewMenu = true

		return m, nil

	case "]":
		return m.cyclePRView(1)

	case "[":
		return m.cyclePRView(-1)

	case "*":
		return m.setPRScope(!m.prFleet)
	}

	return m, nil
}

// handlePRViewMenuKey answers the view picker: a number picks that view,
// enter takes the one under the cursor, and anything else backs out.
func (m Model) handlePRViewMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.prViewMenu = false

	views := models.PRViews()

	if pick, err := strconv.Atoi(msg.String()); err == nil && pick >= 1 && pick <= len(views) {
		m.prViewIndex = pick - 1

		return m.runPRSearch()
	}

	if msg.String() == keyEnter {
		return m.runPRSearch()
	}

	return m, nil
}
