package telegram

import (
	"fmt"

	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

// PeerRefKey returns the stable key for an opaque PeerRef coming from the
// app layer. Safe because all PeerRefs we hand out are tg.InputPeerClass.
func PeerRefKey(p app.PeerRef) string {
	if ip, ok := p.(tg.InputPeerClass); ok {
		return InputPeerKey(ip)
	}
	return ""
}

// PeerKey returns a stable string key for a Peer (used to match updates
// against dialogs).
func PeerKey(p tg.PeerClass) string {
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

// InputPeerKey returns the same key form for an InputPeer.
func InputPeerKey(p tg.InputPeerClass) string {
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
