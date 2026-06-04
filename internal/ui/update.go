package ui

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

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
	// Forward overlay intercepts all keys.
	if m.forwardActive {
		return m.handleForwardOverlay(msg)
	}
	// Confirmations take priority over everything else.
	if m.discardPrompt {
		return m.handleDiscardPrompt(msg)
	}
	if m.deletePrompt {
		return m.handleDeletePrompt(msg)
	}
	// Any key dismisses the :ls overlay.
	if len(m.overlay) > 0 {
		m.overlay = nil
		return m, nil
	}
	// Window navigation chord: <C-w> then h / l / w.
	if m.pendingCtrlW {
		m.pendingCtrlW = false
		m.focusWindow(msg.String())
		return m, nil
	}
	if m.vimMode == app.ModeNormal && msg.String() == "ctrl+w" {
		m.pendingCtrlW = true
		return m, nil
	}
	if m.activeBuffer().kind == bufChatList {
		return m.updateChatList(msg)
	}
	return m.updateChat(msg)
}

// focusWindow handles the key following <C-w>, moving focus between windows.
// The compose draft follows focus: it is saved to the window we leave and the
// new window's chat draft is loaded into the shared input widget.
func (m *Model) focusWindow(key string) {
	n := len(m.wins)
	prev := m.focused
	switch key {
	case "h", "left":
		if m.focused > 0 {
			m.focused--
		}
	case "l", "right":
		if m.focused < n-1 {
			m.focused++
		}
	case "w":
		m.focused = (m.focused + 1) % n
	}
	if m.focused == prev {
		return
	}
	if b := m.bufferOf(m.wins[prev]); b != nil && b.kind == bufChat {
		b.draft = m.msgInput.Value()
	}
	m.syncInputToActive()
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
	if m.vimMode == app.ModeVisual {
		return m.updateVisualMode(msg)
	}
	w := m.activeWindow()
	key := msg.String()

	// Accumulate numeric prefix (e.g. "10" before "j").
	if isDigitKey(key) {
		m.countBuf += key
		return m, nil
	}

	switch key {
	case "esc":
		m.countBuf = ""
	case ":":
		m.countBuf = ""
		m.enterCommandMode()
		return m, nil
	case "v":
		m.countBuf = ""
		m.vimMode = app.ModeVisual
		return m, nil
	case "up", "k":
		m.moveCursor(-m.consumeCount())
	case "down", "j":
		m.moveCursor(m.consumeCount())
	case "g", "home":
		m.countBuf = ""
		w.cursor = 0
		w.listOffset = 0
	case "G", "end":
		m.countBuf = ""
		w.cursor = len(m.visibleDialogs()) - 1
		m.adjustListOffset()
	case "H":
		m.countBuf = ""
		w.cursor = w.listOffset
	case "L":
		m.countBuf = ""
		last := w.listOffset + m.visibleRows() - 1
		if max := len(m.visibleDialogs()) - 1; last > max {
			last = max
		}
		w.cursor = last
	case "enter":
		m.countBuf = ""
		return m.openSelectedChat()
	default:
		m.countBuf = ""
	}
	return m, nil
}

func (m *Model) moveCursor(delta int) {
	w := m.activeWindow()
	next := w.cursor + delta
	if next < 0 || next >= len(m.visibleDialogs()) {
		return
	}
	w.cursor = next
	m.adjustListOffset()
}

func (m Model) openSelectedChat() (tea.Model, tea.Cmd) {
	visible := m.visibleDialogs()
	if len(visible) == 0 {
		return m, nil
	}
	d := visible[m.activeWindow().cursor]

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
	case app.ModeVisual:
		return m.updateVisualMode(msg)
	default:
		return m.updateChatNormal(msg)
	}
}

