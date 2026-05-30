package telegram

import (
	"context"

	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

// buildDispatcher wires up gotd's UpdateDispatcher to invoke onMessage
// for any new incoming message (DM/group via UpdateNewMessage, channel via
// UpdateNewChannelMessage).
func buildDispatcher(onMessage func(peerKey string, m app.Message)) tg.UpdateDispatcher {
	d := tg.NewUpdateDispatcher()

	handle := func(mm *tg.Message, ent tg.Entities) {
		onMessage(PeerKey(mm.PeerID), buildMessage(mm, ent.Users))
	}

	d.OnNewMessage(func(_ context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		if mm, ok := u.Message.(*tg.Message); ok {
			handle(mm, e)
		}
		return nil
	})
	d.OnNewChannelMessage(func(_ context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		if mm, ok := u.Message.(*tg.Message); ok {
			handle(mm, e)
		}
		return nil
	})
	return d
}
