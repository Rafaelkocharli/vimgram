package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gotd/td/tg"

	"vimgram/internal/app"
	"vimgram/internal/telegram"
)

// ----- test helpers -------------------------------------------------------

func apply(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func pressKey(m Model, msg tea.KeyMsg) Model { return apply(m, msg) }

// runColon enters command mode, types the command, and presses enter.
func runColon(m Model, cmd string) Model {
	m = pressKey(m, keyRunes(":"))
	for _, r := range cmd {
		m = pressKey(m, keyRunes(string(r)))
	}
	return pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
}

// authedModel returns a model that has signed in and loaded two dialogs, with
// a known terminal size, focused on the Chats buffer.
func authedModel(t *testing.T) Model {
	t.Helper()
	client := telegram.NewClient(0, "", nil)
	m := NewModel(client, func() {}, make(chan string, 1))
	m = apply(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = apply(m, telegramEventMsg{Event: telegram.EventDialogsLoaded{
		Self: app.Self{FirstName: "Me"},
		Dialogs: []app.Dialog{
			{Key: "u:1", Title: "Alice", Kind: app.KindDM, UserID: 1, Peer: &tg.InputPeerUser{UserID: 1}},
			{Key: "c:2", Title: "Devs", Kind: app.KindGroup, Peer: &tg.InputPeerChat{ChatID: 2}},
		},
	}})
	return m
}

// ----- auth flow ----------------------------------------------------------

func TestAuthFlow(t *testing.T) {
	client := telegram.NewClient(0, "", nil)
	m := NewModel(client, func() {}, make(chan string, 1))

	if m.authed {
		t.Fatal("model should start unauthenticated")
	}
	m = apply(m, telegramEventMsg{Event: telegram.EventConnected{}})
	if m.authState != app.AuthLoading {
		t.Errorf("after connect, authState = %v", m.authState)
	}
	m = apply(m, needPhoneMsg{})
	if m.authState != app.AuthNeedPhone || !m.authInput.Focused() {
		t.Error("needPhoneMsg should prompt for phone and focus the input")
	}
	// View must not panic while unauthenticated.
	if m.View() == "" {
		t.Error("auth view should render something")
	}
}

func TestDialogsLoadedEntersChatList(t *testing.T) {
	m := authedModel(t)
	if !m.authed {
		t.Fatal("should be authed after dialogs loaded")
	}
	if m.activeBuffer().kind != bufChatList {
		t.Fatal("active buffer should be the Chats list")
	}
	if len(m.dialogs) != 2 {
		t.Fatalf("expected 2 dialogs, got %d", len(m.dialogs))
	}
}

// ----- chat list navigation & opening -------------------------------------

func TestChatListCursorAndOpen(t *testing.T) {
	m := authedModel(t)

	m = pressKey(m, keyRunes("j"))
	if m.activeWindow().cursor != 1 {
		t.Fatalf("cursor should move to 1, got %d", m.activeWindow().cursor)
	}
	m = pressKey(m, keyRunes("k"))
	if m.activeWindow().cursor != 0 {
		t.Fatalf("cursor should move back to 0, got %d", m.activeWindow().cursor)
	}

	// Open the first dialog -> a chat buffer is created and focused.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	b := m.activeBuffer()
	if b.kind != bufChat || b.name != "Alice" {
		t.Fatalf("opening should focus the Alice chat buffer, got %+v", b)
	}
	if !b.loadingMsg {
		t.Error("freshly opened chat buffer should be loading")
	}
	if len(m.buffers.list) != 2 {
		t.Fatalf("expected Chats + Alice buffers, got %d", len(m.buffers.list))
	}
}

func TestOpeningSameChatReusesBuffer(t *testing.T) {
	m := authedModel(t)
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // open Alice
	aliceID := m.activeWindow().bufferID

	m = runColon(m, "b1")                            // back to Chats
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})  // open Alice again
	if m.activeWindow().bufferID != aliceID {
		t.Error("reopening the same chat must reuse its buffer, not create a new one")
	}
	if len(m.buffers.list) != 2 {
		t.Errorf("no duplicate buffer should be created, have %d", len(m.buffers.list))
	}
}

// ----- buffer commands ----------------------------------------------------

