package app

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

// Command is a named `:command` invocable from the TUI's command bar.
type Command struct {
	Name        string
	Description string
	Complete    func(m Model, args []string) []string
	Run         func(m Model, args []string) (Model, tea.Cmd)
}

// Registry holds the set of available commands.
type Registry struct {
	commands []Command
}

// NewRegistry builds a Registry from the given commands.
func NewRegistry(commands ...Command) Registry {
	return Registry{commands: commands}
}

// Commands returns all registered commands.
func (r Registry) Commands() []Command {
	return r.commands
}

// Lookup resolves a command by exact name or unique prefix.
func (r Registry) Lookup(name string) (Command, bool) {
	var prefixMatches []Command
	for _, c := range r.commands {
		if c.Name == name {
			return c, true
		}
		if strings.HasPrefix(c.Name, name) {
			prefixMatches = append(prefixMatches, c)
		}
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0], true
	}

	return Command{}, false
}

// Candidates returns command names starting with the given prefix.
func (r Registry) Candidates(prefix string) []string {
	var names []string
	for _, c := range r.commands {
		if strings.HasPrefix(c.Name, prefix) {
			names = append(names, c.Name)
		}
	}

	return names
}

func filterModeNames() map[string]models.FilterMode {
	return map[string]models.FilterMode{
		"ahead":     models.FilterModeAhead,
		"all":       models.FilterModeAll,
		"behind":    models.FilterModeBehind,
		"dirty":     models.FilterModeDirty,
		"has_notes": models.FilterModeHasNotes,
		"has_pr":    models.FilterModeHasPR,
		"has_stash": models.FilterModeHasStash,
	}
}

func sortModeNames() map[string]models.SortMode {
	return map[string]models.SortMode{
		"branch":   models.SortModeBranch,
		"modified": models.SortModeModified,
		"name":     models.SortModeName,
		"status":   models.SortModeStatus,
	}
}

