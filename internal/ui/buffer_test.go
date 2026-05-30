package ui

import (
	"testing"

	"vimgram/internal/app"
)

func dialog(key, title string) app.Dialog {
	return app.Dialog{Key: key, Title: title, Kind: app.KindDM}
}

func TestBufferStoreStartsWithChats(t *testing.T) {
	s := newBufferStore()
	if len(s.list) != 1 {
		t.Fatalf("new store should hold 1 buffer, got %d", len(s.list))
	}
	chats := s.find(chatListBufferID)
	if chats == nil || chats.kind != bufChatList || chats.name != "Chats" {
		t.Fatalf("first buffer must be the Chats list buffer, got %+v", chats)
	}
}

func TestBufferStoreAddAndFind(t *testing.T) {
	s := newBufferStore()
	a := s.addChat(dialog("u:1", "Alice"))
	b := s.addChat(dialog("u:2", "Bob"))

	if a.id == b.id {
		t.Fatal("buffers must get distinct ids")
	}
	if s.find(a.id) != a || s.findByPeer("u:2") != b {
		t.Fatal("find / findByPeer returned the wrong buffer")
	}
	if s.findByPeer("nope") != nil {
		t.Fatal("findByPeer on unknown key should be nil")
	}
}

func TestBufferStoreDelete(t *testing.T) {
	s := newBufferStore()
	a := s.addChat(dialog("u:1", "Alice"))

	if s.delete(chatListBufferID) {
		t.Fatal("the Chats buffer must not be deletable")
	}
	if !s.delete(a.id) {
		t.Fatal("deleting an existing chat buffer should succeed")
	}
	if s.find(a.id) != nil || s.findByPeer("u:1") != nil {
		t.Fatal("deleted buffer should be gone from both indexes")
	}
}

func TestBufferStoreNextPrevWrap(t *testing.T) {
	s := newBufferStore() // id 1 (Chats)
	a := s.addChat(dialog("u:1", "Alice"))
	b := s.addChat(dialog("u:2", "Bob"))

	// order: [Chats(1), Alice, Bob]
	if got := s.next(chatListBufferID); got != a.id {
		t.Errorf("next(Chats) = %d, want %d", got, a.id)
	}
	if got := s.next(b.id); got != chatListBufferID {
		t.Errorf("next(last) should wrap to Chats, got %d", got)
	}
	if got := s.prev(chatListBufferID); got != b.id {
		t.Errorf("prev(Chats) should wrap to last, got %d", got)
	}
	if got := s.prev(a.id); got != chatListBufferID {
		t.Errorf("prev(Alice) = %d, want %d", got, chatListBufferID)
	}
}
