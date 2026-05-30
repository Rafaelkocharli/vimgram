package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"vimgram/internal/app"
)

// View implements tea.Model.
func (m Model) View() string {
	switch m.screen {
	case app.ScreenAuth:
		return m.viewAuth()
	case app.ScreenChatList:
		return m.viewChatList()
	case app.ScreenChat:
		return m.viewChat()
	}
	return ""
}

// ----- Auth ---------------------------------------------------------------

func (m Model) viewAuth() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("vimgram · sign in"))
	b.WriteString("\n")
	b.WriteString(m.viewAuthBody())
	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}
	b.WriteString("\n")
	b.WriteString(m.viewAuthFooter())
	return b.String()
}

func (m Model) viewAuthBody() string {
	switch m.authState {
	case app.AuthConnecting:
		return m.spin.View() + " Connecting to Telegram..."
	case app.AuthLoading:
		return m.spin.View() + " Processing..."
	case app.AuthNeedPhone:
		return labelStyle.Render("Phone number (with country code):") + "\n" + m.authInput.View()
	case app.AuthNeedCode:
		return labelStyle.Render("Telegram code:") + "\n" + m.authInput.View()
	case app.AuthNeedPassword:
		return labelStyle.Render("2FA password:") + "\n" + m.authInput.View()
	}
	return ""
}

func (m Model) viewAuthFooter() string {
	if isAuthPromptActive(m.authState) {
		return footerStyle.Render("enter — submit · esc — quit")
	}
	return footerStyle.Render("esc — quit")
}

// ----- Chat list ----------------------------------------------------------

func (m Model) viewChatList() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Chats · %s", m.self.DisplayName())))
	b.WriteString("\n")

	if len(m.dialogs) == 0 {
		b.WriteString(dimStyle.Render("No chats"))
	} else {
		b.WriteString(m.viewChatListRows())
	}

	b.WriteString("\n")
	b.WriteString(m.statusLine())
	return b.String()
}

func (m Model) viewChatListRows() string {
	var b strings.Builder
	rows := m.visibleRows()
	end := m.listOffset + rows
	if end > len(m.dialogs) {
		end = len(m.dialogs)
	}
	for i := m.listOffset; i < end; i++ {
		b.WriteString(m.renderDialogRow(i, i == m.cursor))
		b.WriteString("\n")
	}
	if len(m.dialogs) > rows {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d / %d", m.cursor+1, len(m.dialogs))))
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderDialogRow(idx int, selected bool) string {
	d := m.dialogs[idx]
	chip := dialogChip(d.Kind)
	title := titleOrFallback(d.Title)
	titleCol := lipgloss.NewStyle().Width(30).Render(truncRunes(title, 28))

	previewWidth := clampMin(m.width-2-8-30-8, 10)
	preview := dimStyle.Render(truncRunes(singleLine(d.LastMsg), previewWidth))

	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	line := prefix + chip + "  " + titleCol + preview
	if d.Unread > 0 {
		line += "  " + unreadStyle.Render(strconv.Itoa(d.Unread))
	}
	if selected {
		return selBg.Render(line)
	}
	return line
}

func dialogChip(kind app.DialogKind) string {
	switch kind {
	case app.KindGroup:
		return chipGroup.Render("Group  ")
	case app.KindChannel:
		return chipChannel.Render("Channel")
	default:
		return chipDM.Render("DM     ")
	}
}

func titleOrFallback(t string) string {
	if t == "" {
		return "(no title)"
	}
	return t
}

// ----- Chat view ----------------------------------------------------------

// viewChat builds the chat screen as exactly `height` lines:
//   header(1) + body(height-3) + input(1) + status(1)
// The body is always padded to its full height, so the input and status line
// stay pinned to the bottom and never jump as content changes.
func (m Model) viewChat() string {
	lines := make([]string, 0, m.heightOrDefault())
	lines = append(lines, m.viewChatHeader())
	lines = append(lines, m.viewChatBody()...)
	lines = append(lines, m.viewChatInput())
	lines = append(lines, m.statusLine())
	return strings.Join(lines, "\n")
}

func (m Model) viewChatHeader() string {
	title := "(chat)"
	rawPrefix := ""
	if m.selected != nil {
		title = m.selected.Title
		rawPrefix = "[" + string(m.selected.Kind) + "] "
	}
	// Truncate so the header is always a single terminal line.
	avail := clampMin(m.widthOrDefault()-len(rawPrefix), 1)
	title = truncRunes(title, avail)

	prefix := ""
	if rawPrefix != "" {
		prefix = dimStyle.Render(rawPrefix)
	}
	return prefix + chatTitleStyle.Render(title)
}

// viewChatBody returns exactly chatBodyHeight lines for the message area.
func (m Model) viewChatBody() []string {
	if m.loadingMsg {
		return m.padBody([]string{m.spin.View() + " Loading messages..."})
	}
	if len(m.messages) == 0 {
		return m.padBody([]string{dimStyle.Render("(empty)")})
	}
	return m.chatViewport()
}

func (m Model) chatAndSelfName() (string, string) {
	chat := "(chat)"
	if m.selected != nil {
		chat = m.selected.Title
	}
	self := m.self.DisplayName()
	if self == "you" {
		self = "You"
	}
	return chat, self
}

func (m Model) viewChatInput() string {
	if m.sending {
		return m.spin.View() + " Sending..."
	}
	if m.vimMode == app.ModeEdit {
		return m.msgInput.View()
	}
	val := m.msgInput.Value()
	if val == "" {
		return dimStyle.Render("(press 'a' to type)")
	}
	return dimStyle.Render(truncRunes("> "+val, m.widthOrDefault()))
}

// ----- Status line --------------------------------------------------------

// statusLine is the single bottom line carrying the mode badge. Like vim, it
// shows no keybinding hints — only the mode, plus the command buffer while
// typing a ":" command, or an error when one occurred.
func (m Model) statusLine() string {
	badge := m.renderModeBadge()
	switch {
	case m.vimMode == app.ModeCommand:
		return badge + "  " + cmdLineStyle.Render(":"+m.cmdBuf+"█")
	case m.err != nil:
		return badge + "  " + errorStyle.Render("Error: "+m.err.Error())
	default:
		return badge
	}
}

func (m Model) renderModeBadge() string {
	switch m.vimMode {
	case app.ModeEdit:
		return modeEditStyle.Render(m.vimMode.Label())
	case app.ModeCommand:
		return modeCommandStyle.Render(m.vimMode.Label())
	default:
		return modeVisualStyle.Render(m.vimMode.Label())
	}
}

// ----- Utilities used by views --------------------------------------------

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(s)
}
