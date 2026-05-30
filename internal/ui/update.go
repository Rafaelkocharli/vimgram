package ui

import (
	"sort"
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

	if m.screen == app.ScreenAuth && m.authInput.Focused() {
		m.authInput, cmd = m.authInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.screen == app.ScreenChat && m.msgInput.Focused() {
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

// ----- Key routing per screen ---------------------------------------------

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case app.ScreenAuth:
		return m.updateAuth(msg)
	case app.ScreenChatList:
		return m.updateChatList(msg)
	case app.ScreenChat:
		return m.updateChat(msg)
	}
	return m, nil
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
	switch msg.String() {
	case ":":
		m.enterCommandMode()
		return m, nil
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "g", "home":
		m.cursor = 0
		m.listOffset = 0
	case "G", "end":
		m.cursor = len(m.dialogs) - 1
		m.adjustListOffset()
	case "enter":
		return m.openSelectedChat()
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	next := m.cursor + delta
	if next < 0 || next >= len(m.dialogs) {
		return
	}
	m.cursor = next
	m.adjustListOffset()
}

func (m Model) openSelectedChat() (tea.Model, tea.Cmd) {
	if len(m.dialogs) == 0 {
		return m, nil
	}
	d := m.dialogs[m.cursor]
	m.selected = &d
	m.screen = app.ScreenChat
	m.messages = nil
	m.loadingMsg = true
	m.loadingMore = false
	m.hasMore = true
	m.lineOffset = 0
	m.vimMode = app.ModeVisual
	m.msgInput.SetValue("")
	m.msgInput.Blur()
	client := m.client
	return m, func() tea.Msg {
		client.OpenChat(d.Peer)
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

	switch msg.String() {
	case ":":
		m.enterCommandMode()
		return m, nil
	case "a", "i":
		m.vimMode = app.ModeEdit
		m.msgInput.Focus()
		return m, textinput.Blink
	case "pgup", "ctrl+u":
		m.lineOffset += height / 2
		if m.lineOffset >= maxOffset {
			m.lineOffset = maxOffset
			if cmd := m.maybeLoadOlder(); cmd != nil {
				return m, cmd
			}
		}
	case "pgdown", "ctrl+d":
		m.lineOffset = clampMin(m.lineOffset-height/2, 0)
	case "k", "up":
		if m.lineOffset < maxOffset {
			m.lineOffset++
		} else if cmd := m.maybeLoadOlder(); cmd != nil {
			return m, cmd
		}
	case "j", "down":
		if m.lineOffset > 0 {
			m.lineOffset--
		}
	case "g", "home":
		m.lineOffset = maxOffset
		if cmd := m.maybeLoadOlder(); cmd != nil {
			return m, cmd
		}
	case "G", "end":
		m.lineOffset = 0
	}
	return m, nil
}

func (m *Model) maybeLoadOlder() tea.Cmd {
	if !m.hasMore || m.loadingMore || len(m.messages) == 0 || m.selected == nil {
		return nil
	}
	m.loadingMore = true
	oldestID := m.messages[0].ID
	peer := m.selected.Peer
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
	text := strings.TrimSpace(m.msgInput.Value())
	if text == "" || m.sending || m.selected == nil {
		return m, nil
	}
	m.msgInput.SetValue("")
	m.sending = true
	m.lineOffset = 0
	peer := m.selected.Peer
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
	switch cmd {
	case app.CmdQuit:
		if m.screen == app.ScreenChat {
			return m.backToChatList(), nil
		}
		m.cancel()
		return m, tea.Quit
	case app.CmdQuitForce:
		m.cancel()
		return m, tea.Quit
	}
	if raw != "" {
		m.err = &unknownCmdError{cmd: raw}
	}
	return m, nil
}

func (m Model) backToChatList() Model {
	m.screen = app.ScreenChatList
	m.selected = nil
	m.messages = nil
	m.msgInput.Blur()
	m.msgInput.SetValue("")
	m.vimMode = app.ModeVisual
	m.lineOffset = 0
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
		m.screen = app.ScreenChatList
		m.cursor = 0
		m.listOffset = 0
		m.authInput.Blur()
		m.seedStatuses(ev.Dialogs)
		return m, nil
	case telegram.EventMessagesLoaded:
		m.messages = ev.Messages
		m.hasMore = ev.HasMore
		m.loadingMsg = false
		return m, nil
	case telegram.EventMessagesPrepended:
		return m.prependMessages(ev.Messages, ev.HasMore), nil
	case telegram.EventMessageSent:
		m.sending = false
		if ev.Err != nil {
			m.err = ev.Err
			return m, nil
		}
		m.messages = append(m.messages, ev.Message)
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

func (m Model) prependMessages(prefix []app.Message, hasMore bool) Model {
	m.loadingMore = false
	m.hasMore = hasMore
	if len(prefix) == 0 {
		return m
	}
	// The viewport is anchored to the bottom, so prepending older messages
	// does not shift what the user is currently looking at — no offset change
	// is needed. Just clamp in case the body height changed meanwhile.
	m.messages = append(prefix, m.messages...)
	if max := m.maxLineOffset(); m.lineOffset > max {
		m.lineOffset = max
	}
	return m
}

func (m Model) onIncomingMessage(peerKey string, msg app.Message) Model {
	isCurrent := m.screen == app.ScreenChat && m.selected != nil &&
		telegram.PeerRefKey(m.selected.Peer) == peerKey

	// 1) update dialog preview
	if idx := indexOfDialog(m.dialogs, peerKey); idx >= 0 {
		m.dialogs[idx].LastMsg = msg.Text
		m.dialogs[idx].LastDate = int(msg.Date.Unix())
		if !msg.Out && !isCurrent {
			m.dialogs[idx].Unread++
		}
		m = resortDialogsKeepingCursor(m)
	}

	// 2) if it's the open chat, append + reset unread
	if isCurrent {
		wasAtBottom := m.lineOffset == 0
		addedLines := m.renderedLineCount(msg)
		m.messages = append(m.messages, msg)
		// If the user has scrolled up, keep their view stable by pushing the
		// offset down by however many lines the new message occupies.
		if !wasAtBottom {
			m.lineOffset += addedLines
		}
		if idx := indexOfDialog(m.dialogs, peerKey); idx >= 0 {
			m.dialogs[idx].Unread = 0
		}
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

func resortDialogsKeepingCursor(m Model) Model {
	var cursorKey string
	if m.cursor < len(m.dialogs) {
		cursorKey = m.dialogs[m.cursor].Key
	}
	sort.SliceStable(m.dialogs, func(i, j int) bool {
		return m.dialogs[i].LastDate > m.dialogs[j].LastDate
	})
	if cursorKey != "" {
		for i := range m.dialogs {
			if m.dialogs[i].Key == cursorKey {
				m.cursor = i
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