// updateVisualMode handles VISUAL mode in a chat buffer. j/k extend the
// selection; y/r/d/f operate on all selected messages.
func (m Model) updateVisualMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Visual mode in the chat list is a no-op except for esc.
	if m.activeBuffer().kind == bufChatList {
		if msg.String() == "esc" {
			m.vimMode = app.ModeNormal
		}
		return m, nil
	}

	b := m.activeBuffer()
	w := m.activeWindow()
	height := m.chatBodyHeight()

	// Handle "d" chord second key.
	if m.pendingVisualDelete {
		m.pendingVisualDelete = false
		switch msg.String() {
		case "m", "d":
			return m.visualDelete(false)
		case "a":
			return m.visualDelete(true)
		}
		return m, nil
	}

	// Accumulate numeric prefix in Visual mode too.
	if isDigitKey(msg.String()) && !m.pendingVisualDelete {
		m.countBuf += msg.String()
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.countBuf = ""
		m.vimMode = app.ModeNormal
	case "j", "down":
		m.moveMsgCursor(m.consumeCount())
	case "k", "up":
		m.moveMsgCursor(-m.consumeCount())
	case "ctrl+u", "pgup":
		n := m.consumeCount()
		m.moveMsgCursor(-(height / 2 * n))
	case "ctrl+d", "pgdown":
		n := m.consumeCount()
		m.moveMsgCursor(height / 2 * n)
	case "g", "home":
		m.countBuf = ""
		total := len(m.chatLines(b, w.width))
		w.msgCursor = 0
		w.colCursor = 0
		w.lineOffset = clampMin(total-height, 0)
	case "G", "end":
		m.countBuf = ""
		m.chatCursorToBottom(b, w)
	case "H":
		total := len(m.chatLines(b, w.width))
		top := total - w.lineOffset - height
		if top < 0 {
			top = 0
		}
		w.msgCursor = top
	case "L":
		total := len(m.chatLines(b, w.width))
		bot := total - w.lineOffset - 1
		if bot < 0 {
			bot = 0
		}
		if bot >= total {
			bot = total - 1
		}
		w.msgCursor = bot
	case "y":
		msgs := m.selectedMessages(b, w)
		var texts []string
		for _, msg := range msgs {
			if msg.Text != "" {
				texts = append(texts, msg.Text)
			}
		}
		if len(texts) > 0 {
			m.yankReg = strings.Join(texts, "\n")
		}
		m.vimMode = app.ModeNormal
	case "r":
		msgs := m.selectedMessages(b, w)
		if len(msgs) > 0 {
			last := msgs[len(msgs)-1]
			b.replyToID = last.ID
			b.replyToPreview = previewSnippet(last.Text)
		}
		m.vimMode = app.ModeNormal
		m.msgInput.Focus()
		return m, textinput.Blink
	case "d":
		if len(m.selectedMessages(b, w)) > 0 {
			m.pendingVisualDelete = true
		}
	case "f":
		msgs := m.selectedMessages(b, w)
		if len(msgs) > 0 {
			ids := make([]int, len(msgs))
			for i, msg := range msgs {
				ids[i] = msg.ID
			}
			m.forwardActive = true
			m.forwardSrcPeer = b.peer
			m.forwardMsgIDs = ids
			m.forwardMsgText = msgs[0].Text
			m.forwardCursor = 0
			m.forwardOffset = 0
		}
		m.vimMode = app.ModeNormal
	}
	return m, nil
}

// selectedMessages returns the unique messages whose visual lines overlap with
// the current visual selection [lo, hi] (inclusive, in chatLines index space).
func (m *Model) selectedMessages(b *buffer, w *window) []app.Message {
	lo := m.visualAnchor
	hi := w.msgCursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if len(b.messages) == 0 {
		return nil
	}
	chatName, selfName := m.chatNames(b)
	var result []app.Message
	lineIdx := 1 // line 0 is historyTopLine
	var prev *app.Message
	for i := range b.messages {
		msg := b.messages[i]
		header := startsNewGroup(prev, msg)
		count := len(renderMessageBlock(msg, chatName, selfName, w.width, header))
		msgHi := lineIdx + count - 1
		if msgHi >= lo && lineIdx <= hi {
			result = append(result, b.messages[i])
		}
		lineIdx += count
		prev = &b.messages[i]
	}
	return result
}

