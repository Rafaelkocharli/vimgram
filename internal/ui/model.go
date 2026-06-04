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

	// Buffers (loaded chats + the chat list) and the windows showing them.
	buffers      *bufferStore
	wins         []*window
	focused      int
	pendingCtrlW bool     // true after <C-w>, awaiting the direction key
	overlay      []string // non-empty => :ls buffer-list overlay is shown

	// Forward overlay: shown when the user presses f on a message.
	forwardActive  bool
	forwardSrcPeer app.PeerRef
	forwardMsgIDs  []int  // message IDs to forward (one or many)
	forwardMsgText string // text of the first source message, for the preview snippet
	forwardCursor  int
	forwardOffset  int

	// Visual mode selection anchor (chat view only).
	visualAnchor       int  // msgCursor position where v was pressed
	pendingVisualDelete bool // waiting for m/d/a after d in visual mode

	// Self + dialog list (global Telegram state, rendered by the Chats buffer).
	self        app.Self
	dialogs     []app.Dialog
	showArchive bool // whether to show archived chats (default: false)

	// Presence: per-user online state and typing expiry timestamps.
	statuses    map[int64]app.UserStatus
	typingUntil map[int64]time.Time

	// Widgets
	authInput textinput.Model
	msgInput  textinput.Model
	spin      spinner.Model

	// Discard-draft confirmation: true while waiting for the user to answer
	// "No write since last change. Discard draft? [y/N]".
	discardPrompt bool

	// Delete-message confirmation: true while waiting for "Delete message? [y/N]".
	// deleteRevoke=true means delete for everyone; false means only for self.
	pendingDelete bool // true after "d", awaiting m/d/a
	deletePrompt  bool
	deleteRevoke  bool

	// Unnamed yank register — holds the last yanked message text (yy).
	// Not connected to the system clipboard.
	pendingYank bool   // true after "y", awaiting second "y"
	yankReg     string // "" means empty

	// Vim marks: "m" then a letter sets a mark; "'" then a letter jumps to it.
	pendingMark bool // true after "m", awaiting the mark letter
	pendingJump bool // true after "'", awaiting the mark letter

	// Channels and shared state
	promptAnswers chan string
	err           error
	width, height int
}

// activeWindow returns the focused window.
func (m Model) activeWindow() *window { return m.wins[m.focused] }

// activeBuffer returns the buffer shown in the focused window.
func (m Model) activeBuffer() *buffer { return m.buffers.find(m.activeWindow().bufferID) }

// bufferOf returns the buffer a given window points at.
func (m Model) bufferOf(w *window) *buffer { return m.buffers.find(w.bufferID) }

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
		wins:          []*window{{bufferID: chatListBufferID}},
		focused:       0,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spin.Tick)
}
