package telegram

import (
	"testing"

	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

func TestMapStatus(t *testing.T) {
	tests := []struct {
		in   tg.UserStatusClass
		want app.UserStatus
	}{
		{&tg.UserStatusOnline{}, app.StatusOnline},
		{&tg.UserStatusOffline{}, app.StatusOffline},
		{&tg.UserStatusRecently{}, app.StatusOffline},
		{&tg.UserStatusLastWeek{}, app.StatusOffline},
		{&tg.UserStatusLastMonth{}, app.StatusOffline},
		{&tg.UserStatusEmpty{}, app.StatusUnknown},
		{nil, app.StatusUnknown},
	}
	for _, tc := range tests {
		if got := mapStatus(tc.in); got != tc.want {
			t.Errorf("mapStatus(%T) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsTypingAction(t *testing.T) {
	if !isTypingAction(&tg.SendMessageTypingAction{}) {
		t.Error("typing action should count as typing")
	}
	if isTypingAction(&tg.SendMessageCancelAction{}) {
		t.Error("cancel action should not count as typing")
	}
	if isTypingAction(nil) {
		t.Error("nil action should not count as typing")
	}
}

func TestBuildMessage(t *testing.T) {
	users := map[int64]*tg.User{1: {ID: 1, FirstName: "Ada", LastName: "Lovelace"}}

	t.Run("text with sender", func(t *testing.T) {
		mm := &tg.Message{ID: 10, Out: false, Message: "hello", Date: 1000}
		mm.SetFromID(&tg.PeerUser{UserID: 1})
		got := buildMessage(mm, users)
		if got.ID != 10 || got.Text != "hello" || got.From != "Ada Lovelace" {
			t.Errorf("buildMessage = %+v", got)
		}
		if got.Date.Unix() != 1000 {
			t.Errorf("date = %v", got.Date)
		}
	})

	t.Run("empty text becomes media placeholder", func(t *testing.T) {
		mm := &tg.Message{ID: 11, Message: ""}
		if got := buildMessage(mm, users); got.Text != "[media]" {
			t.Errorf("empty message should render as [media], got %q", got.Text)
		}
	})

	t.Run("outgoing flag", func(t *testing.T) {
		mm := &tg.Message{ID: 12, Out: true, Message: "yo"}
		if got := buildMessage(mm, users); !got.Out {
			t.Error("Out flag should be carried over")
		}
	})
}

func TestParseDialogsPage(t *testing.T) {
	raw := &tg.MessagesDialogs{
		Dialogs: []tg.DialogClass{
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}, TopMessage: 100, UnreadCount: 3},
			&tg.Dialog{Peer: &tg.PeerChat{ChatID: 2}, TopMessage: 200},
		},
		Users: []tg.UserClass{
			&tg.User{ID: 1, FirstName: "Ada"},
		},
		Chats: []tg.ChatClass{
			&tg.Chat{ID: 2, Title: "Devs"},
		},
		Messages: []tg.MessageClass{
			&tg.Message{ID: 100, Message: "hi there", Date: 10, PeerID: &tg.PeerUser{UserID: 1}},
			&tg.Message{ID: 200, Message: "", Date: 20, PeerID: &tg.PeerChat{ChatID: 2}},
		},
	}

	entries, _, count, err := parseDialogsPage(raw)
	if err != nil {
		t.Fatalf("parseDialogsPage error = %v", err)
	}
	if count != 2 || len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d (count %d)", len(entries), count)
	}

	dm := entries[0]
	if dm.Kind != app.KindDM || dm.Title != "Ada" || dm.Key != "u:1" {
		t.Errorf("DM entry wrong: %+v", dm)
	}
	if dm.Unread != 3 || dm.LastMsg != "hi there" {
		t.Errorf("DM unread/preview wrong: %+v", dm)
	}

	grp := entries[1]
	if grp.Kind != app.KindGroup || grp.Title != "Devs" || grp.Key != "c:2" {
		t.Errorf("group entry wrong: %+v", grp)
	}
	if grp.LastMsg != "[media]" {
		t.Errorf("empty last message should be [media], got %q", grp.LastMsg)
	}
}

func TestParseDialogsPageBroadcastChannel(t *testing.T) {
	raw := &tg.MessagesDialogs{
		Dialogs: []tg.DialogClass{
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 5}},
		},
		Chats: []tg.ChatClass{
			&tg.Channel{ID: 5, Title: "News", Broadcast: true},
		},
	}
	entries, _, _, err := parseDialogsPage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Kind != app.KindChannel {
		t.Errorf("broadcast channel should map to KindChannel, got %+v", entries)
	}
}

func TestParseDialogsPageUnexpectedType(t *testing.T) {
	if _, _, _, err := parseDialogsPage(&tg.MessagesDialogsNotModified{}); err == nil {
		t.Error("unexpected response type should error")
	}
}
