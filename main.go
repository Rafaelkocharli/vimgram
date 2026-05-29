package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// =====================================================================
// .env loader
// =====================================================================

func loadEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.Index(line, "=")
		if i < 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(strings.Trim(strings.TrimSpace(line[i+1:]), `"'`))
		_ = os.Setenv(k, v)
	}
	return sc.Err()
}

// =====================================================================
// Domain types
// =====================================================================

type dialogEntry struct {
	key      string // peerKey для матчинга с апдейтами
	title    string
	kind     string // "Группа" | "Канал" | "Личка"
	peer     tg.InputPeerClass
	lastMsg  string
	lastDate int
	unread   int
	extra    string
}

type messageEntry struct {
	id       int
	out      bool
	fromName string
	text     string
	date     time.Time
}

// =====================================================================
// Messages between goroutine (gotd) and tea model
// =====================================================================

type (
	connectedMsg       struct{}
	needPhoneMsg       struct{}
	needCodeMsg        struct{}
	needPasswordMsg    struct{}
	dialogsLoadedMsg   struct{ self *tg.User; dialogs []dialogEntry }
	messagesLoadedMsg  struct{ messages []messageEntry; hasMore bool }
	messagesPrependMsg struct{ messages []messageEntry; hasMore bool }
	messageSentMsg     struct{ entry messageEntry; err error }
	newIncomingMsg     struct{ peerKey string; entry messageEntry }
	errMsg             struct{ err error }
)

// Requests from tea → goroutine
type (
	openChatReq  struct{ peer tg.InputPeerClass }
	sendMsgReq   struct{ peer tg.InputPeerClass; text string }
	loadMoreReq  struct{ peer tg.InputPeerClass; beforeID int }
)

// =====================================================================
// Auth bridge: implements auth.UserAuthenticator over tea channels
// =====================================================================

type teaAuth struct {
	send func(tea.Msg)
	ch   chan string
}

