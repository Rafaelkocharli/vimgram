package ui

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"vimgram/internal/app"
	"vimgram/internal/telegram"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.msgInput.Width = clampMin(m.width-4, 20)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case needPhoneMsg:
		m.authState = app.AuthNeedPhone
		m.focusAuthInput("+79001234567", false)
		return m, textinput.Blink

	case needCodeMsg:
		m.authState = app.AuthNeedCode
		m.focusAuthInput("12345", false)
		return m, textinput.Blink

	case needPasswordMsg:
		m.authState = app.AuthNeedPassword
		m.focusAuthInput("2FA password", true)
		return m, textinput.Blink

	case telegramEventMsg:
		return m.handleTelegramEvent(msg.Event)
	}

	return m.tickWidgets(msg)
}

// tickWidgets pipes any unhandled message to the spinner / focused inputs.
func (m Model) tickWidgets(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.spin, cmd = m.spin.Update(msg)
	cmds = append(cmds, cmd)

	if !m.authed && m.authInput.Focused() {
		m.authInput, cmd = m.authInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.authed && m.activeBuffer().kind == bufChat && m.msgInput.Focused() {
		m.msgInput, cmd = m.msgInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) focusAuthInput(placeholder string, password bool) {
	m.authInput.Placeholder = placeholder
	if password {
		m.authInput.EchoMode = textinput.EchoPassword
	} else {
		m.authInput.EchoMode = textinput.EchoNormal
	}
	m.authInput.SetValue("")
	m.authInput.Focus()
}

// ----- Key routing --------------------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.authed {
		return m.updateAuth(msg)
	}
	// Any key dismisses the :ls overlay.
	if len(m.overlay) > 0 {
		m.overlay = nil
		return m, nil
	}
	if m.activeBuffer().kind == bufChatList {
		return m.updateChatList(msg)
	}
	return m.updateChat(msg)
}

// ----- Auth screen --------------------------------------------------------

func (m Model) updateAuth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancel()
		return m, tea.Quit
	case "enter":
		return m.submitAuthInput()
	}
	var cmd tea.Cmd
	m.authInput, cmd = m.authInput.Update(msg)
	return m, cmd
}

func (m Model) submitAuthInput() (tea.Model, tea.Cmd) {
	if !isAuthPromptActive(m.authState) {
		return m, nil
	}
	val := strings.TrimSpace(m.authInput.Value())
	if val == "" {
		return m, nil
	}
	m.authInput.SetValue("")
	m.authInput.Blur()
	m.authState = app.AuthLoading
	answers := m.promptAnswers
	return m, func() tea.Msg {
		answers <- val
		return nil
	}
}

func isAuthPromptActive(s app.AuthState) bool {
	return s == app.AuthNeedPhone || s == app.AuthNeedCode || s == app.AuthNeedPassword
}

// ----- Chat list screen ---------------------------------------------------

func (m Model) updateChatList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.vimMode == app.ModeCommand {
		return m.updateCommandMode(msg)
	}
	w := m.activeWindow()
	switch msg.String() {
	case ":":
		m.enterCommandMode()
		return m, nil
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "g", "home":
		w.cursor = 0
		w.listOffset = 0
	case "G", "end":
		w.cursor = len(m.dialogs) - 1
		m.adjustListOffset()
	case "enter":
		return m.openSelectedChat()
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	w := m.activeWindow()
	next := w.cursor + delta
	if next < 0 || next >= len(m.dialogs) {
		return
	}
	w.cursor = next
	m.adjustListOffset()
}

func (m Model) openSelectedChat() (tea.Model, tea.Cmd) {
	if len(m.dialogs) == 0 {
		return m, nil
	}
	d := m.dialogs[m.activeWindow().cursor]

	// Reuse the buffer if this chat is already loaded — switch instantly.
	if existing := m.buffers.findByPeer(d.Key); existing != nil {
		m.switchBuffer(existing.id)
		return m, nil
	}

	// Create a fresh chat buffer and kick off the initial history fetch.
	b := m.buffers.addChat(d)
	b.loadingMsg = true
	m.switchBuffer(b.id)
	client := m.client
	want := m.chatBodyHeight() + 1
	peer := d.Peer
	return m, func() tea.Msg {
		client.OpenChat(peer, want)
		return nil
	}
}

// ----- Chat view ----------------------------------------------------------