func TestBufferSwitchingCommands(t *testing.T) {
	m := authedModel(t)
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // open Alice (buffer 2)
	alice := m.activeWindow().bufferID

	m = runColon(m, "b1") // -> Chats
	if m.activeWindow().bufferID != chatListBufferID {
		t.Fatal(":b1 should switch to the Chats buffer")
	}
	m = runColon(m, "b#") // alternate -> back to Alice
	if m.activeWindow().bufferID != alice {
		t.Fatal(":b# should switch to the alternate buffer")
	}
	m = runColon(m, "bn") // next, wraps within [Chats, Alice]
	if m.activeWindow().bufferID != chatListBufferID {
		t.Fatalf(":bn from last should wrap to Chats, got %d", m.activeWindow().bufferID)
	}
}

func TestBufferDeleteRules(t *testing.T) {
	m := authedModel(t)
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // open Alice

	// Cannot delete Chats.
	m = runColon(m, "bd 1")
	if m.buffers.find(chatListBufferID) == nil {
		t.Fatal("the Chats buffer must survive :bd 1")
	}
	if m.err == nil {
		t.Error("deleting Chats should surface an error")
	}

	// Delete the active chat buffer -> falls back to Chats.
	m = runColon(m, "bd")
	if len(m.buffers.list) != 1 {
		t.Fatalf("Alice buffer should be gone, have %d buffers", len(m.buffers.list))
	}
	if m.activeBuffer().kind != bufChatList {
		t.Fatal("after deleting the visible buffer, window should fall back to Chats")
	}
}

func TestBufferListOverlay(t *testing.T) {
	m := authedModel(t)
	m = runColon(m, "ls")
	if len(m.overlay) == 0 {
		t.Fatal(":ls should populate the overlay")
	}
	if !strings.Contains(m.View(), ":buffers") {
		t.Error("overlay view should show the buffers header")
	}
	// Any key dismisses it.
	m = pressKey(m, keyRunes("j"))
	if len(m.overlay) != 0 {
		t.Error("any key should dismiss the overlay")
	}
}

// ----- windows / splits ---------------------------------------------------

func TestVerticalSplitAndFocus(t *testing.T) {
	m := authedModel(t)
	m = runColon(m, "vs") // split; focus stays on the left window
	if len(m.wins) != 2 {
		t.Fatalf("vsplit should create 2 windows, got %d", len(m.wins))
	}
	if m.focused != 0 {
		t.Errorf("focus should stay on the left window, got %d", m.focused)
	}
	// <C-w>l moves focus right.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlW})
	m = pressKey(m, keyRunes("l"))
	if m.focused != 1 {
		t.Errorf("<C-w>l should focus window 1, got %d", m.focused)
	}
	// Open Chats in the right window independently.
	m = runColon(m, "b1")
	if m.wins[1].bufferID != chatListBufferID {
		t.Error("right window should now show the Chats buffer")
	}
	// <C-w>h back to the left.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyCtrlW})
	m = pressKey(m, keyRunes("h"))
	if m.focused != 0 {
		t.Errorf("<C-w>h should focus window 0, got %d", m.focused)
	}
}

func TestCloseWindow(t *testing.T) {
	m := authedModel(t)
	m = runColon(m, "vs")
	m = runColon(m, "close")
	if len(m.wins) != 1 {
		t.Fatalf(":close should leave 1 window, got %d", len(m.wins))
	}
	// :close on the last window is a no-op (does not quit).
	m = runColon(m, "close")
	if len(m.wins) != 1 {
		t.Error(":close must not remove the last window")
	}
}

func TestViewRendersFullHeight(t *testing.T) {
	m := authedModel(t)
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 24 {
		t.Errorf("windowed view should be exactly 24 rows, got %d", len(lines))
	}
	// Split layout must also fill the height.
	m = runColon(m, "vs")
	lines = strings.Split(m.View(), "\n")
	if len(lines) != 24 {
		t.Errorf("split view should be exactly 24 rows, got %d", len(lines))
	}
}

// ----- message events ------------------------------------------------------

func TestMessagesRouteToBufferByPeer(t *testing.T) {
	m := authedModel(t)
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // open Alice (peer u:1)

	m = apply(m, telegramEventMsg{Event: telegram.EventMessagesLoaded{
		PeerKey:  "u:1",
		Messages: []app.Message{{ID: 1, Text: "hi"}},
		HasMore:  false,
	}})
	b := m.activeBuffer()
	if b.loadingMsg {
		t.Error("loaded event should clear the loading flag")
	}
	if len(b.messages) != 1 {
		t.Fatalf("expected 1 message in Alice buffer, got %d", len(b.messages))
	}
}