func (a *teaAuth) wait(ctx context.Context, want tea.Msg) (string, error) {
	a.send(want)
	select {
	case v := <-a.ch:
		return v, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (a *teaAuth) Phone(ctx context.Context) (string, error) {
	return a.wait(ctx, needPhoneMsg{})
}
func (a *teaAuth) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	return a.wait(ctx, needCodeMsg{})
}
func (a *teaAuth) Password(ctx context.Context) (string, error) {
	return a.wait(ctx, needPasswordMsg{})
}
func (a *teaAuth) AcceptTermsOfService(_ context.Context, _ tg.HelpTermsOfService) error {
	return nil
}
func (a *teaAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported, use an existing account")
}

// =====================================================================
// Bubble Tea model
// =====================================================================

type screen int

const (
	screenAuth screen = iota
	screenChatList
	screenChat
)

type vimMode int

const (
	modeVisual vimMode = iota
	modeEdit
	modeCommand
)

func (v vimMode) label() string {
	switch v {
	case modeEdit:
		return " INSERT "
	case modeCommand:
		return " COMMAND "
	default:
		return " VISUAL "
	}
}

type authState int

const (
	authConnecting authState = iota
	authNeedPhone
	authNeedCode
	authNeedPassword
	authLoading
)

type model struct {
	screen screen

	// auth
	auth      authState
	authInput textinput.Model

	// spinner
	spin spinner.Model

	// chat list
	self       *tg.User
	dialogs    []dialogEntry
	cursor     int
	listOffset int

	// chat view
	selected    *dialogEntry
	messages    []messageEntry
	msgInput    textinput.Model
	loadingMsg  bool
	loadingMore bool
	hasMore     bool
	sending     bool
	msgOffset   int // сколько сообщений от низа прокручено вверх (0 = низ)

	// vim modes / commands
	vimMode vimMode
	cmdBuf  string

	// shared
	err    error
	width  int
	height int

	// channels
	inputCh chan string         // auth replies
	reqCh   chan any            // requests to gotd goroutine
	cancel  context.CancelFunc
}

func newModel(inputCh chan string, reqCh chan any, cancel context.CancelFunc) model {
	ai := textinput.New()
	ai.CharLimit = 64
	ai.Width = 40

	mi := textinput.New()
	mi.Placeholder = "Type a message..."
	mi.CharLimit = 2048
	mi.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return model{
		screen:    screenAuth,
		auth:      authConnecting,
		authInput: ai,
		msgInput:  mi,
		spin:      sp,
		inputCh:   inputCh,
		reqCh:     reqCh,
		cancel:    cancel,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}

// =====================================================================
// Update
// =====================================================================

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.msgInput.Width = max(20, msg.Width-4)
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case screenAuth:
			return m.updateAuth(msg)
		case screenChatList:
			return m.updateChatList(msg)
		case screenChat:
			return m.updateChat(msg)
		}

	case connectedMsg:
		m.auth = authLoading
		return m, nil

	case needPhoneMsg:
		m.auth = authNeedPhone
		m.authInput.Placeholder = "+79001234567"
		m.authInput.EchoMode = textinput.EchoNormal
		m.authInput.SetValue("")
		m.authInput.Focus()
		return m, textinput.Blink

	case needCodeMsg:
		m.auth = authNeedCode
		m.authInput.Placeholder = "12345"
		m.authInput.EchoMode = textinput.EchoNormal
		m.authInput.SetValue("")
		m.authInput.Focus()
		return m, textinput.Blink

	case needPasswordMsg:
		m.auth = authNeedPassword
		m.authInput.Placeholder = "2FA password"
		m.authInput.EchoMode = textinput.EchoPassword
		m.authInput.SetValue("")
		m.authInput.Focus()
		return m, textinput.Blink

	case dialogsLoadedMsg:
		m.self = msg.self
		m.dialogs = msg.dialogs
		m.screen = screenChatList
		m.cursor = 0
		m.listOffset = 0
		m.authInput.Blur()
		return m, nil

	case messagesLoadedMsg:
		m.messages = msg.messages
		m.hasMore = msg.hasMore
		m.loadingMsg = false
		return m, nil

	case messagesPrependMsg:
		m.loadingMore = false
		m.hasMore = msg.hasMore
		if len(msg.messages) > 0 {
			m.messages = append(msg.messages, m.messages...)
			// Удерживаем видимое окно на тех же сообщениях (визуально оно «уезжает» вверх к старым)
			m.msgOffset += len(msg.messages)
			// Clamp
			vm := m.visibleMessages()
			if m.msgOffset > len(m.messages)-vm {
				m.msgOffset = len(m.messages) - vm
			}
			if m.msgOffset < 0 {
				m.msgOffset = 0
			}
		}
		return m, nil

	case messageSentMsg:
		m.sending = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.messages = append(m.messages, msg.entry)
		return m, nil

	case newIncomingMsg:
		// 1. Обновить диалог в списке (превью, дата, unread, пересортировка)
		found := -1
		for i := range m.dialogs {
			if m.dialogs[i].key == msg.peerKey {
				found = i
				break
			}
		}
		if found >= 0 {
			m.dialogs[found].lastMsg = msg.entry.text
			m.dialogs[found].lastDate = int(msg.entry.date.Unix())
			isCurrent := m.screen == screenChat && m.selected != nil && inputPeerKey(m.selected.peer) == msg.peerKey
			if !msg.entry.out && !isCurrent {
				m.dialogs[found].unread++
			}

			// Запомнить, на ком стоял курсор в списке, чтобы не потерять
			var cursorKey string
			if m.cursor < len(m.dialogs) {
				cursorKey = m.dialogs[m.cursor].key
			}
			sort.SliceStable(m.dialogs, func(i, j int) bool {
				return m.dialogs[i].lastDate > m.dialogs[j].lastDate
			})
			if cursorKey != "" {
				for i, d := range m.dialogs {
					if d.key == cursorKey {
						m.cursor = i
						m.adjustOffset()
						break
					}
				}
			}
		}

		// 2. Если этот чат сейчас открыт — добавить сообщение и сбросить unread
		if m.screen == screenChat && m.selected != nil && inputPeerKey(m.selected.peer) == msg.peerKey {
			wasAtBottom := m.msgOffset == 0
			m.messages = append(m.messages, msg.entry)
			if !wasAtBottom {
				// держим окно прокрутки на том же месте
				m.msgOffset++
			}
			// сбросить unread у текущего диалога
			for i := range m.dialogs {
				if m.dialogs[i].key == msg.peerKey {
					m.dialogs[i].unread = 0
					break
				}
			}
		}
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil
	}

	// Pass-through updates for spinner/inputs
	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.spin, cmd = m.spin.Update(msg)
	cmds = append(cmds, cmd)

	if m.screen == screenAuth && m.authInput.Focused() {
		m.authInput, cmd = m.authInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.screen == screenChat && m.msgInput.Focused() {
		m.msgInput, cmd = m.msgInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateAuth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.cancel()
		return m, tea.Quit
	}
	if msg.String() == "enter" {
		if m.auth == authNeedPhone || m.auth == authNeedCode || m.auth == authNeedPassword {
			val := strings.TrimSpace(m.authInput.Value())
			if val == "" {
				return m, nil
			}
			m.authInput.SetValue("")
			m.authInput.Blur()
			ch := m.inputCh
			m.auth = authLoading
			return m, func() tea.Msg {
				ch <- val
				return nil
			}
		}
	}
	var cmd tea.Cmd
	m.authInput, cmd = m.authInput.Update(msg)
	return m, cmd
}

