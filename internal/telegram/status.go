package telegram

import (
	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

// mapStatus reduces Telegram's many presence states to the three the UI
// cares about: online, offline, or unknown.
func mapStatus(s tg.UserStatusClass) app.UserStatus {
	switch s.(type) {
	case *tg.UserStatusOnline:
		return app.StatusOnline
	case *tg.UserStatusOffline,
		*tg.UserStatusRecently,
		*tg.UserStatusLastWeek,
		*tg.UserStatusLastMonth:
		return app.StatusOffline
	default:
		return app.StatusUnknown
	}
}

// peerColorIndex returns the Telegram peer-color palette index for a user, or
// -1 when the user has no explicit color set.
func peerColorIndex(u *tg.User) int {
	c, ok := u.GetColor()
	if !ok {
		return -1
	}
	pc, ok := c.(*tg.PeerColor)
	if !ok {
		return -1
	}
	if idx, ok := pc.GetColor(); ok {
		return idx
	}
	return -1
}

// isTypingAction reports whether a send-message action represents the user
// actively composing something (typing, recording, uploading…), as opposed
// to cancelling.
func isTypingAction(a tg.SendMessageActionClass) bool {
	switch a.(type) {
	case *tg.SendMessageCancelAction, nil:
		return false
	default:
		return true
	}
}
