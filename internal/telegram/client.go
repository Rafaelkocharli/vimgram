package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

// Event is the marker type for events emitted by the client to the UI.
type Event interface{ isEvent() }

type (
	// EventConnected fires once after the underlying connection is up.
	EventConnected struct{}
	// EventDialogsLoaded fires after the initial dialog list is fetched.
	EventDialogsLoaded struct {
		Self    app.Self
		Dialogs []app.Dialog
	}
	// EventMessagesLoaded fires after history is loaded for a chat.
	EventMessagesLoaded struct {
		PeerKey  string
		Messages []app.Message
		HasMore  bool
	}
	// EventMessagesPrepended fires after older history is fetched.
	EventMessagesPrepended struct {
		PeerKey  string
		Messages []app.Message
		HasMore  bool
	}
	// EventMessageSent fires after an outgoing message attempt completes.
	EventMessageSent struct {
		PeerKey string
		Message app.Message
		Err     error
	}
	// EventMessageReceived fires for incoming messages (from updates).
	EventMessageReceived struct {
		PeerKey string
		Message app.Message
	}
	// EventUserStatus fires when a user's online presence changes.
	EventUserStatus struct {
		UserID int64
		Status app.UserStatus
	}
	// EventUserTyping fires when a user starts typing in a DM.
	EventUserTyping struct {
		UserID int64
	}
	// EventMessageEdited fires after a message-edit attempt completes.
	EventMessageEdited struct {
		PeerKey string
		MsgID   int
		NewText string
		Err     error
	}
	// EventMessageDeleted fires after a message-delete attempt completes.
	EventMessageDeleted struct {
		PeerKey string
		MsgID   int
		Err     error
	}
	// EventError signals a non-fatal background error.
	EventError struct{ Err error }
)

func (EventConnected) isEvent()        {}
func (EventDialogsLoaded) isEvent()    {}
func (EventMessagesLoaded) isEvent()   {}
func (EventMessagesPrepended) isEvent() {}
func (EventMessageSent) isEvent()      {}
func (EventMessageReceived) isEvent()  {}
func (EventMessageEdited) isEvent()   {}
func (EventMessageDeleted) isEvent()  {}
func (EventUserStatus) isEvent()      {}
func (EventUserTyping) isEvent()       {}
func (EventError) isEvent()            {}

// Client is the high-level Telegram facade used by the UI.
type Client struct {
	appID    int
	appHash  string
	storage  session.Storage
	prompter AuthPrompter
	emit     func(Event)

	requests chan any
	self     app.Self
}

// NewClient constructs a Client. The session storage is where session data
// is persisted between runs.
func NewClient(appID int, appHash string, storage session.Storage) *Client {
	return &Client{
		appID:    appID,
		appHash:  appHash,
		storage:  storage,
		requests: make(chan any, 8),
	}
}

// SetPrompter attaches an AuthPrompter; must be called before Run.
func (c *Client) SetPrompter(p AuthPrompter) { c.prompter = p }

// SetEventSink attaches the function that receives events from the client.
// Safe to call from any goroutine; the function will be invoked from the
// telegram worker goroutine.
func (c *Client) SetEventSink(fn func(Event)) { c.emit = fn }

// OpenChat asks the client to load history for the given peer. limit hints
// how many messages to fetch so the viewport can be filled on first open.
func (c *Client) OpenChat(peer app.PeerRef, limit int) {
	c.requests <- openChatReq{peer: peer.(tg.InputPeerClass), limit: limit}
}

// LoadMore asks the client to load older history (messages with id < beforeID).
func (c *Client) LoadMore(peer app.PeerRef, beforeID int) {
	c.requests <- loadMoreReq{peer: peer.(tg.InputPeerClass), beforeID: beforeID}
}

