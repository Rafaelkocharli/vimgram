package ui

import "vimgram/internal/app"

// bufferKind distinguishes the special chat-list buffer from chat buffers.
type bufferKind int

const (
	bufChatList bufferKind = iota
	bufChat
)

// chatListBufferID is the fixed id of the always-present "Chats" buffer.
const chatListBufferID = 1

// buffer is a loaded chat (or the chat list). Buffers exist independently of
// windows; a window only references a buffer by id.
type buffer struct {
	id   int
	kind bufferKind
	name string

	// chat buffers only:
	peer      app.PeerRef
	peerKey   string
	userID    int64
	kindLabel app.DialogKind

	messages    []app.Message
	hasMore     bool
	loadingMsg  bool
	loadingMore bool
	sending     bool
	draft       string // unsent compose text, preserved across buffer switches

	msgVersion int
	cache      *chatCache
}

// bufferStore is the ordered registry of buffers. Order drives :ls and
// :bnext/:bprev. The first entry is always the chat-list buffer.
type bufferStore struct {
	list   []*buffer
	nextID int
	byPeer map[string]int // peerKey -> buffer id
}

func newBufferStore() *bufferStore {
	chats := &buffer{id: chatListBufferID, kind: bufChatList, name: "Chats"}
	return &bufferStore{
		list:   []*buffer{chats},
		nextID: chatListBufferID + 1,
		byPeer: make(map[string]int),
	}
}

func (s *bufferStore) find(id int) *buffer {
	for _, b := range s.list {
		if b.id == id {
			return b
		}
	}
	return nil
}

func (s *bufferStore) findByPeer(peerKey string) *buffer {
	if id, ok := s.byPeer[peerKey]; ok {
		return s.find(id)
	}
	return nil
}

// addChat registers a new chat buffer and returns it.
func (s *bufferStore) addChat(d app.Dialog) *buffer {
	b := &buffer{
		id:        s.nextID,
		kind:      bufChat,
		name:      dialogName(d.Title),
		peer:      d.Peer,
		peerKey:   d.Key,
		userID:    d.UserID,
		kindLabel: d.Kind,
		hasMore:   true,
		cache:     &chatCache{},
	}
	s.nextID++
	s.list = append(s.list, b)
	s.byPeer[d.Key] = b.id
	return b
}

// delete removes a buffer by id. The chat-list buffer cannot be deleted.
func (s *bufferStore) delete(id int) bool {
	if id == chatListBufferID {
		return false
	}
	for i, b := range s.list {
		if b.id == id {
			delete(s.byPeer, b.peerKey)
			s.list = append(s.list[:i], s.list[i+1:]...)
			return true
		}
	}
	return false
}

func (s *bufferStore) index(id int) int {
	for i, b := range s.list {
		if b.id == id {
			return i
		}
	}
	return -1
}

// next returns the id of the buffer after id (wrapping). Falls back to id.
func (s *bufferStore) next(id int) int {
	i := s.index(id)
	if i < 0 || len(s.list) == 0 {
		return id
	}
	return s.list[(i+1)%len(s.list)].id
}

// prev returns the id of the buffer before id (wrapping). Falls back to id.
func (s *bufferStore) prev(id int) int {
	i := s.index(id)
	if i < 0 || len(s.list) == 0 {
		return id
	}
	return s.list[(i-1+len(s.list))%len(s.list)].id
}

func dialogName(title string) string {
	if title == "" {
		return "(no title)"
	}
	return title
}

// window is a viewport onto a buffer. It owns the scroll/cursor position so
// two windows can show the same buffer independently (vim semantics).
type window struct {
	bufferID  int
	altBuffer int // for :b#

	// per-window viewport state:
	lineOffset int // chat scroll, in visual lines from the bottom
	cursor     int // chat-list selection
	listOffset int // chat-list scroll

	// width is the inner content width this window was last rendered at.
	// Cached by the layout compositor so scroll math (which depends on word
	// wrapping) is available during key handling, before the next render.
	width int
}