func (m model) updateChatList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.vimMode == modeCommand {
		return m.updateCommand(msg)
	}
	// VISUAL
	switch msg.String() {
	case ":":
		m.vimMode = modeCommand
		m.cmdBuf = ""
		m.err = nil
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.adjustOffset()
		}
	case "down", "j":
		if m.cursor < len(m.dialogs)-1 {
			m.cursor++
			m.adjustOffset()
		}
	case "g", "home":
		m.cursor = 0
		m.listOffset = 0
	case "G", "end":
		m.cursor = len(m.dialogs) - 1
		m.adjustOffset()
	case "enter":
		if len(m.dialogs) == 0 {
			return m, nil
		}
		d := m.dialogs[m.cursor]
		m.selected = &d
		m.screen = screenChat
		m.messages = nil
		m.loadingMsg = true
		m.loadingMore = false
		m.hasMore = true
		m.msgOffset = 0
		m.vimMode = modeVisual // в чат входим в visual
		m.msgInput.SetValue("")
		m.msgInput.Blur()
		reqCh := m.reqCh
		return m, func() tea.Msg {
			reqCh <- openChatReq{peer: d.peer}
			return nil
		}
	}
	return m, nil
}

func (m model) updateChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.vimMode {
	case modeCommand:
		return m.updateCommand(msg)
	case modeEdit:
		return m.updateChatEdit(msg)
	default:
		return m.updateChatVisual(msg)
	}
}

func (m model) updateChatVisual(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	vm := m.visibleMessages()
	maxOffset := len(m.messages) - vm
	if maxOffset < 0 {
		maxOffset = 0
	}

	loadOlder := func() tea.Cmd {
		if !m.hasMore || m.loadingMore || len(m.messages) == 0 || m.selected == nil {
			return nil
		}
		oldest := m.messages[0].id
		peer := m.selected.peer
		reqCh := m.reqCh
		return func() tea.Msg {
			reqCh <- loadMoreReq{peer: peer, beforeID: oldest}
			return nil
		}
	}

	switch msg.String() {
	case ":":
		m.vimMode = modeCommand
		m.cmdBuf = ""
		m.err = nil
		return m, nil
	case "a", "i":
		m.vimMode = modeEdit
		m.msgInput.Focus()
		return m, textinput.Blink
	case "pgup", "ctrl+u":
		m.msgOffset += vm / 2
		if m.msgOffset >= maxOffset {
			m.msgOffset = maxOffset
			if cmd := loadOlder(); cmd != nil {
				m.loadingMore = true
				return m, cmd
			}
		}
	case "pgdown", "ctrl+d":
		m.msgOffset -= vm / 2
		if m.msgOffset < 0 {
			m.msgOffset = 0
		}
	case "k", "up":
		if m.msgOffset < maxOffset {
			m.msgOffset++
		} else if cmd := loadOlder(); cmd != nil {
			m.loadingMore = true
			return m, cmd
		}
	case "j", "down":
		if m.msgOffset > 0 {
			m.msgOffset--
		}
	case "g", "home":
		m.msgOffset = maxOffset
		if cmd := loadOlder(); cmd != nil {
			m.loadingMore = true
			return m, cmd
		}
	case "G", "end":
		m.msgOffset = 0
	}
	return m, nil
}

func (m model) updateChatEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.vimMode = modeVisual
		m.msgInput.Blur()
		return m, nil
	case "enter":
		text := strings.TrimSpace(m.msgInput.Value())
		if text == "" || m.sending || m.selected == nil {
			return m, nil
		}
		m.msgInput.SetValue("")
		m.sending = true
		m.msgOffset = 0
		peer := m.selected.peer
		reqCh := m.reqCh
		return m, func() tea.Msg {
			reqCh <- sendMsgReq{peer: peer, text: text}
			return nil
		}
	}
	var cmd tea.Cmd
	m.msgInput, cmd = m.msgInput.Update(msg)
	return m, cmd
}

func (m model) updateCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.vimMode = modeVisual
		m.cmdBuf = ""
		return m, nil
	case "enter":
		cmd := strings.TrimSpace(m.cmdBuf)
		m.cmdBuf = ""
		m.vimMode = modeVisual
		return m.executeCommand(cmd)
	case "backspace":
		if len(m.cmdBuf) > 0 {
			r := []rune(m.cmdBuf)
			m.cmdBuf = string(r[:len(r)-1])
		}
		return m, nil
	}
	if len(msg.Runes) > 0 {
		m.cmdBuf += string(msg.Runes)
	}
	return m, nil
}