// SendMessage queues an outgoing message to peer. Pass replyToMsgID > 0 to
// send as a reply to an existing message.
// ForwardMessage forwards a message from srcPeer to dstPeer.
func (c *Client) ForwardMessage(srcPeer app.PeerRef, dstPeer app.PeerRef, msgID int) {
	c.requests <- forwardMsgReq{
		srcPeer: srcPeer.(tg.InputPeerClass),
		dstPeer: dstPeer.(tg.InputPeerClass),
		msgID:   msgID,
	}
}

// DeleteMessage queues deletion of a message. revoke=true deletes for everyone.
func (c *Client) DeleteMessage(peer app.PeerRef, msgID int, revoke bool) {
	c.requests <- deleteMsgReq{peer: peer.(tg.InputPeerClass), msgID: msgID, revoke: revoke}
}

// EditMessage queues an edit of an existing message.
func (c *Client) EditMessage(peer app.PeerRef, msgID int, newText string) {
	c.requests <- editMsgReq{peer: peer.(tg.InputPeerClass), msgID: msgID, newText: newText}
}

func (c *Client) SendMessage(peer app.PeerRef, text string, replyToMsgID int, replyToPreview string) {
	c.requests <- sendMsgReq{
		peer:           peer.(tg.InputPeerClass),
		text:           text,
		replyToMsgID:   replyToMsgID,
		replyToPreview: replyToPreview,
	}
}

// Internal request types passed via c.requests.
type (
	openChatReq struct {
		peer  tg.InputPeerClass
		limit int
	}
	loadMoreReq struct {
		peer     tg.InputPeerClass
		beforeID int
	}
	sendMsgReq struct {
		peer           tg.InputPeerClass
		text           string
		replyToMsgID   int
		replyToPreview string
	}
	editMsgReq struct {
		peer    tg.InputPeerClass
		msgID   int
		newText string
	}
	deleteMsgReq struct {
		peer   tg.InputPeerClass
		msgID  int
		revoke bool // true = delete for everyone
	}
	forwardMsgReq struct {
		srcPeer tg.InputPeerClass
		dstPeer tg.InputPeerClass
		msgID   int
	}
)

// Run connects to Telegram, performs auth if needed, loads the initial
// dialog list, then services UI requests until ctx is cancelled.
func (c *Client) Run(ctx context.Context) error {
	if c.prompter == nil || c.emit == nil {
		return fmt.Errorf("client not fully configured: prompter and event sink required")
	}

	rand.Seed(time.Now().UnixNano())

	dispatcher := buildDispatcher(updateHandlers{
		onMessage: func(peerKey string, m app.Message) {
			c.emit(EventMessageReceived{PeerKey: peerKey, Message: m})
		},
		onStatus: func(userID int64, status app.UserStatus) {
			c.emit(EventUserStatus{UserID: userID, Status: status})
		},
		onTyping: func(userID int64) {
			c.emit(EventUserTyping{UserID: userID})
		},
	})

	tgc := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		SessionStorage: c.storage,
		UpdateHandler:  dispatcher,
	})

	defer c.recoverPanic()

	return tgc.Run(ctx, func(ctx context.Context) error {
		c.emit(EventConnected{})

		flow := auth.NewFlow(userAuth{prompter: c.prompter}, auth.SendCodeOptions{})
		if err := tgc.Auth().IfNecessary(ctx, flow); err != nil {
			return fmt.Errorf("auth: %w", err)
		}

		me, err := tgc.Self(ctx)
		if err != nil {
			return fmt.Errorf("self: %w", err)
		}
		c.self = app.Self{ID: me.ID, FirstName: me.FirstName, LastName: me.LastName}

		dialogs, err := fetchDialogs(ctx, tgc)
		if err != nil {
			return err
		}
		c.emit(EventDialogsLoaded{Self: c.self, Dialogs: dialogs})

		return c.serveRequests(ctx, tgc)
	})
}

func (c *Client) recoverPanic() {
	if r := recover(); r != nil {
		c.emit(EventError{Err: fmt.Errorf("panic: %v", r)})
	}
}

// serveRequests is the main request loop after auth completes.
func (c *Client) serveRequests(ctx context.Context, tgc *telegram.Client) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case req := <-c.requests:
			c.handleRequest(ctx, tgc, req)
		}
	}
}

