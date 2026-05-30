package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"vimgram/internal/app"
)

// View implements tea.Model.
func (m Model) View() string {
	if !m.authed {
		return m.viewAuth()
	}
	if len(m.overlay) > 0 {
		return m.viewOverlay()
	}
	return m.viewWindow()
}

// viewWindow renders the active window's buffer full-screen (MVP: one window).
func (m Model) viewWindow() string {
	if m.activeBuffer().kind == bufChatList {
		return m.viewChatListBuffer()
	}
	return m.viewChatBuffer()
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

// ----- :ls overlay --------------------------------------------------------

func (m Model) viewOverlay() string {
	lines := make([]string, 0, m.heightOrDefault())
	lines = append(lines, chatTitleStyle.Render(":buffers"))
	lines = append(lines, m.overlay...)
	// pad to height-1, then footer
	for len(lines) < m.heightOrDefault()-1 {
		lines = append(lines, "")
	}
	lines = append(lines, footerStyle.Render("press any key to close"))
	if len(lines) > m.heightOrDefault() {
		lines = lines[:m.heightOrDefault()]
	}
	return strings.Join(lines, "\n")
}

// bufferListLines renders the :ls table for the current window.
func (m Model) bufferListLines() []string {
	w := m.activeWindow()
	out := make([]string, 0, len(m.buffers.list))
	for _, b := range m.buffers.list {
		flags := []byte("   ") // [current/alt][active][space]
		if b.id == w.bufferID {
			flags[0] = '%'
			flags[1] = 'a'
		} else if b.id == w.altBuffer {
			flags[0] = '#'
		}
		out = append(out, fmt.Sprintf("%3d %s %s", b.id, string(flags), b.name))
	}
	return out
}

// ----- Chat list buffer ---------------------------------------------------

// viewChatListBuffer renders exactly `height` lines: header(1) + body + status(1).
func (m Model) viewChatListBuffer() string {
	lines := make([]string, 0, m.heightOrDefault())
	lines = append(lines, m.viewChatListHeader())
	lines = append(lines, m.viewChatListBody()...)
	lines = append(lines, m.statusLine())
	return strings.Join(lines, "\n")
}

func (m Model) viewChatListHeader() string {
	header := fmt.Sprintf("Chats · %s", m.self.DisplayName())
	if len(m.dialogs) > 0 {
		header += fmt.Sprintf("  (%d/%d)", m.activeWindow().cursor+1, len(m.dialogs))
	}
	return chatTitleStyle.Render(truncRunes(header, m.widthOrDefault()))
}

// viewChatListBody returns exactly visibleRows() lines, top-anchored.
func (m Model) viewChatListBody() []string {
	rows := m.visibleRows()
	w := m.activeWindow()
	out := make([]string, 0, rows)

	if len(m.dialogs) == 0 {
		out = append(out, dimStyle.Render("No chats"))
	} else {
		end := w.listOffset + rows
		if end > len(m.dialogs) {
			end = len(m.dialogs)
		}
		for i := w.listOffset; i < end; i++ {
			out = append(out, m.renderDialogRow(i, i == w.cursor))
		}
	}

	for len(out) < rows {
		out = append(out, "")
	}
	return out[:rows]
}

func (m Model) renderDialogRow(idx int, selected bool) string {
	d := m.dialogs[idx]
	chip := dialogChip(d.Kind)
	title := titleOrFallback(d.Title)
	titleCol := dialogTitleStyle.Render(truncRunes(title, 28))

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

// ----- Chat buffer --------------------------------------------------------

// viewChatBuffer builds the chat screen as exactly `height` lines:
//
//	header(1) + body(height-3) + input(1) + status(1)
func (m Model) viewChatBuffer() string {
	lines := make([]string, 0, m.heightOrDefault())
	lines = append(lines, m.viewChatHeader())
	lines = append(lines, m.viewChatBody()...)
	lines = append(lines, m.viewChatInput())
	lines = append(lines, m.statusLine())
	return strings.Join(lines, "\n")
}

func (m Model) viewChatHeader() string {
	b := m.activeBuffer()
	rawPrefix := "[" + string(b.kindLabel) + "] "

	statusRaw, statusStyled := m.chatStatusLabel()

	// Truncate the title so the whole header stays on a single terminal line.
	avail := clampMin(m.widthOrDefault()-len(rawPrefix)-len(statusRaw), 1)
	title := truncRunes(b.name, avail)

	return dimStyle.Render(rawPrefix) + chatTitleStyle.Render(title) + statusStyled
}

// chatStatusLabel returns the presence label for the active DM buffer, both as
// raw text (for width math) and styled (for display).
func (m Model) chatStatusLabel() (raw, styled string) {
	b := m.activeBuffer()
	if b.kindLabel != app.KindDM || b.userID == 0 {
		return "", ""
	}
	uid := b.userID

	if until, ok := m.typingUntil[uid]; ok && time.Now().Before(until) {
		raw = " (typing...)"
		return raw, statusTypingStyle.Render(raw)
	}

	switch m.statuses[uid] {
	case app.StatusOnline:
		raw = " (online)"
		return raw, statusOnlineStyle.Render(raw)
	case app.StatusOffline:
		raw = " (offline)"
		return raw, statusOfflineStyle.Render(raw)
	default:
		return "", ""
	}
}

// viewChatBody returns exactly chatBodyHeight lines for the message area.
func (m Model) viewChatBody() []string {
	b := m.activeBuffer()
	if b.loadingMsg {
		return m.padBody([]string{m.spin.View() + " Loading messages..."})
	}
	if len(b.messages) == 0 {
		return m.padBody([]string{dimStyle.Render("(empty)")})
	}
	return m.chatViewport()
}

func (m Model) chatAndSelfName() (string, string) {
	chat := m.activeBuffer().name
	self := m.self.DisplayName()
	if self == "you" {
		self = "You"
	}
	return chat, self
}

func (m Model) viewChatInput() string {
	b := m.activeBuffer()
	if b.sending {
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