func (m Model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.vimMode {
	case app.ModeCommand:
		return m.updateCommandMode(msg)
	case app.ModeEdit:
		return m.updateChatEdit(msg)
	default:
		return m.updateChatVisual(msg)
	}
}

func (m Model) updateChatVisual(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	height := m.chatBodyHeight()
	maxOffset := m.maxLineOffset()
	w := m.activeWindow()

	switch msg.String() {
	case ":":
		m.enterCommandMode()
		return m, nil
	case "a", "i":
		m.vimMode = app.ModeEdit
		m.msgInput.Focus()
		return m, textinput.Blink
	case "pgup", "ctrl+u":
		w.lineOffset += height / 2
		if w.lineOffset >= maxOffset {
			w.lineOffset = maxOffset
			if cmd := m.maybeLoadOlder(); cmd != nil {
				return m, cmd
			}
		}
	case "pgdown", "ctrl+d":
		w.lineOffset = clampMin(w.lineOffset-height/2, 0)
	case "k", "up":
		if w.lineOffset < maxOffset {
			w.lineOffset++
		} else if cmd := m.maybeLoadOlder(); cmd != nil {
			return m, cmd
		}
	case "j", "down":
		if w.lineOffset > 0 {
			w.lineOffset--
		}
	case "g", "home":
		w.lineOffset = maxOffset
		if cmd := m.maybeLoadOlder(); cmd != nil {
			return m, cmd
		}
	case "G", "end":
		w.lineOffset = 0
	}
	return m, nil
}

func (m *Model) maybeLoadOlder() tea.Cmd {
	b := m.activeBuffer()
	if !b.hasMore || b.loadingMore || len(b.messages) == 0 {
		return nil
	}
	b.loadingMore = true
	oldestID := b.messages[0].ID
	peer := b.peer
	client := m.client
	return func() tea.Msg {
		client.LoadMore(peer, oldestID)
		return nil
	}
}

func (m Model) updateChatEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.vimMode = app.ModeVisual
		m.msgInput.Blur()
		return m, nil
	case "enter":
		return m.submitMessage()
	}
	var cmd tea.Cmd
	m.msgInput, cmd = m.msgInput.Update(msg)
	return m, cmd
}

func (m Model) submitMessage() (tea.Model, tea.Cmd) {
	b := m.activeBuffer()
	text := strings.TrimSpace(m.msgInput.Value())
	if text == "" || b.sending {
		return m, nil
	}
	m.msgInput.SetValue("")
	b.sending = true
	m.activeWindow().lineOffset = 0
	peer := b.peer
	client := m.client
	return m, func() tea.Msg {
		client.SendMessage(peer, text)
		return nil
	}
}

// ----- Command mode (":") -------------------------------------------------

func (m *Model) enterCommandMode() {
	m.vimMode = app.ModeCommand
	m.cmdBuf = ""
	m.err = nil
}

func (m Model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.exitCommandMode("")
		return m, nil
	case "enter":
		cmd := strings.TrimSpace(m.cmdBuf)
		m.exitCommandMode("")
		return m.executeCommand(cmd)
	case "backspace":
		m.cmdBuf = trimLastRune(m.cmdBuf)
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.cmdBuf += string(msg.Runes)
	}
	return m, nil
}

func (m *Model) exitCommandMode(buf string) {
	m.vimMode = app.ModeVisual
	m.cmdBuf = buf
}

func (m Model) executeCommand(raw string) (tea.Model, tea.Cmd) {
	cmd := app.ParseCommand(raw)
	switch cmd.Kind {
	case app.CmdQuit, app.CmdQuitForce:
		// MVP: a single window, so :q behaves like quit. Phase 2 will close
		// the focused window when more than one exists.
		m.cancel()
		return m, tea.Quit
	case app.CmdBuffers:
		m.overlay = m.bufferListLines()
		return m, nil
	case app.CmdBufferSwitch:
		return m.switchByID(parseID(cmd.Arg)), nil
	case app.CmdBufferAlt:
		if m.win.altBuffer != 0 && m.buffers.find(m.win.altBuffer) != nil {
			m.switchBuffer(m.win.altBuffer)
		}
		return m, nil
	case app.CmdBufferNext:
		m.switchBuffer(m.buffers.next(m.win.bufferID))
		return m, nil
	case app.CmdBufferPrev:
		m.switchBuffer(m.buffers.prev(m.win.bufferID))
		return m, nil
	case app.CmdBufferDelete:
		id := m.win.bufferID
		if cmd.HasArg {
			id = parseID(cmd.Arg)
		}
		return m.deleteBuffer(id), nil
	}
	if raw != "" {
		m.err = &unknownCmdError{cmd: raw}
	}
	return m, nil
}

