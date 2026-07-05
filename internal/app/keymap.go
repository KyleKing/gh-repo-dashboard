package app

import "charm.land/bubbles/v2/key"

// Key name strings shared across key bindings, key handling, footers, and fixture tests.
const (
	keyEnter = "enter"
	keyEsc   = "esc"
)

// KeyMap holds the key bindings used across the TUI.
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

	Command key.Binding
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

// DefaultKeyMap returns the built-in key bindings.
func DefaultKeyMap() KeyMap {
	km := navigationKeyMap()
	km.FetchAll = key.NewBinding(
		key.WithKeys("F"),
		key.WithHelp("F+obj", "fetch"),
	)
	km.PruneRemote = key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P+obj", "prune"),
	)
	km.CleanupMerged = key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C+obj", "cleanup"),
	)
	km.OpenPR = key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "open/create PR"),
	)
	km.CopyBranch = key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "copy branch name"),
	)
	km.CopyURL = key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "copy URL"),
	)
	km.CopyPRNumber = key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "copy PR number"),
	)
	km.OpenURL = key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open URL"),
	)

	return km
}

// navigationKeyMap builds the movement, mode-switching, and command-line bindings;
// DefaultKeyMap layers the batch-operation and clipboard/PR bindings on top.
func navigationKeyMap() KeyMap {
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
			key.WithKeys(keyEnter, "space"),
			key.WithHelp(keyEnter, "select"),
		),
		Back: key.NewBinding(
			key.WithKeys(keyEsc, "backspace"),
			key.WithHelp(keyEsc, "back"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		Command: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "command"),
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
	}
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Filter, k.Sort, k.Search, k.Command, k.Refresh, k.Quit}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom},
		{k.Enter, k.Back},
		{k.Filter, k.Sort, k.Search, k.Command},
		{k.Refresh, k.FetchAll, k.PruneRemote, k.CleanupMerged},
		{k.Help, k.Quit},
	}
}
