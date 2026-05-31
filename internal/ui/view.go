package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

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
	return m.viewWindows()
}

// ----- Window compositor --------------------------------------------------

// viewWindows lays the windows out left-to-right (vertical splits), then pins
// a single global status line at the bottom. Total output is exactly `height`
// rows, so the status line never moves.
func (m Model) viewWindows() string {
	n := len(m.wins)
	widths := splitWidths(m.widthOrDefault(), n)
	innerHeight := m.heightOrDefault() - 1

	cols := make([][]string, n)
	for i, w := range m.wins {
		inner := clampMin(widths[i]-1, 1) // -1 for the focus/separator bar
		w.width = inner                   // cache for scroll math during key handling
		rows := m.renderWindowRows(w, inner, i == m.focused)
		bar := windowBar(i == m.focused)
		clamp := lipgloss.NewStyle().MaxWidth(inner)
		for r := range rows {
			rows[r] = bar + padTo(clamp.Render(rows[r]), inner)
		}
		cols[i] = rows
	}

	var b strings.Builder
	for r := 0; r < innerHeight; r++ {
		for i := 0; i < n; i++ {
			b.WriteString(cols[i][r])
		}
		b.WriteString("\n")
	}
	b.WriteString(m.statusLine())
	return b.String()
}

// splitWidths divides total columns among n windows as evenly as possible,
// giving the remainder to the leftmost windows.
func splitWidths(total, n int) []int {
	if n <= 0 {
		return nil
	}
	base := total / n
	rem := total % n
	out := make([]int, n)
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}

func windowBar(focused bool) string {
	if focused {
		return focusBarStyle.Render("┃")
	}
	return dimBarStyle.Render("│")
}