func (m Model) switchByID(id int) Model {
	if id <= 0 || m.buffers.find(id) == nil {
		m.err = &unknownCmdError{cmd: "b " + itoa(id)}
		return m
	}
	m.switchBuffer(id)
	return m
}

// switchBuffer points the active window at buffer id, preserving the draft of
// the buffer we leave and restoring the target's draft.
func (m *Model) switchBuffer(id int) {
	if id == m.win.bufferID || m.buffers.find(id) == nil {
		return
	}
	// Save the current chat buffer's draft.
	if cur := m.activeBuffer(); cur != nil && cur.kind == bufChat {
		cur.draft = m.msgInput.Value()
	}
	m.win.altBuffer = m.win.bufferID
	m.win.bufferID = id
	// Reset this window's viewport for the new buffer.
	m.win.lineOffset = 0
	m.win.cursor = 0
	m.win.listOffset = 0
	m.vimMode = app.ModeVisual
	m.err = nil

	if b := m.activeBuffer(); b != nil && b.kind == bufChat {
		m.msgInput.SetValue(b.draft)
	} else {
		m.msgInput.SetValue("")
	}
	m.msgInput.Blur()
}

// deleteBuffer removes a buffer (never the chat list). If it is the one shown
// in the window, the window falls back to the alternate buffer or the chat
// list. The Telegram chat itself is untouched.
func (m Model) deleteBuffer(id int) Model {
	if id == chatListBufferID {
		m.err = &unknownCmdError{cmd: "cannot delete the Chats buffer"}
		return m
	}
	if m.buffers.find(id) == nil {
		m.err = &unknownCmdError{cmd: "bd " + itoa(id)}
		return m
	}

	visible := m.win.bufferID == id
	fallback := m.win.altBuffer
	if fallback == id || m.buffers.find(fallback) == nil {
		fallback = chatListBufferID
	}

	m.buffers.delete(id)
	if m.win.altBuffer == id {
		m.win.altBuffer = 0
	}
	if visible {
		// Switch directly (avoid switchBuffer saving a now-deleted draft).
		m.win.bufferID = fallback
		m.win.lineOffset, m.win.cursor, m.win.listOffset = 0, 0, 0
		m.vimMode = app.ModeVisual
		if b := m.activeBuffer(); b != nil && b.kind == bufChat {
			m.msgInput.SetValue(b.draft)
		} else {
			m.msgInput.SetValue("")
		}
		m.msgInput.Blur()
	}
	return m
}

// ----- Telegram events ----------------------------------------------------

func (m Model) handleTelegramEvent(e telegram.Event) (tea.Model, tea.Cmd) {
	switch ev := e.(type) {
	case telegram.EventConnected:
		m.authState = app.AuthLoading
		return m, nil
	case telegram.EventDialogsLoaded:
		m.self = ev.Self
		m.dialogs = ev.Dialogs
		m.authed = true
		m.win.bufferID = chatListBufferID
		m.win.cursor = 0
		m.win.listOffset = 0
		m.authInput.Blur()
		m.seedStatuses(ev.Dialogs)
		return m, nil
	case telegram.EventMessagesLoaded:
		if b := m.buffers.findByPeer(ev.PeerKey); b != nil {
			b.messages = ev.Messages
			b.hasMore = ev.HasMore
			b.loadingMsg = false
			b.msgVersion++
		}
		return m, nil
	case telegram.EventMessagesPrepended:
		return m.prependMessages(ev.PeerKey, ev.Messages, ev.HasMore), nil
	case telegram.EventMessageSent:
		if b := m.buffers.findByPeer(ev.PeerKey); b != nil {
			b.sending = false
			if ev.Err != nil {
				m.err = ev.Err
				return m, nil
			}
			b.messages = append(b.messages, ev.Message)
			b.msgVersion++
		}
		return m, nil
	case telegram.EventMessageReceived:
		return m.onIncomingMessage(ev.PeerKey, ev.Message), nil
	case telegram.EventUserStatus:
		if ev.Status != app.StatusUnknown {
			m.statuses[ev.UserID] = ev.Status
		}
		delete(m.typingUntil, ev.UserID)
		return m, nil
	case telegram.EventUserTyping:
		m.typingUntil[ev.UserID] = time.Now().Add(typingTTL)
		return m, nil
	case telegram.EventError:
		m.err = ev.Err
		return m, nil
	}
	return m, nil
}

