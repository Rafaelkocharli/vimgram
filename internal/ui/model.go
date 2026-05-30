package ui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"vimgram/internal/app"
	"vimgram/internal/telegram"
)

// Model is the bubble-tea state. It composes domain state (current screen,
// dialogs, messages...) with bubbletea-specific widgets (textinput, spinner).
type Model struct {
	// Lifecycle
	client *telegram.Client
	cancel context.CancelFunc

	// Domain state
	authed    bool // true once signed in and the chat list is loaded
	vimMode   app.VimMode
	authState app.AuthState
	cmdBuf    string

	// Buffers (loaded chats + the chat list) and the single MVP window.
	buffers *bufferStore
	win     *window
	overlay []string // non-empty => :ls buffer-list overlay is shown

	// Self + dialog list (global Telegram state, rendered by the Chats buffer).
	self    app.Self
	dialogs []app.Dialog

	// Presence: per-user online state and typing expiry timestamps.
	statuses    map[int64]app.UserStatus
	typingUntil map[int64]time.Time

	// Widgets
	authInput textinput.Model
	msgInput  textinput.Model
	spin      spinner.Model

	// Channels and shared state
	promptAnswers chan string
	err           error
	width, height int
}

// activeWindow returns the focused window (single one in the MVP).
func (m Model) activeWindow() *window { return m.win }

// activeBuffer returns the buffer shown in the focused window.
func (m Model) activeBuffer() *buffer { return m.buffers.find(m.win.bufferID) }

// NewModel constructs a Model with all widgets initialized.
func NewModel(client *telegram.Client, cancel context.CancelFunc, answers chan string) Model {
	auth := textinput.New()
	auth.CharLimit = 64
	auth.Width = 40

	msg := textinput.New()
	msg.Placeholder = "Type a message..."
	msg.CharLimit = 2048
	msg.Width = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	store := newBufferStore()
	return Model{
		client:        client,
		cancel:        cancel,
		authState:     app.AuthConnecting,
		authInput:     auth,
		msgInput:      msg,
		spin:          sp,
		promptAnswers: answers,
		statuses:      make(map[int64]app.UserStatus),
		typingUntil:   make(map[int64]time.Time),
		buffers:       store,
		win:           &window{bufferID: chatListBufferID},
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}
