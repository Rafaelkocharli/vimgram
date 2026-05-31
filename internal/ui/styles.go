package ui

import "github.com/charmbracelet/lipgloss"

// All lipgloss styles live here so views can be pure compositions.
var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5fafff")).MarginBottom(1)
	// chatTitleStyle has no bottom margin so the chat header is exactly one
	// line — required for the fixed-height, bottom-pinned layout.
	chatTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5fafff"))
	labelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a0a0a0"))
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5fff87"))
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff5f5f"))
	footerStyle  = lipgloss.NewStyle().Faint(true).MarginTop(1)
	dimStyle     = lipgloss.NewStyle().Faint(true)

	chipGroup   = chipStyle("#5f87d7")
	chipChannel = chipStyle("#af87d7")
	chipDM      = chipStyle("#5faf87")

	selBg = lipgloss.NewStyle().Background(lipgloss.Color("#444444")).Foreground(lipgloss.Color("#ffffff"))

	// Fixed-width column for dialog titles in the chat list. Hoisted out of
	// the per-row hot path to avoid allocating a style on every frame.
	dialogTitleStyle = lipgloss.NewStyle().Width(30)

	// All message bodies are white; the sender's name carries the color.
	msgTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))

	// namePalette approximates Telegram's peer-color palette (indices 0..6:
	// red, orange, purple, green, cyan, blue, pink). Names are colored by their
	// Telegram color index when known, otherwise by a stable hash of the name —
	// either way different users get different colors.
	namePalette = []lipgloss.Color{
		"#e0524b", // 0 red
		"#df9a3a", // 1 orange
		"#9c6ce0", // 2 purple
		"#4ab74e", // 3 green
		"#3fb8c0", // 4 cyan
		"#508cf0", // 5 blue
		"#e36fae", // 6 pink
	}
	// unreadStyle paints the exactly-3-char unread badge (see unreadBadge).
	// No padding here — width is controlled by the rendered string itself.
	unreadStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#5fafd7"))

	modeNormalStyle  = modeBadgeStyle("#5fafff")
	modeVisualStyle  = modeBadgeStyle("#d75fd7")
	modeEditStyle    = modeBadgeStyle("#5fff87")
	modeCommandStyle = modeBadgeStyle("#ffaf00")
	cmdLineStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))

	statusOnlineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fff87"))
	statusTypingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))
	statusOfflineStyle = lipgloss.NewStyle().Faint(true)

	// Chat cursor line (Normal mode): full-line background, and a block
	// cursor character on the active column.
	cursorLineStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2d2d4a")).
			Foreground(lipgloss.Color("#e0e0e0"))
	cursorCharStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#5fafff")).
			Foreground(lipgloss.Color("#000000"))

	// Window focus / separator bars.
	focusBarStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5fafff"))
	dimBarStyle   = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#444444"))
)

func chipStyle(bg string) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color("#ffffff")).
		Padding(0, 1)
}

func modeBadgeStyle(bg string) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color("#000000"))
}

