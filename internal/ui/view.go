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
	b.WriteString(m.viewStatusLine(chatListHints))
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}
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

func (m Model) viewChat() string {
	var b strings.Builder
	b.WriteString(m.viewChatHeader())
	b.WriteString("\n")
	b.WriteString(m.viewChatBody())
	b.WriteString("\n")
	b.WriteString(m.viewChatInput())
	b.WriteString("\n")
	b.WriteString(m.viewStatusLine(m.chatHints()))
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}
	return b.String()
}

func (m Model) viewChatHeader() string {
	title := "(chat)"
	kind := ""
	if m.selected != nil {
		title = m.selected.Title
		kind = string(m.selected.Kind)
	}
	prefix := ""
	if kind != "" {
		prefix = dimStyle.Render("[" + kind + "] ")
	}
	return prefix + titleStyle.Render(title)
}

func (m Model) viewChatBody() string {
	if m.loadingMsg {
		return m.spin.View() + " Loading messages..."
	}
	if len(m.messages) == 0 {
		return dimStyle.Render("(empty)")
	}
	chatName, selfName := m.chatAndSelfName()
	stack, start, end := m.windowMessages(chatName, selfName)

	var b strings.Builder
	b.WriteString(m.viewAboveHint(start))
	for _, r := range stack {
		b.WriteString(r)
		b.WriteString("\n")
	}
	b.WriteString(m.viewBelowHint(end))
	return strings.TrimRight(b.String(), "\n")
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

// windowMessages renders messages starting from the latest visible one,
// walking backwards until the visible-line budget is exhausted. Returns the
// rendered slice (in chronological order), the first visible index, and the
// last visible index (exclusive).
func (m Model) windowMessages(chatName, selfName string) ([]string, int, int) {
	maxLines := m.visibleMessages()
	end := len(m.messages) - m.msgOffset
	if end > len(m.messages) {
		end = len(m.messages)
	}
	if end < 0 {
		end = 0
	}

	var rendered []string
	used := 0
	start := end
	for i := end - 1; i >= 0; i-- {
		r := renderMessage(m.messages[i], chatName, selfName, m.width)
		n := strings.Count(r, "\n") + 1
		if used+n > maxLines && len(rendered) > 0 {
			break
		}
		rendered = append([]string{r}, rendered...)
		used += n
		start = i
	}
	return rendered, start, end
}

func (m Model) viewAboveHint(start int) string {
	if start > 0 {
		hint := fmt.Sprintf("↑ %d more above", start)
		if m.hasMore {
			hint += " (k/PgUp — load older)"
		}
		return dimStyle.Render(hint) + "\n"
	}
	if m.loadingMore {
		return m.spin.View() + dimStyle.Render(" loading older messages...") + "\n"
	}
	if !m.hasMore && len(m.messages) > 0 {
		return dimStyle.Render("— start of chat —") + "\n"
	}
	return ""
}

func (m Model) viewBelowHint(end int) string {
	if end >= len(m.messages) {
		return ""
	}
	return dimStyle.Render(fmt.Sprintf("↓ %d more below (G — bottom)", len(m.messages)-end)) + "\n"
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
	return dimStyle.Render("> " + val)
}

func (m Model) chatHints() string {
	if m.vimMode == app.ModeEdit {
		return "enter — send · esc — visual"
	}
	return "a — insert · j/k pgup/pgdn — scroll · g/G — edge · : — command (:q back, :wq/:qa quit)"
}

const chatListHints = "j/k — move · g/G — edge · enter — open · : — command (:q :wq :qa)"

// ----- Status line (mode badge + hints / command line) -------------------

func (m Model) viewStatusLine(hints string) string {
	badge := m.renderModeBadge()
	tail := footerStyle.Render(hints)
	if m.vimMode == app.ModeCommand {
		tail = cmdLineStyle.Render(":" + m.cmdBuf + "█")
	}
	return badge + "  " + tail
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
