package ui

import "vimgram/internal/telegram"

// tea messages that bubble up from the AuthPrompter implementation.
type (
	needPhoneMsg    struct{}
	needCodeMsg     struct{}
	needPasswordMsg struct{}
)

// telegramEventMsg is a thin wrapper letting us pipe telegram.Event values
// through bubbletea's tea.Msg channel.
type telegramEventMsg struct{ Event telegram.Event }
