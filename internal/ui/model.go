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
	screen    app.Screen
	vimMode   app.VimMode
	authState app.AuthState
	cmdBuf    string

	// Self + chat list
	self       app.Self
	dialogs    []app.Dialog
	cursor     int
	listOffset int

	// Presence: per-user online state and typing expiry timestamps.
	statuses    map[int64]app.UserStatus
	typingUntil map[int64]time.Time

	// Chat view
	selected    *app.Dialog
	messages    []app.Message
	lineOffset  int // scroll position in visual lines from the bottom (0 = newest)
	loadingMsg  bool
	loadingMore bool
	hasMore     bool
	sending     bool

	// Widgets
	authInput textinput.Model
	msgInput  textinput.Model
	spin      spinner.Model

	// Channels and shared state
	promptAnswers chan string
	err           error
	width, height int
}

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

	return Model{
		client:        client,
		cancel:        cancel,
		screen:        app.ScreenAuth,
		authState:     app.AuthConnecting,
		authInput:     auth,
		msgInput:      msg,
		spin:          sp,
		promptAnswers: answers,
		statuses:      make(map[int64]app.UserStatus),
		typingUntil:   make(map[int64]time.Time),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}
