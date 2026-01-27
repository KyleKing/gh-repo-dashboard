package styles

import "github.com/charmbracelet/lipgloss"

var (
	Base     = lipgloss.Color("#24273a")
	Mantle   = lipgloss.Color("#1e2030")
	Crust    = lipgloss.Color("#181926")
	Surface0 = lipgloss.Color("#363a4f")
	Surface1 = lipgloss.Color("#494d64")
	Surface2 = lipgloss.Color("#5b6078")
	Overlay0 = lipgloss.Color("#6e738d")
	Overlay1 = lipgloss.Color("#8087a2")
	Overlay2 = lipgloss.Color("#939ab7")
	Subtext0 = lipgloss.Color("#a5adcb")
	Subtext1 = lipgloss.Color("#b8c0e0")
	Text     = lipgloss.Color("#cad3f5")

	Rosewater = lipgloss.Color("#f4dbd6")
	Flamingo  = lipgloss.Color("#f0c6c6")
	Pink      = lipgloss.Color("#f5bde6")
	Mauve     = lipgloss.Color("#c6a0f6")
	Red       = lipgloss.Color("#ed8796")
	Maroon    = lipgloss.Color("#ee99a0")
	Peach     = lipgloss.Color("#f5a97f")
	Yellow    = lipgloss.Color("#eed49f")
	Green     = lipgloss.Color("#a6da95")
	Teal      = lipgloss.Color("#8bd5ca")
	Sky       = lipgloss.Color("#91d7e3")
	Sapphire  = lipgloss.Color("#7dc4e4")
	Blue      = lipgloss.Color("#8aadf4")
	Lavender  = lipgloss.Color("#b7bdf8")
)

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

	FooterStyle = lipgloss.NewStyle().
			Foreground(Subtext0)

	FooterKeyStyle = lipgloss.NewStyle().
			Foreground(Blue).
			Bold(true)

	FooterDescStyle = lipgloss.NewStyle().
			Foreground(Subtext0)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Surface1)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Blue).
			Padding(1, 2).
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
			Padding(0, 2)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(Subtext0).
				Padding(0, 2)

	TabSeparatorStyle = lipgloss.NewStyle().
				Foreground(Surface1)
)

func Badge(text string, style lipgloss.Style) string {
	return style.Render(text)
}

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