func (m model) executeCommand(cmd string) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenChatList:
		switch cmd {
		case "q", "wq", "qa", "qw":
			m.cancel()
			return m, tea.Quit
		}
	case screenChat:
		switch cmd {
		case "q":
			// назад в список
			m.screen = screenChatList
			m.selected = nil
			m.messages = nil
			m.msgInput.Blur()
			m.msgInput.SetValue("")
			m.vimMode = modeVisual
			m.msgOffset = 0
			return m, nil
		case "wq", "qa", "qw":
			m.cancel()
			return m, tea.Quit
		}
	}
	if cmd != "" {
		m.err = fmt.Errorf("unknown command: :%s", cmd)
	}
	return m, nil
}

func (m *model) visibleRows() int {
	// header(2) + footer(2) + counter(1) + padding(1)
	h := m.height
	if h <= 0 {
		h = 24
	}
	v := h - 6
	if v < 3 {
		return 3
	}
	return v
}

// visibleMessages — сколько сообщений (по 1 строке каждое) влезает на экран чата.
func (m *model) visibleMessages() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	// title(2) + input(1) + status(1) + footer(1) + scroll-hints(2) + padding(1) = 8
	lines := h - 8
	if lines < 3 {
		lines = 3
	}
	return lines
}

func (m *model) adjustOffset() {
	vr := m.visibleRows()
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}
	if m.cursor >= m.listOffset+vr {
		m.listOffset = m.cursor - vr + 1
	}
}

// =====================================================================
// Styles
// =====================================================================

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5fafff")).MarginBottom(1)
	labelStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a0a0a0"))
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#5fff87"))
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff5f5f"))
	footerStyle  = lipgloss.NewStyle().Faint(true).MarginTop(1)
	dimStyle     = lipgloss.NewStyle().Faint(true)

	chipGroup   = lipgloss.NewStyle().Background(lipgloss.Color("#5f87d7")).Foreground(lipgloss.Color("#ffffff")).Padding(0, 1)
	chipChannel = lipgloss.NewStyle().Background(lipgloss.Color("#af87d7")).Foreground(lipgloss.Color("#ffffff")).Padding(0, 1)
	chipDM      = lipgloss.NewStyle().Background(lipgloss.Color("#5faf87")).Foreground(lipgloss.Color("#ffffff")).Padding(0, 1)

	selBg = lipgloss.NewStyle().Background(lipgloss.Color("#444444")).Foreground(lipgloss.Color("#ffffff"))

	outMsgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafff"))
	inMsgStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#dddddd"))
	unreadStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#5fafd7")).Padding(0, 1)

	modeVisualStyle  = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#5fafff")).Foreground(lipgloss.Color("#000000"))
	modeEditStyle    = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#5fff87")).Foreground(lipgloss.Color("#000000"))
	modeCommandStyle = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("#ffaf00")).Foreground(lipgloss.Color("#000000"))
	cmdLineStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))
)

func (m model) renderModeBadge() string {
	switch m.vimMode {
	case modeEdit:
		return modeEditStyle.Render(m.vimMode.label())
	case modeCommand:
		return modeCommandStyle.Render(m.vimMode.label())
	default:
		return modeVisualStyle.Render(m.vimMode.label())
	}
}

func (m model) renderCommandLine() string {
	if m.vimMode != modeCommand {
		return ""
	}
	return cmdLineStyle.Render(":" + m.cmdBuf + "█")
}

// =====================================================================
// View
// =====================================================================

func (m model) View() string {
	switch m.screen {
	case screenAuth:
		return m.viewAuth()
	case screenChatList:
		return m.viewChatList()
	case screenChat:
		return m.viewChat()
	}
	return ""
}

func (m model) viewAuth() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("vimgram · sign in"))
	b.WriteString("\n")

	switch m.auth {
	case authConnecting:
		b.WriteString(m.spin.View())
		b.WriteString(" Connecting to Telegram...")
	case authLoading:
		b.WriteString(m.spin.View())
		b.WriteString(" Processing...")
	case authNeedPhone:
		b.WriteString(labelStyle.Render("Phone number (with country code):"))
		b.WriteString("\n")
		b.WriteString(m.authInput.View())
	case authNeedCode:
		b.WriteString(labelStyle.Render("Telegram code:"))
		b.WriteString("\n")
		b.WriteString(m.authInput.View())
	case authNeedPassword:
		b.WriteString(labelStyle.Render("2FA password:"))
		b.WriteString("\n")
		b.WriteString(m.authInput.View())
	}

	if m.err != nil {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}

	b.WriteString("\n")
	switch m.auth {
	case authNeedPhone, authNeedCode, authNeedPassword:
		b.WriteString(footerStyle.Render("enter — submit · esc — quit"))
	default:
		b.WriteString(footerStyle.Render("esc — quit"))
	}
	return b.String()
}

