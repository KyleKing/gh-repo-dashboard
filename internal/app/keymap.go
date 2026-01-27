package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Quit   key.Binding
	Help   key.Binding
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Top    key.Binding
	Bottom key.Binding
	Enter  key.Binding
	Back   key.Binding
	Tab    key.Binding

	Refresh key.Binding
	Filter  key.Binding
	Sort    key.Binding
	Search  key.Binding
	Reverse key.Binding

	FetchAll      key.Binding
	PruneRemote   key.Binding
	CleanupMerged key.Binding

	OpenPR       key.Binding
	CopyBranch   key.Binding
	CopyURL      key.Binding
	CopyPRNumber key.Binding
	OpenURL      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h/←", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l/→", "right"),
		),
		Top: key.NewBinding(
			key.WithKeys("g", "home"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G", "end"),
			key.WithHelp("G", "bottom"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r", "ctrl+r"),
			key.WithHelp("r/ctrl+r", "refresh"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Reverse: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "reverse"),
		),
		FetchAll: key.NewBinding(
			key.WithKeys("F"),
			key.WithHelp("F", "fetch all"),
		),
		PruneRemote: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "prune"),
		),
		CleanupMerged: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "cleanup"),
		),
		OpenPR: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "open/create PR"),
		),
		CopyBranch: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "copy branch name"),
		),
		CopyURL: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "copy URL"),
		),
		CopyPRNumber: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "copy PR number"),
		),
		OpenURL: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open URL"),
		),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Filter, k.Sort, k.Search, k.Refresh, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom},
		{k.Enter, k.Back},
		{k.Filter, k.Sort, k.Search},
		{k.Refresh, k.FetchAll, k.PruneRemote, k.CleanupMerged},
		{k.Help, k.Quit},
	}
}