// padTo pads s with spaces to exactly w visible columns (ANSI-aware).
func padTo(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// renderWindowRows renders one window's content to exactly height-1 rows.
func (m Model) renderWindowRows(w *window, width int, focused bool) []string {
	switch m.bufferOf(w).kind {
	case bufChatList:
		return m.renderListWindow(w, width, focused)
	case bufHelp:
		return m.renderHelpWindow(w, width, focused)
	default:
		return m.renderChatWindow(w, width, focused)
	}
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
	for len(lines) < m.heightOrDefault()-1 {
		lines = append(lines, "")
	}
	lines = append(lines, footerStyle.Render("press any key to close"))
	if len(lines) > m.heightOrDefault() {
		lines = lines[:m.heightOrDefault()]
	}
	return strings.Join(lines, "\n")
}

// bufferListLines renders the :ls table for the focused window.
func (m Model) bufferListLines() []string {
	w := m.activeWindow()
	out := make([]string, 0, len(m.buffers.list))
	for _, b := range m.buffers.list {
		flags := []byte("   ")
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

// ----- Chat list window ---------------------------------------------------

func (m Model) renderListWindow(w *window, width int, focused bool) []string {
	rows := make([]string, 0, m.heightOrDefault()-1)
	rows = append(rows, m.listHeader(w, width, focused))
	rows = append(rows, m.listBody(w, width)...)
	return rows
}

// visibleDialogs returns the subset of dialogs that should be shown given the
// current showArchive setting.
func (m Model) visibleDialogs() []app.Dialog {
	if m.showArchive {
		return m.dialogs
	}
	out := make([]app.Dialog, 0, len(m.dialogs))
	for _, d := range m.dialogs {
		if !d.Archived {
			out = append(out, d)
		}
	}
	return out
}

func (m Model) listHeader(w *window, width int, focused bool) string {
	visible := m.visibleDialogs()
	header := fmt.Sprintf("Chats · %s", m.self.DisplayName())
	if len(visible) > 0 {
		header += fmt.Sprintf("  (%d/%d)", w.cursor+1, len(visible))
	}
	if m.showArchive {
		header += dimStyle.Render(" [+archive]")
	}
	return titleStyleFor(focused).Render(truncRunes(header, width))
}

func (m Model) listBody(w *window, width int) []string {
	rows := m.visibleRows()
	out := make([]string, 0, rows)
	visible := m.visibleDialogs()

	if len(visible) == 0 {
		out = append(out, dimStyle.Render("No chats"))
	} else {
		end := w.listOffset + rows
		if end > len(visible) {
			end = len(visible)
		}
		for i := w.listOffset; i < end; i++ {
			out = append(out, m.renderDialogRow(visible, i, i == w.cursor, width))
		}
	}
	for len(out) < rows {
		out = append(out, "")
	}
	return out[:rows]
}

func (m Model) renderDialogRow(dialogs []app.Dialog, idx int, selected bool, width int) string {
	d := dialogs[idx]
	chip := dialogChip(d.Kind)
	titleCol := dialogTitleStyle.Render(truncRunes(titleOrFallback(d.Title), 28))

	previewWidth := clampMin(width-2-8-30-8, 8)
	preview := dimStyle.Render(truncRunes(singleLine(d.LastMsg), previewWidth))

	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	left := prefix + chip + "  " + titleCol + preview

	// Right-align the unread badge to the window's right edge so badges line
	// up in a column regardless of how short the message preview is.
	if d.Unread > 0 {
		badge := unreadBadge(d.Unread)
		pad := width - lipgloss.Width(left) - lipgloss.Width(badge)
		if pad < 1 {
			pad = 1
		}
		left += strings.Repeat(" ", pad) + badge
	}
	if selected {
		return selBg.Render(left)
	}
	return left
}

// unreadBadge renders the unread counter as an exactly-3-character badge on a
// turquoise background. The number is centred with any extra space biased to
// the left (" 5 ", " 12", "345"); counts above 999 are capped at "999".
func unreadBadge(count int) string {
	if count > 999 {
		count = 999
	}
	s := strconv.Itoa(count)
	pad := 3 - len(s)
	if pad < 0 {
		pad = 0
	}
	leftPad := pad - pad/2
	rightPad := pad / 2
	return unreadStyle.Render(strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad))
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

// ----- Chat window --------------------------------------------------------

func (m Model) renderHelpWindow(w *window, width int, focused bool) []string {
	b := m.bufferOf(w)
	rows := make([]string, 0, m.heightOrDefault()-1)
	rows = append(rows, titleStyleFor(focused).Render(truncRunes("help", width)))
	rows = append(rows, m.chatBody(b, w, width)...)
	return rows
}

func (m Model) renderChatWindow(w *window, width int, focused bool) []string {
	b := m.bufferOf(w)
	rows := make([]string, 0, m.heightOrDefault()-1)
	rows = append(rows, m.chatHeader(b, width, focused))
	rows = append(rows, m.chatBody(b, w, width)...)
	if b.replyToID != 0 {
		rows = append(rows, m.chatReplyLine(b, width))
	} else if b.editMsgID != 0 {
		rows = append(rows, m.chatEditLine(b, width))
	}
	rows = append(rows, m.chatInputLine(b, focused))
	return rows
}

// chatReplyLine renders the one-line reply banner shown above the compose input.
func (m Model) chatReplyLine(b *buffer, width int) string {
	preview := b.replyToPreview
	if preview == "" {
		preview = "..."
	}
	label := fmt.Sprintf(`↩ replying to "%s"`, preview)
	return dimStyle.Render(truncRunes(label, width))
}

// chatEditLine renders the one-line edit banner shown above the compose input.
func (m Model) chatEditLine(b *buffer, width int) string {
	preview := b.editOrigText
	if preview == "" {
		preview = "..."
	}
	label := fmt.Sprintf(`✏ editing: "%s"`, preview)
	return dimStyle.Render(truncRunes(label, width))
}

func (m Model) chatHeader(b *buffer, width int, focused bool) string {
	rawPrefix := "[" + string(b.kindLabel) + "] "
	statusRaw, statusStyled := m.chatStatusLabel(b)
	avail := clampMin(width-len(rawPrefix)-len(statusRaw), 1)
	title := truncRunes(b.name, avail)
	return dimStyle.Render(rawPrefix) + titleStyleFor(focused).Render(title) + statusStyled
}

// chatStatusLabel returns the presence label for a DM buffer.
func (m Model) chatStatusLabel(b *buffer) (raw, styled string) {
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

// chatBody returns exactly chatBodyHeight lines for the window's message area.
func (m Model) chatBody(b *buffer, w *window, width int) []string {
	if b.loadingMsg {
		return m.padBody([]string{m.spin.View() + " Loading messages..."})
	}
	if len(m.chatLines(b, width)) == 0 {
		return m.padBody([]string{dimStyle.Render("(empty)")})
	}
	return m.chatViewport(b, w, width)
}

// chatNames returns the chat title and the user's own display name for a buffer.
func (m Model) chatNames(b *buffer) (string, string) {
	self := m.self.DisplayName()
	if self == "you" {
		self = "You"
	}
	return b.name, self
}

// chatInputLine renders the compose line. Only the focused window shows the
// live input widget; others show their buffer's saved draft.
func (m Model) chatInputLine(b *buffer, focused bool) string {
	if b.sending {
		return m.spin.View() + " Sending..."
	}
	if focused && m.discardPrompt {
		return errorStyle.Render("No write since last change. Discard draft? [y/N]")
	}
	if focused && m.deletePrompt {
		if m.deleteRevoke {
			return errorStyle.Render("Delete message for everyone? [y/N]")
		}
		return errorStyle.Render("Delete message? [y/N]")
	}
	if focused && m.vimMode == app.ModeEdit {
		return m.msgInput.View()
	}
	val := b.draft
	if focused {
		val = m.msgInput.Value()
	}
	if val == "" {
		return dimStyle.Render("(press 'a' to type)")
	}
	return dimStyle.Render(truncRunes("> "+val, m.widthOrDefault()))
}

// ----- Status line --------------------------------------------------------

// statusLine is the single global bottom line: the mode badge, plus the
// command buffer while typing a ":" command, or an error.
func (m Model) statusLine() string {
	badge := m.renderModeBadge()
	switch {
	case m.vimMode == app.ModeCommand:
		return badge + "  " + cmdLineStyle.Render(":"+m.cmdBuf+"█")
	case m.err != nil:
		return badge + "  " + errorStyle.Render("Error: "+m.err.Error())
	case m.authed && m.vimMode == app.ModeNormal && m.activeBuffer().kind == bufChat:
		w := m.activeWindow()
		return badge + "  " + dimStyle.Render(fmt.Sprintf("Col %d", w.colCursor+1))
	default:
		return badge
	}
}

func (m Model) renderModeBadge() string {
	switch m.vimMode {
	case app.ModeVisual:
		return modeVisualStyle.Render(m.vimMode.Label())
	case app.ModeEdit:
		return modeEditStyle.Render(m.vimMode.Label())
	case app.ModeCommand:
		return modeCommandStyle.Render(m.vimMode.Label())
	default:
		return modeNormalStyle.Render(m.vimMode.Label())
	}
}

// ----- Utilities used by views --------------------------------------------

// titleStyleFor highlights a window title when its window is focused.
func titleStyleFor(focused bool) lipgloss.Style {
	if focused {
		return chatTitleStyle
	}
	return dimStyle
}

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