func (m model) viewChatList() string {
	var b strings.Builder

	name := "you"
	if m.self != nil {
		name = strings.TrimSpace(m.self.FirstName + " " + m.self.LastName)
		if name == "" {
			name = "you"
		}
	}
	b.WriteString(titleStyle.Render(fmt.Sprintf("Chats · %s", name)))
	b.WriteString("\n")

	if len(m.dialogs) == 0 {
		b.WriteString(dimStyle.Render("No chats"))
	} else {
		vr := m.visibleRows()
		end := m.listOffset + vr
		if end > len(m.dialogs) {
			end = len(m.dialogs)
		}
		for i := m.listOffset; i < end; i++ {
			b.WriteString(m.renderDialogRow(i, i == m.cursor))
			b.WriteString("\n")
		}
		if len(m.dialogs) > vr {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  %d / %d", m.cursor+1, len(m.dialogs))))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	// status line: mode badge + command line / hints
	b.WriteString(m.renderModeBadge())
	b.WriteString("  ")
	if m.vimMode == modeCommand {
		b.WriteString(m.renderCommandLine())
	} else {
		b.WriteString(footerStyle.Render("j/k — move · g/G — edge · enter — open · : — command (:q :wq :qa)"))
	}
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}
	return b.String()
}

func (m model) renderDialogRow(idx int, selected bool) string {
	d := m.dialogs[idx]
	var chip string
	switch d.kind {
	case "Group":
		chip = chipGroup.Render("Group  ")
	case "Channel":
		chip = chipChannel.Render("Channel")
	default:
		chip = chipDM.Render("DM     ")
	}

	title := d.title
	if title == "" {
		title = "(no title)"
	}
	title = truncRunes(title, 28)

	// Преью — занимает остаток ширины
	maxPreview := m.width - 2 /*prefix*/ - 8 /*chip*/ - 30 /*title+padding*/ - 8 /*unread+padding*/
	if maxPreview < 10 {
		maxPreview = 10
	}
	preview := singleLine(d.lastMsg)
	preview = truncRunes(preview, maxPreview)

	unread := ""
	if d.unread > 0 {
		unread = unreadStyle.Render(strconv.Itoa(d.unread))
	}

	prefix := "  "
	if selected {
		prefix = "▸ "
	}

	// Одна строка: prefix chip  TITLE(28)  preview  unread
	titleCol := lipgloss.NewStyle().Width(30).Render(title)
	previewCol := dimStyle.Render(preview)

	line := prefix + chip + "  " + titleCol + previewCol
	if unread != "" {
		line += "  " + unread
	}
	if selected {
		return selBg.Render(line)
	}
	return line
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

func (m model) viewChat() string {
	var b strings.Builder

	title := "(chat)"
	kind := ""
	if m.selected != nil {
		title = m.selected.title
		kind = m.selected.kind
	}

	// Заголовок: "[Личка] Иван Иванов"  /  "[Группа] Название"
	kindPrefix := ""
	if kind != "" {
		kindPrefix = dimStyle.Render("["+kind+"] ")
	}
	b.WriteString(kindPrefix + titleStyle.Render(title))
	b.WriteString("\n")

	chatName := title
	selfName := ""
	if m.self != nil {
		selfName = strings.TrimSpace(m.self.FirstName + " " + m.self.LastName)
		if selfName == "" {
			selfName = "You"
		}
	}

	if m.loadingMsg {
		b.WriteString(m.spin.View())
		b.WriteString(" Loading messages...")
	} else if len(m.messages) == 0 {
		b.WriteString(dimStyle.Render("(empty)"))
	} else {
		vm := m.visibleMessages()
		end := len(m.messages) - m.msgOffset
		if end > len(m.messages) {
			end = len(m.messages)
		}
		if end < 0 {
			end = 0
		}
		start := end - vm
		if start < 0 {
			start = 0
		}

		// Индикатор "выше есть сообщения"
		if start > 0 {
			hint := fmt.Sprintf("↑ %d more above", start)
			if m.hasMore {
				hint += " (k/PgUp — load older)"
			}
			b.WriteString(dimStyle.Render(hint))
			b.WriteString("\n")
		} else if m.loadingMore {
			b.WriteString(m.spin.View())
			b.WriteString(dimStyle.Render(" loading older messages..."))
			b.WriteString("\n")
		} else if !m.hasMore && len(m.messages) > 0 {
			b.WriteString(dimStyle.Render("— start of chat —"))
			b.WriteString("\n")
		}

		for _, msg := range m.messages[start:end] {
			b.WriteString(renderMessageInline(msg, chatName, selfName, m.width))
			b.WriteString("\n")
		}

		// Индикатор "ниже есть сообщения"
		if end < len(m.messages) {
			b.WriteString(dimStyle.Render(fmt.Sprintf("↓ %d more below (G — bottom)", len(m.messages)-end)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	if m.sending {
		b.WriteString(m.spin.View())
		b.WriteString(" Sending...")
	} else if m.vimMode == modeEdit {
		b.WriteString(m.msgInput.View())
	} else {
		// in visual/command show passive draft line
		val := m.msgInput.Value()
		if val == "" {
			b.WriteString(dimStyle.Render("(press 'a' to type)"))
		} else {
			b.WriteString(dimStyle.Render("> " + val))
		}
	}

	b.WriteString("\n")
	// статус-строка
	b.WriteString(m.renderModeBadge())
	b.WriteString("  ")
	if m.vimMode == modeCommand {
		b.WriteString(m.renderCommandLine())
	} else {
		switch m.vimMode {
		case modeEdit:
			b.WriteString(footerStyle.Render("enter — send · esc — visual"))
		default:
			b.WriteString(footerStyle.Render("a — insert · j/k pgup/pgdn — scroll · g/G — edge · : — command (:q back, :wq/:qa quit)"))
		}
	}

	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}
	return b.String()
}

func renderMessageInline(msg messageEntry, chatName, selfName string, width int) string {
	ts := msg.date.Local().Format("15:04")

	sender := msg.fromName
	if sender == "" {
		if msg.out {
			sender = selfName
			if sender == "" {
				sender = "You"
			}
		} else {
			sender = chatName
		}
	}

	body := singleLine(msg.text)
	if body == "" {
		body = "(empty)"
	}

	// формат: [HH:MM] Имя: текст
	timeS := dimStyle.Render("[" + ts + "]")
	var nameS string
	if msg.out {
		nameS = outMsgStyle.Bold(true).Render(sender)
	} else {
		nameS = inMsgStyle.Bold(true).Render(sender)
	}

	line := timeS + " " + nameS + ": " + body

	// обрезаем по ширине терминала (грубо — по рунам)
	maxW := width - 2
	if maxW > 20 {
		line = truncRendered(line, maxW)
	}
	return line
}

// truncRendered обрезает строку с ANSI-кодами по визуальной ширине (приблизительно — по рунам без учёта кодов).
// Для простоты используем lipgloss.Width.
func truncRendered(s string, max int) string {
	w := lipgloss.Width(s)
	if w <= max {
		return s
	}
	// грубое усечение: режем сырую строку до max рун, добавляя многоточие
	// для большинства случаев достаточно
	r := []rune(s)
	if len(r) > max {
		r = r[:max-1]
		return string(r) + "…"
	}
	return s
}

// =====================================================================
// Background telegram worker
// =====================================================================

func runTelegram(
	ctx context.Context,
	send func(tea.Msg),
	inputCh chan string,
	reqCh chan any,
	appID int,
	appHash string,
) {
	defer func() {
		if r := recover(); r != nil {
			send(errMsg{err: fmt.Errorf("panic: %v", r)})
		}
	}()

	dispatcher := tg.NewUpdateDispatcher()

	handleMsg := func(mm *tg.Message, ent tg.Entities) {
		entry := messageEntry{
			out:  mm.Out,
			text: mm.Message,
			date: time.Unix(int64(mm.Date), 0),
		}
		if entry.text == "" {
			entry.text = "[media]"
		}
		if fromPeer, ok := mm.GetFromID(); ok {
			if pu, ok := fromPeer.(*tg.PeerUser); ok {
				if u, ok := ent.Users[pu.UserID]; ok {
					entry.fromName = strings.TrimSpace(u.FirstName + " " + u.LastName)
				}
			}
		}
		send(newIncomingMsg{peerKey: peerKey(mm.PeerID), entry: entry})
	}

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		if mm, ok := u.Message.(*tg.Message); ok {
			handleMsg(mm, e)
		}
		return nil
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		if mm, ok := u.Message.(*tg.Message); ok {
			handleMsg(mm, e)
		}
		return nil
	})

	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: "session.json"},
		UpdateHandler:  dispatcher,
	})

	a := &teaAuth{send: send, ch: inputCh}
	flow := auth.NewFlow(a, auth.SendCodeOptions{})

	err := client.Run(ctx, func(ctx context.Context) error {
		send(connectedMsg{})
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return err
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("self: %w", err)
		}

		dialogs, err := fetchDialogs(ctx, client)
		if err != nil {
			return fmt.Errorf("dialogs: %w", err)
		}
		send(dialogsLoadedMsg{self: self, dialogs: dialogs})

		// Event loop
		for {
			select {
			case <-ctx.Done():
				return nil
			case req := <-reqCh:
				switch r := req.(type) {
				case openChatReq:
					msgs, more, err := fetchHistory(ctx, client, r.peer, 0)
					if err != nil {
						send(errMsg{err: err})
						continue
					}
					send(messagesLoadedMsg{messages: msgs, hasMore: more})
				case loadMoreReq:
					msgs, more, err := fetchHistory(ctx, client, r.peer, r.beforeID)
					if err != nil {
						send(errMsg{err: err})
						continue
					}
					send(messagesPrependMsg{messages: msgs, hasMore: more})
				case sendMsgReq:
					entry, err := sendMessage(ctx, client, r.peer, r.text, self)
					send(messageSentMsg{entry: entry, err: err})
				}
			}
		}
	})

	if err != nil && ctx.Err() == nil {
		send(errMsg{err: err})
	}
}

