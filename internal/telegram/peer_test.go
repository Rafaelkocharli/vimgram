package telegram

import (
	"testing"

	"github.com/gotd/td/tg"
)

func TestPeerKey(t *testing.T) {
	tests := []struct {
		peer tg.PeerClass
		want string
	}{
		{&tg.PeerUser{UserID: 7}, "u:7"},
		{&tg.PeerChat{ChatID: 8}, "c:8"},
		{&tg.PeerChannel{ChannelID: 9}, "ch:9"},
		{nil, ""},
	}
	for _, tc := range tests {
		if got := PeerKey(tc.peer); got != tc.want {
			t.Errorf("PeerKey(%T) = %q, want %q", tc.peer, got, tc.want)
		}
	}
}

func TestInputPeerKey(t *testing.T) {
	tests := []struct {
		peer tg.InputPeerClass
		want string
	}{
		{&tg.InputPeerUser{UserID: 7}, "u:7"},
		{&tg.InputPeerChat{ChatID: 8}, "c:8"},
		{&tg.InputPeerChannel{ChannelID: 9}, "ch:9"},
		{&tg.InputPeerEmpty{}, ""},
	}
	for _, tc := range tests {
		if got := InputPeerKey(tc.peer); got != tc.want {
			t.Errorf("InputPeerKey(%T) = %q, want %q", tc.peer, got, tc.want)
		}
	}
}

// PeerKey and InputPeerKey must agree so updates match the dialog/buffer they
// belong to.
func TestPeerKeyConsistency(t *testing.T) {
	if PeerKey(&tg.PeerUser{UserID: 42}) != InputPeerKey(&tg.InputPeerUser{UserID: 42}) {
		t.Fatal("PeerKey and InputPeerKey disagree for the same user")
	}
}
