// Package styles holds the one skin the TUI draws with, plus the faces that
// only this tool has a use for.
package styles

import (
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/markdown"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

// Palette is Ink, whatever the terminal's background, so this tool and its
// siblings read the same on a light terminal as on a dark one.
var Palette = theme.Ink()

// Skin is aragonite's shared faces under this tool's accent. Sapphire is
// unused by the siblings, so the accent tells the tools apart at a glance.
var Skin = skin.New(Palette, Palette.Sapphire)

// Faces for state that only a repository dashboard draws.
var (
	Ahead  = Skin.Warning
	Behind = Skin.Muted
	Dirty  = Skin.Warning

	PRDraft  = Skin.Muted
	PRMerged = lipgloss.NewStyle().Foreground(Palette.Mauve)
	PROpen   = Skin.Success
)

// Markdown maps the skin onto the roles the markdown renderer asks for.
var Markdown = markdown.Styles{
	Body:    Skin.Body,
	Heading: Skin.Heading,
	Muted:   Skin.Subtitle,
}

// Selected paints s over the row under the cursor, so a colored cell keeps its
// color and gains the selection's background.
func Selected(s lipgloss.Style) lipgloss.Style {
	return s.Background(Skin.Selection.GetBackground())
}