// =====================================================================
// Telegram helpers
// =====================================================================

func peerKey(p tg.PeerClass) string {
	switch x := p.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("u:%d", x.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("c:%d", x.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("ch:%d", x.ChannelID)
	}
	return ""
}

func inputPeerKey(p tg.InputPeerClass) string {
	switch x := p.(type) {
	case *tg.InputPeerUser:
		return fmt.Sprintf("u:%d", x.UserID)
	case *tg.InputPeerChat:
		return fmt.Sprintf("c:%d", x.ChatID)
	case *tg.InputPeerChannel:
		return fmt.Sprintf("ch:%d", x.ChannelID)
	}
	return ""
}

const dialogsPageSize = 100

func fetchDialogs(ctx context.Context, client *telegram.Client) ([]dialogEntry, error) {
	var all []dialogEntry
	seen := map[string]bool{}

	offsetDate := 0
	offsetID := 0
	var offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}

	for page := 0; page < 50; page++ { // safety: до 5000 диалогов максимум
		raw, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      dialogsPageSize,
			OffsetPeer: offsetPeer,
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
		})
		if err != nil {
			return nil, err
		}

		entries, nextDate, nextID, nextPeer, raw_dialog_count, err := processDialogsPage(raw)
		if err != nil {
			return nil, err
		}

		added := 0
		for _, e := range entries {
			if seen[e.key] {
				continue
			}
			seen[e.key] = true
			all = append(all, e)
			added++
		}

		// Стоп: либо сервер вернул меньше страницы, либо мы не добавили ничего нового
		if raw_dialog_count < dialogsPageSize || added == 0 || nextPeer == nil {
			break
		}

		offsetDate = nextDate
		offsetID = nextID
		offsetPeer = nextPeer
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].lastDate > all[j].lastDate })
	return all, nil
}

