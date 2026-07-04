package app

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/gh-repo-dashboard/internal/filters"
	"github.com/kyleking/gh-repo-dashboard/internal/models"
)

type Command struct {
	Name        string
	Description string
	Complete    func(m Model, args []string) []string
	Run         func(m Model, args []string) (Model, tea.Cmd)
}

type Registry struct {
	commands []Command
}

func NewRegistry(commands ...Command) Registry {
	return Registry{commands: commands}
}

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

func DefaultRegistry() Registry {
	return NewRegistry(
		Command{
			Name:        "fetch",
			Description: "Fetch all visible repos",
			Run: func(m Model, args []string) (Model, tea.Cmd) {
				newModel, cmd := m.startBatchTask("Fetch All", batchFetchAllCmd)
				return newModel.(Model), cmd
			},
		},
		Command{
			Name:        "filter",
			Description: "Filter repos: :filter <mode|predicate> or :filter to open the modal",
			Complete: func(m Model, args []string) []string {
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
		},
		Command{
			Name:        "help",
			Description: "Show help",
			Run: func(m Model, args []string) (Model, tea.Cmd) {
				m.viewMode = ViewModeHelp
				return m, nil
			},
		},
		Command{
			Name:        "quit",
			Description: "Quit",
			Run: func(m Model, args []string) (Model, tea.Cmd) {
				return m, tea.Quit
			},
		},
		Command{
			Name:        "refresh",
			Description: "Clear caches and reload the current view",
			Run: func(m Model, args []string) (Model, tea.Cmd) {
				newModel, cmd := m.handleRefresh()
				return newModel.(Model), cmd
			},
		},
		Command{
			Name:        "select",
			Description: "Mark repos: :select where <predicate>, :select all, :select none",
			Complete: func(m Model, args []string) []string {
				if len(args) <= 1 {
					prefix := ""
					if len(args) == 1 {
						prefix = args[0]
					}
					return namesMatching(map[string]struct{}{"all": {}, "none": {}, "where": {}}, prefix)
				}
				return predicateCandidates(args[len(args)-1])
			},
			Run: func(m Model, args []string) (Model, tea.Cmd) {
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
					return m, statusCmd(fmt.Sprintf("Unknown select action: %s", args[0]))
				}
			},
		},
		Command{
			Name:        "sort",
			Description: "Cycle sort for a mode: :sort <mode> or :sort to open the modal",
			Complete: func(m Model, args []string) []string {
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
					return m, statusCmd(fmt.Sprintf("Unknown sort: %s", args[0]))
				}
				m.CycleSortState(mode)
				m.updateFilteredPaths()
				return m, nil
			},
		},
	)
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

// ExecuteCommand parses and dispatches a command line like "filter dirty".
func (m Model) ExecuteCommand(line string) (Model, tea.Cmd) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return m, nil
	}
	cmd, ok := m.registry.Lookup(fields[0])
	if !ok {
		return m, statusCmd(fmt.Sprintf("Unknown command: %s", fields[0]))
	}
	return cmd.Run(m, fields[1:])
}