// typingTTL is how long a "typing..." indicator stays lit after the last
// typing update from a user. Telegram resends typing actions every few
// seconds while the user keeps composing.
const typingTTL = 6 * time.Second

// seedStatuses primes the presence map from the initial dialog snapshot.
func (m *Model) seedStatuses(dialogs []app.Dialog) {
	for _, d := range dialogs {
		if d.UserID != 0 && d.Status != app.StatusUnknown {
			m.statuses[d.UserID] = d.Status
		}
	}
}

func (m Model) prependMessages(peerKey string, prefix []app.Message, hasMore bool) Model {
	b := m.buffers.findByPeer(peerKey)
	if b == nil {
		return m
	}
	b.loadingMore = false
	b.hasMore = hasMore
	if len(prefix) == 0 {
		return m
	}
	// The viewport is bottom-anchored, so prepending older messages does not
	// shift what the user is looking at — no offset change is needed.
	b.messages = append(prefix, b.messages...)
	b.msgVersion++
	// Clamp the active window only if it shows this buffer.
	if m.win.bufferID == b.id {
		if max := m.maxLineOffset(); m.win.lineOffset > max {
			m.win.lineOffset = max
		}
	}
	return m
}

func (m Model) onIncomingMessage(peerKey string, msg app.Message) Model {
	buf := m.buffers.findByPeer(peerKey)
	isActiveChat := buf != nil && m.win.bufferID == buf.id

	// 1) append to the chat buffer (loaded but maybe not visible)
	if buf != nil {
		wasAtBottom := isActiveChat && m.win.lineOffset == 0
		addedLines := 0
		if isActiveChat && !wasAtBottom {
			addedLines = m.renderedLineCount(msg)
		}
		buf.messages = append(buf.messages, msg)
		buf.msgVersion++
		// Keep a scrolled-up active window stable.
		if isActiveChat && !wasAtBottom {
			m.win.lineOffset += addedLines
		}
	}

	// 2) update the dialog preview / unread counter
	if idx := indexOfDialog(m.dialogs, peerKey); idx >= 0 {
		m.dialogs[idx].LastMsg = msg.Text
		m.dialogs[idx].LastDate = int(msg.Date.Unix())
		if msg.Out || isActiveChat {
			m.dialogs[idx].Unread = 0
		} else {
			m.dialogs[idx].Unread++
		}
		m = resortDialogsKeepingCursor(m)
	}
	return m
}

// renderedLineCount returns how many visual lines a message occupies at the
// current width.
func (m Model) renderedLineCount(msg app.Message) int {
	chatName, selfName := m.chatAndSelfName()
	return strings.Count(renderMessage(msg, chatName, selfName, m.width), "\n") + 1
}

func indexOfDialog(dialogs []app.Dialog, key string) int {
	for i := range dialogs {
		if dialogs[i].Key == key {
			return i
		}
	}
	return -1
}

// resortDialogsKeepingCursor re-sorts dialogs by recency while keeping the
// active window's chat-list cursor on the same dialog.
func resortDialogsKeepingCursor(m Model) Model {
	w := m.activeWindow()
	var cursorKey string
	if w.cursor < len(m.dialogs) {
		cursorKey = m.dialogs[w.cursor].Key
	}
	sort.SliceStable(m.dialogs, func(i, j int) bool {
		return m.dialogs[i].LastDate > m.dialogs[j].LastDate
	})
	if cursorKey != "" {
		for i := range m.dialogs {
			if m.dialogs[i].Key == cursorKey {
				w.cursor = i
				m.adjustListOffset()
				break
			}
		}
	}
	return m
}

// ----- helpers ------------------------------------------------------------

type unknownCmdError struct{ cmd string }

func (e *unknownCmdError) Error() string { return "unknown command: :" + e.cmd }

func clampMin(v, min int) int {
	if v < min {
		return min
	}
	return v
}

func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(r[:len(r)-1])
}

func parseID(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

func itoa(n int) string { return strconv.Itoa(n) }