// processDialogsPage парсит одну страницу ответа MessagesGetDialogs и возвращает
// записи + курсор для следующей страницы (date/id/peer самого старого диалога).
func processDialogsPage(raw tg.MessagesDialogsClass) (
	entries []dialogEntry,
	nextDate int,
	nextID int,
	nextPeer tg.InputPeerClass,
	rawCount int,
	err error,
) {
	var dialogs []tg.DialogClass
	var chats []tg.ChatClass
	var users []tg.UserClass
	var messages []tg.MessageClass

	switch d := raw.(type) {
	case *tg.MessagesDialogs:
		dialogs, chats, users, messages = d.Dialogs, d.Chats, d.Users, d.Messages
	case *tg.MessagesDialogsSlice:
		dialogs, chats, users, messages = d.Dialogs, d.Chats, d.Users, d.Messages
	default:
		return nil, 0, 0, nil, 0, fmt.Errorf("unexpected dialogs response: %T", raw)
	}

	userByID := map[int64]*tg.User{}
	for _, u := range users {
		if uu, ok := u.(*tg.User); ok {
			userByID[uu.ID] = uu
		}
	}
	chatByID := map[int64]*tg.Chat{}
	channelByID := map[int64]*tg.Channel{}
	for _, c := range chats {
		switch cc := c.(type) {
		case *tg.Chat:
			chatByID[cc.ID] = cc
		case *tg.Channel:
			channelByID[cc.ID] = cc
		}
	}

	lastByPeer := map[string]*tg.Message{}
	for _, m := range messages {
		mm, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		k := peerKey(mm.PeerID)
		if cur, ok := lastByPeer[k]; !ok || mm.Date > cur.Date {
			lastByPeer[k] = mm
		}
	}

	out := make([]dialogEntry, 0, len(dialogs))
	var lastEntry *dialogEntry
	var lastTopMsgID int

	for _, d := range dialogs {
		dd, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}
		entry := dialogEntry{unread: dd.UnreadCount}
		switch p := dd.Peer.(type) {
		case *tg.PeerUser:
			u, ok := userByID[p.UserID]
			if !ok {
				continue
			}
			entry.kind = "DM"
			entry.title = strings.TrimSpace(u.FirstName + " " + u.LastName)
			if entry.title == "" {
				entry.title = "(no name)"
			}
			entry.peer = &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}
		case *tg.PeerChat:
			c, ok := chatByID[p.ChatID]
			if !ok {
				continue
			}
			entry.kind = "Group"
			entry.title = c.Title
			entry.peer = &tg.InputPeerChat{ChatID: c.ID}
		case *tg.PeerChannel:
			c, ok := channelByID[p.ChannelID]
			if !ok {
				continue
			}
			if c.Broadcast {
				entry.kind = "Channel"
			} else {
				entry.kind = "Group"
			}
			entry.title = c.Title
			entry.peer = &tg.InputPeerChannel{ChannelID: c.ID, AccessHash: c.AccessHash}
		default:
			continue
		}

		entry.key = peerKey(dd.Peer)

		if last, ok := lastByPeer[entry.key]; ok {
			entry.lastDate = last.Date
			entry.lastMsg = singleLine(last.Message)
			if entry.lastMsg == "" {
				entry.lastMsg = "[media]"
			}
		}

		out = append(out, entry)

		// Сохраняем последний обработанный диалог для пагинации
		e := out[len(out)-1]
		lastEntry = &e
		lastTopMsgID = dd.TopMessage
	}

	rawCount = len(dialogs)
	if lastEntry != nil {
		nextDate = lastEntry.lastDate
		nextID = lastTopMsgID
		nextPeer = lastEntry.peer
	}
	return out, nextDate, nextID, nextPeer, rawCount, nil
}

