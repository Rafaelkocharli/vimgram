package telegram

import (
	"context"

	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

// updateHandlers groups the callbacks the dispatcher invokes for the update
// types the UI reacts to.
type updateHandlers struct {
	onMessage func(peerKey string, m app.Message)
	onStatus  func(userID int64, status app.UserStatus)
	onTyping  func(userID int64)
}

// buildDispatcher wires gotd's UpdateDispatcher to the given handlers.
func buildDispatcher(h updateHandlers) tg.UpdateDispatcher {
	d := tg.NewUpdateDispatcher()

	handleMsg := func(mm *tg.Message, ent tg.Entities) {
		h.onMessage(PeerKey(mm.PeerID), buildMessage(mm, ent.Users))
	}

	d.OnNewMessage(func(_ context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		if mm, ok := u.Message.(*tg.Message); ok {
			handleMsg(mm, e)
		}
		return nil
	})
	d.OnNewChannelMessage(func(_ context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		if mm, ok := u.Message.(*tg.Message); ok {
			handleMsg(mm, e)
		}
		return nil
	})
	d.OnUserStatus(func(_ context.Context, _ tg.Entities, u *tg.UpdateUserStatus) error {
		h.onStatus(u.UserID, mapStatus(u.Status))
		return nil
	})
	d.OnUserTyping(func(_ context.Context, _ tg.Entities, u *tg.UpdateUserTyping) error {
		if isTypingAction(u.Action) {
			h.onTyping(u.UserID)
		}
		return nil
	})
	return d
}
