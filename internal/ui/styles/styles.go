// Package styles names the lipgloss styles the TUI's views share, built from
// the Catppuccin flavor aragonite detects for the terminal.
package styles

import (
	"charm.land/lipgloss/v2"
	"github.com/kyleking/aragonite/tui/markdown"
	"github.com/kyleking/aragonite/tui/theme"
)

// horizontalPadding is the left/right padding applied to modals and tabs.
const horizontalPadding = 2

// Palette is the flavor detected at startup. Every style below closes over
// these values at init, so reassigning it later changes nothing.
var Palette = theme.Detect()

// Palette colors, re-exported so views can name a color without reaching
// through Palette on every use.
var (
	Base     = Palette.Base
	Mantle   = Palette.Mantle
	Crust    = Palette.Crust
	Surface0 = Palette.Surface0
	Surface1 = Palette.Surface1
	Surface2 = Palette.Surface2
	Overlay0 = Palette.Overlay0
	Overlay1 = Palette.Overlay1
	Overlay2 = Palette.Overlay2
	Subtext0 = Palette.Subtext0
	Subtext1 = Palette.Subtext1
	Text     = Palette.Text

	Rosewater = Palette.Rosewater
	Flamingo  = Palette.Flamingo
	Pink      = Palette.Pink
	Mauve     = Palette.Mauve
	Red       = Palette.Red
	Maroon    = Palette.Maroon
	Peach     = Palette.Peach
	Yellow    = Palette.Yellow
	Green     = Palette.Green
	Teal      = Palette.Teal
	Sky       = Palette.Sky
	Sapphire  = Palette.Sapphire
	Blue      = Palette.Blue
	Lavender  = Palette.Lavender
)

// Shared lipgloss styles used across views.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Blue).
			PaddingLeft(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Subtext0)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(Subtext0).
			Bold(true)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(Text)

	SelectedRowStyle = lipgloss.NewStyle().
				Background(Surface0).
				Foreground(Text)

	TabStyle = lipgloss.NewStyle().
			Background(Surface0).
			Foreground(Overlay1)

	SelectedTabStyle = lipgloss.NewStyle().
				Background(Blue).
				Foreground(Base).
				Bold(true)

	// TabRuleStyle draws the divider separating the tab bar from the content
	// below it, so the bar reads as a strip rather than floating text.
	TabRuleStyle = lipgloss.NewStyle().
			Foreground(Surface1)

	DirtyStyle = lipgloss.NewStyle().
			Foreground(Peach)

	CleanStyle = lipgloss.NewStyle().
			Foreground(Green)

	AheadStyle = lipgloss.NewStyle().
			Foreground(Yellow)

	BehindStyle = lipgloss.NewStyle().
			Foreground(Sky)

	BranchStyle = lipgloss.NewStyle().
			Foreground(Mauve)

	PROpenStyle = lipgloss.NewStyle().
			Foreground(Green)

	PRDraftStyle = lipgloss.NewStyle().
			Foreground(Overlay1)

	PRMergedStyle = lipgloss.NewStyle().
			Foreground(Mauve)

	BadgeStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)

	FilterBadgeStyle = BadgeStyle.
				Background(Yellow).
				Foreground(Base)

	SearchBadgeStyle = BadgeStyle.
				Background(Mauve).
				Foreground(Base)

	SortBadgeStyle = BadgeStyle.
			Background(Blue).
			Foreground(Base)

	CountBadgeStyle = BadgeStyle.
			Background(Surface1).
			Foreground(Text)

	// CountStyle marks a count inside a fixed-width table cell. Unlike
	// CountBadgeStyle it carries no padding, so it cannot widen its column
	// and shift the columns rendered after it.
	CountStyle = lipgloss.NewStyle().
			Foreground(Lavender).
			Bold(true)

	NotesBadgeStyle = lipgloss.NewStyle().
			Foreground(Teal).
			Bold(true)

	NotesPreviewNameStyle = lipgloss.NewStyle().
				Foreground(Teal)

	NotesPreviewLineStyle = lipgloss.NewStyle().
				Foreground(Subtext0)

	// NotesPreviewBangStyle marks a note line flagged with a leading '!' as
	// worth a second look, without giving up the section's muted look.
	NotesPreviewBangStyle = lipgloss.NewStyle().
				Foreground(Yellow)

	FooterStyle = lipgloss.NewStyle().
			Foreground(Subtext0)

	FooterKeyStyle = lipgloss.NewStyle().
			Foreground(Blue).
			Bold(true)

	FooterDescStyle = lipgloss.NewStyle().
			Foreground(Subtext0)

	StatusMessageStyle = lipgloss.NewStyle().
				Foreground(Green).
				Background(Surface0).
				Padding(0, 1)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Surface1)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Blue).
			Padding(1, horizontalPadding).
			Background(Base)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Red)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Green)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Yellow)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Blue).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(Subtext0)

	TabActiveStyle = lipgloss.NewStyle().
			Foreground(Blue).
			Bold(true).
			Underline(true).
			Padding(0, horizontalPadding)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(Subtext0).
				Padding(0, horizontalPadding)

	TabSeparatorStyle = lipgloss.NewStyle().
				Foreground(Surface1)
)

// Badge renders text with the given style.
func Badge(text string, style lipgloss.Style) string {
	return style.Render(text)
}

// StatusBadge renders a CI/check status as a colored badge.
func StatusBadge(status string) string {
	switch status {
	case "passing", "success":
		return Badge(status, BadgeStyle.Background(Green).Foreground(Base))
	case "failing", "failure":
		return Badge(status, BadgeStyle.Background(Red).Foreground(Base))
	case "pending", "running":
		return Badge(status, BadgeStyle.Background(Yellow).Foreground(Base))
	default:
		return Badge(status, BadgeStyle.Background(Surface1).Foreground(Text))
	}
}

// PRStatusBadge renders a pull request's state as a colored badge.
func PRStatusBadge(state string, isDraft bool) string {
	if isDraft {
		return Badge("DRAFT", PRDraftStyle)
	}
	switch state {
	case "OPEN":
		return Badge("OPEN", PROpenStyle)
	case "MERGED":
		return Badge("MERGED", PRMergedStyle)
	case "CLOSED":
		return Badge("CLOSED", ErrorStyle)
	default:
		return Badge(state, SubtitleStyle)
	}
}

// Markdown maps this app's named styles onto the roles the markdown renderer
// asks for.
var Markdown = markdown.Styles{
	Body:    TableRowStyle,
	Heading: HeaderStyle,
	Muted:   SubtitleStyle,
}