const historyPageSize = 50

func fetchHistory(ctx context.Context, client *telegram.Client, peer tg.InputPeerClass, beforeID int) ([]messageEntry, bool, error) {
	req := &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: historyPageSize,
	}
	if beforeID > 0 {
		req.OffsetID = beforeID
	}
	raw, err := client.API().MessagesGetHistory(ctx, req)
	if err != nil {
		return nil, false, err
	}

	var msgs []tg.MessageClass
	var users []tg.UserClass

	switch r := raw.(type) {
	case *tg.MessagesMessages:
		msgs, users = r.Messages, r.Users
	case *tg.MessagesMessagesSlice:
		msgs, users = r.Messages, r.Users
	case *tg.MessagesChannelMessages:
		msgs, users = r.Messages, r.Users
	default:
		return nil, false, fmt.Errorf("unexpected history response: %T", raw)
	}

	userByID := map[int64]*tg.User{}
	for _, u := range users {
		if uu, ok := u.(*tg.User); ok {
			userByID[uu.ID] = uu
		}
	}

	out := make([]messageEntry, 0, len(msgs))
	for _, m := range msgs {
		mm, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		entry := messageEntry{
			id:   mm.ID,
			out:  mm.Out,
			text: mm.Message,
			date: time.Unix(int64(mm.Date), 0),
		}
		if entry.text == "" {
			entry.text = "[media]"
		}
		if fromPeer, ok := mm.GetFromID(); ok {
			if pu, ok := fromPeer.(*tg.PeerUser); ok {
				if u, ok := userByID[pu.UserID]; ok {
					entry.fromName = strings.TrimSpace(u.FirstName + " " + u.LastName)
				}
			}
		}
		out = append(out, entry)
	}

	// API returns newest-first → reverse for chronological order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}

	hasMore := len(msgs) >= historyPageSize
	return out, hasMore, nil
}

func sendMessage(ctx context.Context, client *telegram.Client, peer tg.InputPeerClass, text string, self *tg.User) (messageEntry, error) {
	_, err := client.API().MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		RandomID: rand.Int63(),
	})
	if err != nil {
		return messageEntry{}, err
	}
	entry := messageEntry{
		out:  true,
		text: text,
		date: time.Now(),
	}
	if self != nil {
		entry.fromName = strings.TrimSpace(self.FirstName + " " + self.LastName)
	}
	return entry, nil
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(s)
}

// =====================================================================
// main
// =====================================================================

// Bundled Telegram app credentials. Can be overridden via .env (see below).
const (
	defaultAppID   = 31452204
	defaultAppHash = "7be152ba05c87019d22948ae3188b8e9"
)

func main() {
	appID := defaultAppID
	appHash := defaultAppHash

	// .env is an optional override; silently ignore missing/unreadable file.
	_ = loadEnv(".env")

	// Only override when BOTH fields are present and non-empty
	// (avoids mismatched id/hash pairs).
	envID := os.Getenv("TG_APP_ID")
	envHash := os.Getenv("TG_APP_HASH")
	if envID != "" && envHash != "" {
		id, err := strconv.Atoi(envID)
		if err != nil {
			die("TG_APP_ID must be a number: %v", err)
		}
		appID = id
		appHash = envHash
	}

	rand.Seed(time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputCh := make(chan string, 1)
	reqCh := make(chan any, 4)
	m := newModel(inputCh, reqCh, cancel)

	p := tea.NewProgram(m, tea.WithAltScreen())

	go runTelegram(ctx, p.Send, inputCh, reqCh, appID, appHash)

	if _, err := p.Run(); err != nil {
		die("tea: %v", err)
	}
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