func namesMatching[T any](modes map[string]T, prefix string) []string {
	var names []string
	for name := range modes {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	return names
}

// DefaultRegistry builds the Registry of all built-in ":command" commands.
func DefaultRegistry() Registry {
	return NewRegistry(
		cleanupCommand(),
		batchCommand("fetch",
			"Fetch visible repos, optionally scoped: :fetch [predicate]",
			"Fetch All", batchFetchAllCmd),
		filterCommand(),
		Command{
			Name:        "help",
			Description: "Show help",
			Run: func(m Model, _ []string) (Model, tea.Cmd) {
				m.viewMode = ViewModeHelp
				return m, nil
			},
		},
		historyCommand(),
		batchCommand("prune",
			"Prune remote refs in visible repos, optionally scoped: :prune [predicate]",
			"Prune Remote", batchPruneRemoteCmd),
		Command{
			Name:        "quit",
			Description: "Quit",
			Run: func(m Model, _ []string) (Model, tea.Cmd) {
				return m, tea.Quit
			},
		},
		Command{
			Name:        "refresh",
			Description: "Clear caches and reload the current view",
			Run: func(m Model, _ []string) (Model, tea.Cmd) {
				return m.handleRefresh()
			},
		},
		selectCommand(),
		sortCommand(),
	)
}

// filterCommand builds the ":filter" command: a bare mode name, or a
// predicate expression, or no args to open the filter modal.
func filterCommand() Command {
	return Command{
		Name:        "filter",
		Description: "Filter repos: :filter <mode|predicate> or :filter to open the modal",
		Complete: func(_ Model, args []string) []string {
			prefix := ""
			if len(args) > 0 {
				prefix = args[len(args)-1]
			}

			return predicateCandidates(prefix)
		},
		Run: func(m Model, args []string) (Model, tea.Cmd) {
			if len(args) == 0 {
				m.viewMode = ViewModeFilter
				return m, nil
			}
			if len(args) == 1 {
				if mode, ok := filterModeNames()[args[0]]; ok {
					if mode == models.FilterModeAll {
						m.ResetFilters()
					} else {
						m.SetFilter(mode)
					}
					m.updateFilteredPaths()
					m.cursor = 0

					return m, nil
				}
			}
			expr := strings.Join(args, " ")
			pred, err := filters.ParsePredicate(expr)
			if err != nil {
				return m, statusCmd(err.Error())
			}
			m.SetPredicate(expr, pred)
			m.updateFilteredPaths()
			m.cursor = 0

			return m, nil
		},
	}
}

// selectCommand builds the ":select" command: "all", "none", or
// "where <predicate>".
func selectCommand() Command {
	return Command{
		Name:        "select",
		Description: "Mark repos: :select where <predicate>, :select all, :select none",
		Complete: func(_ Model, args []string) []string {
			if len(args) <= 1 {
				prefix := ""
				if len(args) == 1 {
					prefix = args[0]
				}

				return namesMatching(map[string]struct{}{"all": {}, "none": {}, "where": {}}, prefix)
			}

			return predicateCandidates(args[len(args)-1])
		},
		Run: runSelectCommand,
	}
}

func runSelectCommand(m Model, args []string) (Model, tea.Cmd) {
	if len(args) == 0 {
		return m, statusCmd("Usage: :select where <predicate> | :select all | :select none")
	}
	switch args[0] {
	case "none":
		m.selectedPaths = nil
		return m, nil
	case "all":
		m.selectedPaths = make(map[string]bool, len(m.repoPaths))
		for _, path := range m.repoPaths {
			m.selectedPaths[path] = true
		}

		return m, statusCmd(fmt.Sprintf("Selected %d repos", len(m.selectedPaths)))
	case "where":
		expr := strings.Join(args[1:], " ")
		pred, err := filters.ParsePredicate(expr)
		if err != nil {
			return m, statusCmd(err.Error())
		}
		m.selectedPaths = make(map[string]bool)
		for _, path := range m.repoPaths {
			if summary, ok := m.summaries[path]; ok && pred(summary) {
				m.selectedPaths[path] = true
			}
		}

		return m, statusCmd(fmt.Sprintf("Selected %d repos", len(m.selectedPaths)))
	default:
		return m, statusCmd("Unknown select action: " + args[0])
	}
}

// sortCommand builds the ":sort" command: a bare mode name to cycle, or no
// args to open the sort modal.
func sortCommand() Command {
	return Command{
		Name:        "sort",
		Description: "Cycle sort for a mode: :sort <mode> or :sort to open the modal",
		Complete: func(_ Model, args []string) []string {
			prefix := ""
			if len(args) > 0 {
				prefix = args[len(args)-1]
			}

			return namesMatching(sortModeNames(), prefix)
		},
		Run: func(m Model, args []string) (Model, tea.Cmd) {
			if len(args) == 0 {
				m.viewMode = ViewModeSort
				m.sortCursor = 0

				return m, nil
			}
			mode, ok := sortModeNames()[args[0]]
			if !ok {
				return m, statusCmd("Unknown sort: " + args[0])
			}
			m.CycleSortState(mode)
			m.updateFilteredPaths()

			return m, nil
		},
	}
}

// batchCommand builds a batch operator command that runs over the visible
// repos, narrowed by an optional predicate expression argument.
func batchCommand(name, description, taskName string, taskCmd func([]string) tea.Cmd) Command {
	return Command{
		Name:        name,
		Description: description,
		Complete: func(_ Model, args []string) []string {
			prefix := ""
			if len(args) > 0 {
				prefix = args[len(args)-1]
			}

			return predicateCandidates(prefix)
		},
		Run: func(m Model, args []string) (Model, tea.Cmd) {
			return runBatchCommand(m, args, taskName, taskCmd)
		},
	}
}

// runBatchCommand scopes paths to the visible repos, narrows them by an
// optional predicate expression, and starts the batch task on the result.
func runBatchCommand(m Model, args []string, taskName string, taskCmd func([]string) tea.Cmd) (Model, tea.Cmd) {
	paths := m.filteredPaths
	label := taskName
	if len(args) > 0 {
		expr := strings.Join(args, " ")
		pred, err := filters.ParsePredicate(expr)
		if err != nil {
			return m, statusCmd(err.Error())
		}
		paths = nil
		for _, path := range m.filteredPaths {
			if summary, ok := m.summaries[path]; ok && pred(summary) {
				paths = append(paths, path)
			}
		}
		label = fmt.Sprintf("%s (%s)", taskName, expr)
	}
	if len(paths) == 0 {
		return m, statusCmd("No repos match")
	}

	return m.startBatchTaskOn(label, paths, taskCmd)
}

// dryRunFlag is the ":cleanup" flag that previews deletions instead of
// performing them.
const dryRunFlag = "--dry-run"

// cleanupCommand builds the ":cleanup" command: deletes merged and
// squash-merged branches in the visible repos, optionally narrowed by a
// predicate, or previews the same detection with "--dry-run" instead of
// deleting anything.
func cleanupCommand() Command {
	return Command{
		Name: "cleanup",
		Description: "Delete merged branches in visible repos, optionally scoped: " +
			":cleanup [--dry-run] [predicate]",
		Complete: func(_ Model, args []string) []string {
			prefix := ""
			if len(args) > 0 {
				prefix = args[len(args)-1]
			}

			candidates := predicateCandidates(prefix)
			if !slices.Contains(args, dryRunFlag) && strings.HasPrefix(dryRunFlag, prefix) {
				candidates = append([]string{dryRunFlag}, candidates...)
			}

			return candidates
		},
		Run: func(m Model, args []string) (Model, tea.Cmd) {
			taskName := "Cleanup Merged"
			taskCmd := batchCleanupMergedCmd

			rest := make([]string, 0, len(args))
			for _, arg := range args {
				if arg == dryRunFlag {
					taskName = "Cleanup Merged (dry run)"
					taskCmd = batchPreviewCleanupCmd

					continue
				}
				rest = append(rest, arg)
			}

			return runBatchCommand(m, rest, taskName, taskCmd)
		},
	}
}

func predicateCandidates(prefix string) []string {
	var names []string
	for _, name := range filters.AtomNames() {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	for _, word := range []string{"and", "all", "not", "or"} {
		if strings.HasPrefix(word, prefix) && prefix != "" {
			names = append(names, word)
		}
	}
	slices.Sort(names)

	return names
}

func statusCmd(message string) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg{Message: message}
	}
}

