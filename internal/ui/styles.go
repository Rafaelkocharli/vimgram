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

	outMsgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafff"))
	inMsgStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#dddddd"))
	unreadStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#5fafd7")).Padding(0, 1)

	modeNormalStyle  = modeBadgeStyle("#5fafff")
	modeVisualStyle  = modeBadgeStyle("#d75fd7")
	modeEditStyle    = modeBadgeStyle("#5fff87")
	modeCommandStyle = modeBadgeStyle("#ffaf00")
	cmdLineStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))

	statusOnlineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fff87"))
	statusTypingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))
	statusOfflineStyle = lipgloss.NewStyle().Faint(true)

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