func (c *Client) handleRequest(ctx context.Context, tgc *telegram.Client, req any) {
	switch r := req.(type) {
	case openChatReq:
		msgs, more, err := fetchHistory(ctx, tgc, r.peer, 0, r.limit)
		if err != nil {
			c.emit(EventError{Err: err})
			return
		}
		c.emit(EventMessagesLoaded{PeerKey: InputPeerKey(r.peer), Messages: msgs, HasMore: more})
	case loadMoreReq:
		msgs, more, err := fetchHistory(ctx, tgc, r.peer, r.beforeID, historyPageSize)
		if err != nil {
			c.emit(EventError{Err: err})
			return
		}
		c.emit(EventMessagesPrepended{PeerKey: InputPeerKey(r.peer), Messages: msgs, HasMore: more})
	case sendMsgReq:
		msg, err := sendMessage(ctx, tgc, r.peer, r.text, r.replyToMsgID, r.replyToPreview, c.self)
		c.emit(EventMessageSent{PeerKey: InputPeerKey(r.peer), Message: msg, Err: err})
	case editMsgReq:
		err := editMessage(ctx, tgc, r.peer, r.msgID, r.newText)
		c.emit(EventMessageEdited{PeerKey: InputPeerKey(r.peer), MsgID: r.msgID, NewText: r.newText, Err: err})
	case deleteMsgReq:
		err := deleteMessage(ctx, tgc, r.msgID, r.revoke)
		c.emit(EventMessageDeleted{PeerKey: InputPeerKey(r.peer), MsgID: r.msgID, Err: err})
	case forwardMsgReq:
		if err := forwardMessage(ctx, tgc, r.srcPeer, r.dstPeer, r.msgID); err != nil {
			c.emit(EventError{Err: err})
		}
	}
}

// sendMessage submits an outgoing text message and returns the local
// representation that the UI can append optimistically.
func sendMessage(
	ctx context.Context,
	tgc *telegram.Client,
	peer tg.InputPeerClass,
	text string,
	replyToMsgID int,
	replyToPreview string,
	self app.Self,
) (app.Message, error) {
	req := &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		RandomID: rand.Int63(),
	}
	if replyToMsgID > 0 {
		req.ReplyTo = &tg.InputReplyToMessage{ReplyToMsgID: replyToMsgID}
	}
	_, err := tgc.API().MessagesSendMessage(ctx, req)
	if err != nil {
		return app.Message{}, fmt.Errorf("send: %w", err)
	}
	return app.Message{
		Out:          true,
		Text:         text,
		Date:         time.Now(),
		From:         strings.TrimSpace(self.FirstName + " " + self.LastName),
		NameColor:    -1,
		ReplyToID:    replyToMsgID,
		ReplyPreview: replyToPreview,
	}, nil
}

// deleteMessage removes a message. revoke=true deletes for all participants.
func deleteMessage(ctx context.Context, tgc *telegram.Client, msgID int, revoke bool) error {
	_, err := tgc.API().MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
		Revoke: revoke,
		ID:     []int{msgID},
	})
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// forwardMessage forwards a single message from srcPeer to dstPeer.
func forwardMessage(ctx context.Context, tgc *telegram.Client, srcPeer, dstPeer tg.InputPeerClass, msgID int) error {
	_, err := tgc.API().MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
		FromPeer: srcPeer,
		ToPeer:   dstPeer,
		ID:       []int{msgID},
		RandomID: []int64{rand.Int63()},
	})
	if err != nil {
		return fmt.Errorf("forward: %w", err)
	}
	return nil
}

// editMessage updates the text of an existing message via the Telegram API.
func editMessage(ctx context.Context, tgc *telegram.Client, peer tg.InputPeerClass, msgID int, newText string) error {
	_, err := tgc.API().MessagesEditMessage(ctx, &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      msgID,
		Message: newText,
	})
	if err != nil {
		return fmt.Errorf("edit: %w", err)
	}
	return nil
}