// commandHistoryLimit caps the retained command history; historyStatusCount
// is how many recent entries ":history" shows.
const (
	commandHistoryLimit = 50
	historyStatusCount  = 5
)

// ExecuteCommand parses and dispatches a command line like "filter dirty",
// recording recognized commands in the history that ":history" and the "@:"
// repeat key replay.
func (m Model) ExecuteCommand(line string) (Model, tea.Cmd) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return m, nil
	}
	cmd, ok := m.registry.Lookup(fields[0])
	if !ok {
		return m, statusCmd("Unknown command: " + fields[0])
	}

	if cmd.Name != "history" {
		m.commandHistory = append(m.commandHistory, strings.TrimSpace(line))
		if len(m.commandHistory) > commandHistoryLimit {
			m.commandHistory = m.commandHistory[len(m.commandHistory)-commandHistoryLimit:]
		}
	}

	return cmd.Run(m, fields[1:])
}

// repeatLastCommand re-executes the most recent history entry ("@:").
func (m Model) repeatLastCommand() (Model, tea.Cmd) {
	if len(m.commandHistory) == 0 {
		return m, statusCmd("No command to repeat")
	}

	return m.ExecuteCommand(m.commandHistory[len(m.commandHistory)-1])
}

// historyCommand builds the ":history" command, showing the most recent
// command lines newest-first in the status bar.
func historyCommand() Command {
	return Command{
		Name:        "history",
		Description: "Show recent :commands (newest first); repeat the last with @:",
		Run: func(m Model, _ []string) (Model, tea.Cmd) {
			if len(m.commandHistory) == 0 {
				return m, statusCmd("History: (empty)")
			}

			start := max(0, len(m.commandHistory)-historyStatusCount)
			recent := slices.Clone(m.commandHistory[start:])
			slices.Reverse(recent)

			return m, statusCmd("History: " + strings.Join(recent, " | "))
		},
	}
}