func TestIncomingMessageUnreadAndAppend(t *testing.T) {
	m := authedModel(t)
	// Open Alice, load history, then receive a new incoming message for Alice.
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = apply(m, telegramEventMsg{Event: telegram.EventMessagesLoaded{PeerKey: "u:1"}})
	m = apply(m, telegramEventMsg{Event: telegram.EventMessageReceived{
		PeerKey: "u:1",
		Message: app.Message{ID: 5, Text: "ping"},
	}})
	if got := len(m.activeBuffer().messages); got != 1 {
		t.Fatalf("incoming message should append to the active buffer, got %d", got)
	}

	// Incoming for a non-open chat -> unread on its dialog, no buffer needed.
	m = apply(m, telegramEventMsg{Event: telegram.EventMessageReceived{
		PeerKey: "c:2",
		Message: app.Message{ID: 6, Text: "yo"},
	}})
	idx := indexOfDialog(m.dialogs, "c:2")
	if idx < 0 || m.dialogs[idx].Unread != 1 {
		t.Errorf("incoming to a non-open chat should bump unread, got %+v", m.dialogs)
	}
}

// ----- presence ------------------------------------------------------------

func TestPresenceStatusInHeader(t *testing.T) {
	m := authedModel(t)
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // open Alice (DM, userID 1)
	m = apply(m, telegramEventMsg{Event: telegram.EventUserStatus{UserID: 1, Status: app.StatusOnline}})

	if !strings.Contains(m.View(), "(online)") {
		t.Error("header should show (online) for an online DM peer")
	}
	m = apply(m, telegramEventMsg{Event: telegram.EventUserTyping{UserID: 1}})
	if !strings.Contains(m.View(), "(typing...)") {
		t.Error("header should show (typing...) after a typing update")
	}
}

// ----- chat scrolling ------------------------------------------------------

func TestChatScrollByLine(t *testing.T) {
	m := authedModel(t)
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter})

	msgs := make([]app.Message, 100)
	for i := range msgs {
		msgs[i] = app.Message{ID: i + 1, Text: "line"}
	}
	m = apply(m, telegramEventMsg{Event: telegram.EventMessagesLoaded{PeerKey: "u:1", Messages: msgs}})

	if m.activeWindow().lineOffset != 0 {
		t.Fatal("should start at the bottom (offset 0)")
	}
	m = pressKey(m, keyRunes("k")) // scroll up one line
	if m.activeWindow().lineOffset != 1 {
		t.Errorf("k should scroll up one line, offset = %d", m.activeWindow().lineOffset)
	}
	m = pressKey(m, keyRunes("j")) // scroll back down
	if m.activeWindow().lineOffset != 0 {
		t.Errorf("j should scroll down one line, offset = %d", m.activeWindow().lineOffset)
	}
	m = pressKey(m, keyRunes("g")) // jump to oldest
	if m.activeWindow().lineOffset == 0 {
		t.Error("g should jump to the top (non-zero offset)")
	}
	m = pressKey(m, keyRunes("G")) // back to newest
	if m.activeWindow().lineOffset != 0 {
		t.Error("G should jump back to the bottom")
	}
}

// ----- quitting ------------------------------------------------------------

func TestQuitCommands(t *testing.T) {
	cancelled := false
	client := telegram.NewClient(0, "", nil)
	m := NewModel(client, func() { cancelled = true }, make(chan string, 1))
	m = apply(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = apply(m, telegramEventMsg{Event: telegram.EventDialogsLoaded{Self: app.Self{FirstName: "Me"}}})

	// :q with a single window quits the app (cancel is called).
	_ = runColon(m, "q")
	if !cancelled {
		t.Error(":q with one window should cancel/quit the app")
	}
}

func TestEnterInsertMode(t *testing.T) {
	m := authedModel(t)
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEnter}) // open Alice
	m = pressKey(m, keyRunes("a"))                  // enter INSERT
	if m.vimMode != app.ModeEdit {
		t.Fatal("'a' should enter INSERT mode")
	}
	m = pressKey(m, tea.KeyMsg{Type: tea.KeyEsc}) // back to VISUAL
	if m.vimMode != app.ModeVisual {
		t.Fatal("esc should return to VISUAL mode")
	}
}