// visualDelete deletes all selected messages. revoke=true deletes for everyone.
func (m Model) visualDelete(revoke bool) (tea.Model, tea.Cmd) {
	b := m.activeBuffer()
	w := m.activeWindow()
	msgs := m.selectedMessages(b, w)
	if len(msgs) == 0 {
		m.vimMode = app.ModeNormal
		return m, nil
	}
	ids := make([]int, len(msgs))
	for i, msg := range msgs {
		ids[i] = msg.ID
	}
	m.vimMode = app.ModeNormal
	peer := b.peer
	client := m.client
	return m, func() tea.Msg {
		client.DeleteMessages(peer, ids, revoke)
		return nil
	}
}

func (m Model) updateChatNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	height := m.chatBodyHeight()
	w := m.activeWindow()
	b := m.activeBuffer()
	readOnly := b.kind == bufHelp
	key := msg.String()

	// Accumulate numeric prefix — but only when no chord is pending, so "2da"
	// doesn't misfire. Digits are consumed by the next motion key.
	if isDigitKey(key) && !m.pendingYank && !m.pendingDelete && !m.pendingMark && !m.pendingJump {
		m.countBuf += key
		return m, nil
	}

	// Handle second key of "y" chord.
	if m.pendingYank {
		m.pendingYank = false
		if msg.String() == "y" {
			if cm := m.cursorMessage(b, w); cm != nil {
				m.yankReg = cm.Text
			}
		}
		return m, nil
	}

	// Handle second key of "d" chord.
	if m.pendingDelete {
		m.pendingDelete = false
		switch msg.String() {
		case "m", "d":
			if cm := m.cursorMessage(b, w); cm != nil {
				m.deleteRevoke = false
				m.deletePrompt = true
			}
		case "a":
			if cm := m.cursorMessage(b, w); cm != nil {
				m.deleteRevoke = true
				m.deletePrompt = true
			}
		}
		return m, nil
	}

	// Handle second key of "m" chord: set mark.
	if m.pendingMark {
		m.pendingMark = false
		r := []rune(msg.String())
		if len(r) == 1 && r[0] >= 'a' && r[0] <= 'z' {
			if b.marks == nil {
				b.marks = make(map[rune]int)
			}
			b.marks[r[0]] = w.msgCursor
		}
		return m, nil
	}

	// Handle second key of "'" chord: jump to mark.
	if m.pendingJump {
		m.pendingJump = false
		r := []rune(msg.String())
		if len(r) == 1 && r[0] >= 'a' && r[0] <= 'z' {
			if pos, ok := b.marks[r[0]]; ok {
				total := len(m.chatLines(b, w.width))
				if pos < total {
					w.msgCursor = pos
					w.colCursor = 0
					// Center the mark in the viewport.
					w.lineOffset = clampMin(total-pos-height/2, 0)
					if w.lineOffset > clampMin(total-height, 0) {
						w.lineOffset = clampMin(total-height, 0)
					}
				}
			}
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.countBuf = ""
		draft := strings.TrimSpace(m.msgInput.Value())
		if draft != "" {
			m.discardPrompt = true
			return m, nil
		}
		b.replyToID = 0
		b.replyToPreview = ""
		return m, nil
	case "y":
		if !readOnly && m.cursorMessage(b, w) != nil {
			m.pendingYank = true
		}
		return m, nil
	case "p":
		if readOnly {
			return m, nil
		}
		if m.yankReg != "" {
			m.msgInput.SetValue(m.yankReg)
			m.vimMode = app.ModeEdit
			m.msgInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	case "d":
		if !readOnly && m.cursorMessage(b, w) != nil {
			m.pendingDelete = true
		}
		return m, nil
	case "m":
		if len(m.chatLines(b, w.width)) > 0 {
			m.pendingMark = true
		}
		return m, nil
	case "'":
		if b.marks != nil && len(b.marks) > 0 {
			m.pendingJump = true
		}
		return m, nil
	case "f":
		if cm := m.cursorMessage(b, w); cm != nil {
			m.forwardActive = true
			m.forwardSrcPeer = b.peer
			m.forwardMsgIDs = []int{cm.ID}
			m.forwardMsgText = cm.Text
			m.forwardCursor = 0
			m.forwardOffset = 0
		}
		return m, nil
	case "r":
		if !readOnly {
			if msg := m.cursorMessage(b, w); msg != nil {
				b.replyToID = msg.ID
				b.replyToPreview = previewSnippet(msg.Text)
			}
		}
		return m, nil
	case "e":
		if readOnly {
			return m, nil
		}
		if msg := m.cursorMessage(b, w); msg != nil {
			b.editMsgID = msg.ID
			b.editOrigText = previewSnippet(msg.Text)
			b.replyToID = 0
			b.replyToPreview = ""
			m.msgInput.SetValue(msg.Text)
			m.vimMode = app.ModeEdit
			m.msgInput.Focus()
			return m, textinput.Blink
		}
		return m, nil
	case ":":
		m.enterCommandMode()
		return m, nil
	case "v":
		m.vimMode = app.ModeVisual
		m.visualAnchor = w.msgCursor
		return m, nil
	case "a", "i":
		if readOnly {
			return m, nil
		}
		m.vimMode = app.ModeEdit
		m.msgInput.Focus()
		return m, textinput.Blink
	case "k", "up":
		if cmd := m.moveMsgCursor(-m.consumeCount()); cmd != nil {
			return m, cmd
		}
	case "j", "down":
		m.moveMsgCursor(m.consumeCount())
	case "pgup", "ctrl+u":
		n := m.consumeCount()
		if cmd := m.moveMsgCursor(-(height / 2 * n)); cmd != nil {
			return m, cmd
		}
	case "pgdown", "ctrl+d":
		n := m.consumeCount()
		m.moveMsgCursor(height / 2 * n)
	case "g", "home":
		m.countBuf = ""
		total := len(m.chatLines(b, w.width))
		w.msgCursor = 0
		w.colCursor = 0
		w.lineOffset = clampMin(total-height, 0)
		if cmd := m.maybeLoadOlder(); cmd != nil {
			return m, cmd
		}
	case "G", "end":
		m.countBuf = ""
		m.chatCursorToBottom(b, w)
	case "H":
		m.countBuf = ""
		total := len(m.chatLines(b, w.width))
		top := total - w.lineOffset - height
		if top < 0 {
			top = 0
		}
		w.msgCursor = top
		w.colCursor = 0
	case "L":
		m.countBuf = ""
		total := len(m.chatLines(b, w.width))
		bot := total - w.lineOffset - 1
		if bot < 0 {
			bot = 0
		}
		if bot >= total {
			bot = total - 1
		}
		w.msgCursor = bot
		w.colCursor = 0
	default:
		m.countBuf = ""
	}
	return m, nil
}

// moveMsgCursor moves the chat cursor by delta visual lines, scrolling the
// viewport to keep it visible. Returns a load-older command when the cursor
// reaches the very top of the loaded history.
func (m *Model) moveMsgCursor(delta int) tea.Cmd {
	w := m.activeWindow()
	b := m.activeBuffer()
	total := len(m.chatLines(b, w.width))
	if total == 0 {
		return nil
	}
	height := m.chatBodyHeight()

	// Snap to bottom on first use (msgCursor==0, lineOffset==0 means the user
	// hasn't moved yet and the natural position is the last line).
	if w.msgCursor == 0 && w.lineOffset == 0 {
		w.msgCursor = total - 1
	}

	w.msgCursor += delta
	if w.msgCursor < 0 {
		w.msgCursor = 0
	}
	if w.msgCursor >= total {
		w.msgCursor = total - 1
	}
	w.colCursor = 0

	// Keep cursor inside the visible band:
	//   visible = [ total - lineOffset - height,  total - lineOffset )
	visTop := total - w.lineOffset - height
	visBot := total - w.lineOffset

	if w.msgCursor < visTop {
		// cursor scrolled above the top edge → scroll up
		w.lineOffset = total - w.msgCursor - height
		if max := clampMin(total-height, 0); w.lineOffset > max {
			w.lineOffset = max
		}
	} else if w.msgCursor >= visBot {
		// cursor scrolled below the bottom edge → scroll down
		w.lineOffset = total - w.msgCursor - 1
		if w.lineOffset < 0 {
			w.lineOffset = 0
		}
	}

	if w.msgCursor == 0 {
		return m.maybeLoadOlder()
	}
	return nil
}

// chatCursorToBottom moves the cursor and viewport to the last (newest) line.
func (m *Model) chatCursorToBottom(b *buffer, w *window) {
	total := len(m.chatLines(b, w.width))
	w.msgCursor = clampMin(total-1, 0)
	w.colCursor = 0
	w.lineOffset = 0
}

// cursorMessage returns the app.Message that the visual cursor sits on, or nil
// if the cursor is on a non-message line (the history-top marker).
func (m *Model) cursorMessage(b *buffer, w *window) *app.Message {
	target := w.msgCursor
	if target <= 0 || len(b.messages) == 0 {
		return nil
	}
	chatName, selfName := m.chatNames(b)
	lineIdx := 1 // line 0 is always historyTopLine
	var prev *app.Message
	for i := range b.messages {
		msg := b.messages[i]
		header := startsNewGroup(prev, msg)
		count := len(renderMessageBlock(msg, chatName, selfName, w.width, header))
		if target >= lineIdx && target < lineIdx+count {
			return &b.messages[i]
		}
		lineIdx += count
		prev = &b.messages[i]
	}
	return nil
}

// clampCol returns col clamped to the visible character count of the cursor line.
func (m *Model) clampCol(b *buffer, w *window, col int) int {
	lines := m.chatLines(b, w.width)
	if w.msgCursor < 0 || w.msgCursor >= len(lines) {
		return 0
	}
	runes := []rune(ansi.Strip(lines[w.msgCursor]))
	if col >= len(runes) {
		col = clampMin(len(runes)-1, 0)
	}
	return col
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
		b := m.activeBuffer()
		if b.editMsgID != 0 {
			// Cancel edit: restore original draft and clear edit state.
			m.msgInput.SetValue(b.draft)
			b.editMsgID = 0
			b.editOrigText = ""
		}
		m.vimMode = app.ModeNormal
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
	b.draft = ""

	// Edit mode: update an existing message instead of sending a new one.
	if b.editMsgID != 0 {
		editID := b.editMsgID
		b.editMsgID = 0
		b.editOrigText = ""
		m.vimMode = app.ModeNormal
		m.msgInput.Blur()
		peer := b.peer
		client := m.client
		return m, func() tea.Msg {
			client.EditMessage(peer, editID, text)
			return nil
		}
	}

	b.sending = true
	m.activeWindow().lineOffset = 0
	peer := b.peer
	replyToMsgID := b.replyToID
	replyToPreview := b.replyToPreview
	b.replyToID = 0
	b.replyToPreview = ""
	client := m.client
	return m, func() tea.Msg {
		client.SendMessage(peer, text, replyToMsgID, replyToPreview)
		return nil
	}
}

// ----- Forward overlay ----------------------------------------------------

func (m Model) handleForwardOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := m.visibleDialogs()
	h := m.heightOrDefault()
	bodyRows := h - 3

	switch msg.String() {
	case "esc":
		m.forwardActive = false
		return m, nil
	case "j", "down":
		if m.forwardCursor < len(visible)-1 {
			m.forwardCursor++
			if m.forwardCursor >= m.forwardOffset+bodyRows {
				m.forwardOffset = m.forwardCursor - bodyRows + 1
			}
		}
	case "k", "up":
		if m.forwardCursor > 0 {
			m.forwardCursor--
			if m.forwardCursor < m.forwardOffset {
				m.forwardOffset = m.forwardCursor
			}
		}
	case "enter":
		if len(visible) == 0 {
			return m, nil
		}
		dest := visible[m.forwardCursor]
		m.forwardActive = false
		srcPeer := m.forwardSrcPeer
		msgIDs := m.forwardMsgIDs
		client := m.client
		return m, func() tea.Msg {
			client.ForwardMessages(srcPeer, dest.Peer, msgIDs)
			return nil
		}
	}
	return m, nil
}

// ----- Delete confirmation ------------------------------------------------

// handleDeletePrompt handles the y/N answer to "Delete message?".
func (m Model) handleDeletePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.deletePrompt = false
	if msg.String() != "y" && msg.String() != "Y" {
		return m, nil
	}
	b := m.activeBuffer()
	w := m.activeWindow()
	cm := m.cursorMessage(b, w)
	if cm == nil {
		return m, nil
	}
	revoke := m.deleteRevoke
	peer := b.peer
	client := m.client
	return m, func() tea.Msg {
		client.DeleteMessages(peer, []int{cm.ID}, revoke)
		return nil
	}
}

// ----- Discard-draft confirmation -----------------------------------------

// handleDiscardPrompt handles the y/N answer to "Discard draft?".
func (m Model) handleDiscardPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.discardPrompt = false
	switch msg.String() {
	case "y", "Y":
		b := m.activeBuffer()
		m.msgInput.SetValue("")
		b.draft = ""
		b.replyToID = 0
		b.replyToPreview = ""
	}
	// Any other key (including N, n, esc) keeps draft and reply intact.
	return m, nil
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
	m.vimMode = app.ModeNormal
	m.cmdBuf = buf
}

func (m Model) executeCommand(raw string) (tea.Model, tea.Cmd) {
	cmd := app.ParseCommand(raw)
	switch cmd.Kind {
	case app.CmdQuit:
		// With multiple windows :q closes the focused one; otherwise it quits.
		if len(m.wins) > 1 {
			return m.closeWindow(), nil
		}
		m.cancel()
		return m, tea.Quit
	case app.CmdQuitForce:
		m.cancel()
		return m, tea.Quit
	case app.CmdClose:
		if len(m.wins) > 1 {
			return m.closeWindow(), nil
		}
		return m, nil // cannot close the last window
	case app.CmdVSplit:
		return m.splitVertical(cmd.Arg), nil
	case app.CmdBuffers:
		m.overlay = m.bufferListLines()
		return m, nil
	case app.CmdBufferSwitch:
		return m.switchByID(parseID(cmd.Arg)), nil
	case app.CmdBufferAlt:
		if m.activeWindow().altBuffer != 0 && m.buffers.find(m.activeWindow().altBuffer) != nil {
			m.switchBuffer(m.activeWindow().altBuffer)
		}
		return m, nil
	case app.CmdBufferNext:
		m.switchBuffer(m.buffers.next(m.activeWindow().bufferID))
		return m, nil
	case app.CmdBufferPrev:
		m.switchBuffer(m.buffers.prev(m.activeWindow().bufferID))
		return m, nil
	case app.CmdBufferDelete:
		id := m.activeWindow().bufferID
		if cmd.HasArg {
			id = parseID(cmd.Arg)
		}
		return m.deleteBuffer(id), nil
	case app.CmdHelp:
		b := m.buffers.addHelp(helpContent)
		m.switchBuffer(b.id)
		m.chatCursorToBottom(b, m.activeWindow())
		return m, nil
	case app.CmdSet:
		switch cmd.Arg {
		case "showarchive":
			m.showArchive = true
		case "noshowarchive":
			m.showArchive = false
		default:
			if raw != "" {
				m.err = &unknownCmdError{cmd: raw}
			}
		}
		return m, nil
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
	if id == m.activeWindow().bufferID || m.buffers.find(id) == nil {
		return
	}
	// Save the current chat buffer's draft.
	if cur := m.activeBuffer(); cur != nil && cur.kind == bufChat {
		cur.draft = m.msgInput.Value()
	}
	m.activeWindow().altBuffer = m.activeWindow().bufferID
	m.activeWindow().bufferID = id
	// Reset this window's viewport for the new buffer.
	m.activeWindow().lineOffset = 0
	m.activeWindow().cursor = 0
	m.activeWindow().listOffset = 0
	m.vimMode = app.ModeNormal
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

	m.buffers.delete(id)

	// Repoint every window that referenced the deleted buffer.
	for _, w := range m.wins {
		if w.altBuffer == id {
			w.altBuffer = 0
		}
		if w.bufferID == id {
			fallback := w.altBuffer
			if fallback == 0 || m.buffers.find(fallback) == nil {
				fallback = chatListBufferID
			}
			w.bufferID = fallback
			w.lineOffset, w.cursor, w.listOffset = 0, 0, 0
		}
	}
	m.vimMode = app.ModeNormal
	m.syncInputToActive()
	return m
}

// syncInputToActive loads the focused chat buffer's draft into the compose
// widget (or clears it for the chat list), and blurs it.
func (m *Model) syncInputToActive() {
	if b := m.activeBuffer(); b != nil && b.kind == bufChat {
		m.msgInput.SetValue(b.draft)
	} else {
		m.msgInput.SetValue("")
	}
	m.msgInput.Blur()
}

// splitVertical inserts a new window just to the right of the focused one.
// Focus stays on the current (left) window, matching the spec workflow.
// arg: empty => same buffer; numeric => buffer id; "chats" => the chat list.
func (m Model) splitVertical(arg string) Model {
	target := m.activeWindow().bufferID
	if arg != "" {
		if strings.EqualFold(arg, "chats") {
			target = chatListBufferID
		} else if id := parseID(arg); id > 0 && m.buffers.find(id) != nil {
			target = id
		} else {
			m.err = &unknownCmdError{cmd: "vs " + arg}
			return m
		}
	}
	w := &window{bufferID: target}
	idx := m.focused + 1
	m.wins = append(m.wins, nil)
	copy(m.wins[idx+1:], m.wins[idx:])
	m.wins[idx] = w
	// focus stays on the current (left) window
	return m
}

// closeWindow removes the focused window (callers ensure len(wins) > 1).
func (m Model) closeWindow() Model {
	m.wins = append(m.wins[:m.focused], m.wins[m.focused+1:]...)
	if m.focused >= len(m.wins) {
		m.focused = len(m.wins) - 1
	}
	m.vimMode = app.ModeNormal
	m.syncInputToActive()
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
		m.activeWindow().bufferID = chatListBufferID
		m.activeWindow().cursor = 0
		m.activeWindow().listOffset = 0
		m.authInput.Blur()
		m.seedStatuses(ev.Dialogs)
		return m, nil
	case telegram.EventMessagesLoaded:
		if b := m.buffers.findByPeer(ev.PeerKey); b != nil {
			b.messages = ev.Messages
			b.hasMore = ev.HasMore
			b.loadingMsg = false
			b.msgVersion++
			// Place cursor at the newest (bottom) line for all windows.
			for _, w := range m.wins {
				if w.bufferID == b.id {
					m.chatCursorToBottom(b, w)
				}
			}
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
			for _, w := range m.wins {
				if w.bufferID == b.id {
					m.chatCursorToBottom(b, w)
				}
			}
		}
		return m, nil
	case telegram.EventMessageReceived:
		return m.onIncomingMessage(ev.PeerKey, ev.Message), nil
	case telegram.EventMessageEdited:
		if ev.Err != nil {
			m.err = ev.Err
			return m, nil
		}
		if b := m.buffers.findByPeer(ev.PeerKey); b != nil {
			for i := range b.messages {
				if b.messages[i].ID == ev.MsgID {
					b.messages[i].Text = ev.NewText
					b.msgVersion++
					break
				}
			}
		}
		return m, nil
	case telegram.EventMessageDeleted:
		if ev.Err != nil {
			m.err = ev.Err
			return m, nil
		}
		if b := m.buffers.findByPeer(ev.PeerKey); b != nil {
			for i, msg := range b.messages {
				if msg.ID == ev.MsgID {
					b.messages = append(b.messages[:i], b.messages[i+1:]...)
					b.msgVersion++
					// Keep cursor in bounds.
					for _, w := range m.wins {
						if w.bufferID == b.id {
							total := len(m.chatLines(b, w.width))
							if w.msgCursor >= total {
								w.msgCursor = clampMin(total-1, 0)
							}
						}
					}
					break
				}
			}
		}
		return m, nil
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

	// Snapshot old line counts per window before mutation so we can compute
	// how many visual lines were inserted at the top.
	type snap struct {
		w        *window
		oldTotal int
	}
	var snaps []snap
	for _, w := range m.wins {
		if w.bufferID == b.id {
			snaps = append(snaps, snap{w, len(m.chatLines(b, w.width))})
		}
	}

	b.messages = append(prefix, b.messages...)
	b.msgVersion++

	for _, s := range snaps {
		newTotal := len(m.chatLines(b, s.w.width))
		added := newTotal - s.oldTotal
		// Shift the cursor so it stays on the same message.
		s.w.msgCursor += added
		if s.w.msgCursor >= newTotal {
			s.w.msgCursor = clampMin(newTotal-1, 0)
		}
		// Clamp scroll offset.
		if max := clampMin(newTotal-m.chatBodyHeight(), 0); s.w.lineOffset > max {
			s.w.lineOffset = max
		}
	}
	return m
}

func (m Model) onIncomingMessage(peerKey string, msg app.Message) Model {
	buf := m.buffers.findByPeer(peerKey)
	isActiveChat := buf != nil && m.activeWindow().bufferID == buf.id

	// 1) append to the chat buffer (loaded but maybe not visible)
	if buf != nil {
		// If this is a reply and we don't yet have a preview, try to find the
		// replied-to message among the already-loaded buffer messages.
		if msg.ReplyToID != 0 && msg.ReplyPreview == "" {
			if prev := findMessageByID(buf.messages, msg.ReplyToID); prev != nil {
				msg.ReplyPreview = previewSnippet(prev.Text)
			}
		}
		wasAtBottom := isActiveChat && m.activeWindow().lineOffset == 0
		addedLines := 0
		if isActiveChat && !wasAtBottom {
			addedLines = m.appendedLineCount(buf, msg)
		}
		buf.messages = append(buf.messages, msg)
		buf.msgVersion++
		// Keep a scrolled-up active window stable.
		if isActiveChat && !wasAtBottom {
			m.activeWindow().lineOffset += addedLines
		}
		// Auto-advance cursor for windows that were already at the bottom.
		for _, w := range m.wins {
			if w.bufferID == buf.id && w.lineOffset == 0 {
				total := len(m.chatLines(buf, w.width))
				w.msgCursor = clampMin(total-1, 0)
				w.colCursor = 0
			}
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

// appendedLineCount returns how many visual lines msg will add to buf when
// appended, accounting for whether it starts a new "[HH-MM] Name" group.
func (m Model) appendedLineCount(buf *buffer, msg app.Message) int {
	chatName, selfName := m.chatNames(buf)
	var prev *app.Message
	if n := len(buf.messages); n > 0 {
		prev = &buf.messages[n-1]
	}
	header := startsNewGroup(prev, msg)
	return len(renderMessageBlock(msg, chatName, selfName, m.activeWindow().width, header))
}

// findMessageByID returns a pointer to the message with the given id, or nil.
func findMessageByID(msgs []app.Message, id int) *app.Message {
	for i := range msgs {
		if msgs[i].ID == id {
			return &msgs[i]
		}
	}
	return nil
}

// previewSnippet shortens a message body for a single-line "replies to ..."
// hint, matching what the telegram layer produces for messages loaded as a page.
func previewSnippet(text string) string {
	t := singleLine(text)
	if t == "" {
		return "[media]"
	}
	const max = 80
	r := []rune(t)
	if len(r) > max {
		return string(r[:max])
	}
	return t
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
	visible := m.visibleDialogs()
	if w.cursor < len(visible) {
		cursorKey = visible[w.cursor].Key
	}
	sort.SliceStable(m.dialogs, func(i, j int) bool {
		return m.dialogs[i].LastDate > m.dialogs[j].LastDate
	})
	if cursorKey != "" {
		newVisible := m.visibleDialogs()
		for i := range newVisible {
			if newVisible[i].Key == cursorKey {
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

// consumeCount reads m.countBuf as an integer multiplier (default 1) and
// clears it. Call before any motion that should respect a numeric prefix.
func (m *Model) consumeCount() int {
	s := m.countBuf
	m.countBuf = ""
	if s == "" {
		return 1
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// isDigitKey reports whether key is a single ASCII digit (0-9).
func isDigitKey(key string) bool {
	return len(key) == 1 && key[0] >= '0' && key[0] <= '9'
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

