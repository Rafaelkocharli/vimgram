package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

const (
	historyPageSize = 50  // page size for incremental "load older" requests
	historyMaxLimit = 100 // Telegram's per-request cap for messages.getHistory
)

// fetchHistory loads up to `limit` messages from the given peer. The limit is
// clamped to [historyPageSize, historyMaxLimit]. If beforeID > 0, only
// messages older than that ID are returned. hasMore is true if the server
// returned a full page (i.e. more may exist).
func fetchHistory(ctx context.Context, client *telegram.Client, peer tg.InputPeerClass, beforeID, limit int) (
	[]app.Message, bool, error,
) {
	if limit < historyPageSize {
		limit = historyPageSize
	}
	if limit > historyMaxLimit {
		limit = historyMaxLimit
	}

	req := &tg.MessagesGetHistoryRequest{Peer: peer, Limit: limit}
	if beforeID > 0 {
		req.OffsetID = beforeID
	}

	raw, err := client.API().MessagesGetHistory(ctx, req)
	if err != nil {
		return nil, false, fmt.Errorf("history: %w", err)
	}

	rawMsgs, users, chats, err := extractHistoryPage(raw)
	if err != nil {
		return nil, false, err
	}

	byUser := indexUsers(users)
	_, byChannel := indexChatsAndChannels(chats)
	byID := indexMessageTexts(rawMsgs)
	messages := make([]app.Message, 0, len(rawMsgs))
	for _, m := range rawMsgs {
		mm, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		messages = append(messages, buildMessage(mm, byUser, byChannel, byID))
	}

	// API returns newest-first; flip to chronological.
	reverse(messages)
	hasMore := len(rawMsgs) >= limit
	return messages, hasMore, nil
}

func extractHistoryPage(raw tg.MessagesMessagesClass) ([]tg.MessageClass, []tg.UserClass, []tg.ChatClass, error) {
	switch r := raw.(type) {
	case *tg.MessagesMessages:
		return r.Messages, r.Users, r.Chats, nil
	case *tg.MessagesMessagesSlice:
		return r.Messages, r.Users, r.Chats, nil
	case *tg.MessagesChannelMessages:
		return r.Messages, r.Users, r.Chats, nil
	default:
		return nil, nil, nil, fmt.Errorf("unexpected history response: %T", raw)
	}
}

// buildMessage converts a gotd Message into the domain Message.
// repliesByID maps message id -> short text; nil is fine (no preview enrichment).
func buildMessage(mm *tg.Message, users map[int64]*tg.User, channels map[int64]*tg.Channel, repliesByID map[int]string) app.Message {
	m := app.Message{
		ID:        mm.ID,
		Out:       mm.Out,
		Text:      mm.Message,
		Date:      time.Unix(int64(mm.Date), 0),
		NameColor: -1,
	}
	if m.Text == "" {
		m.Text = "[media]"
	}
	if from, ok := mm.GetFromID(); ok {
		if pu, ok := from.(*tg.PeerUser); ok {
			if u, ok := users[pu.UserID]; ok {
				m.From = strings.TrimSpace(u.FirstName + " " + u.LastName)
				m.NameColor = peerColorIndex(u)
			}
		}
	}
	if fwd, ok := mm.GetFwdFrom(); ok {
		m.ForwardedFrom = fwdName(fwd, users, channels)
	}
	if rh, ok := mm.GetReplyTo(); ok {
		if mrh, ok := rh.(*tg.MessageReplyHeader); ok {
			if id, ok := mrh.GetReplyToMsgID(); ok && id > 0 {
				m.ReplyToID = id
				if repliesByID != nil {
					if t, ok := repliesByID[id]; ok {
						m.ReplyPreview = t
					}
				}
			}
		}
	}
	return m
}

// indexMessageTexts maps message id -> a short single-line snippet, used to
// fill the reply-to preview when both messages came in the same response.
func indexMessageTexts(raw []tg.MessageClass) map[int]string {
	out := make(map[int]string, len(raw))
	for _, m := range raw {
		mm, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		out[mm.ID] = snippet(mm.Message)
	}
	return out
}

// snippet condenses text to a single line and caps it for inline display.
func snippet(s string) string {
	t := strings.ReplaceAll(s, "\n", " ")
	t = strings.ReplaceAll(t, "\r", " ")
	t = strings.TrimSpace(t)
	if t == "" {
		return "[media]"
	}
	const max = 80
	r := []rune(t)
	if len(r) > max {
		return string(r[:max])
	}
	return t
}

func reverse(m []app.Message) {
	for i, j := 0, len(m)-1; i < j; i, j = i+1, j-1 {
		m[i], m[j] = m[j], m[i]
	}
}

// fwdName extracts a human-readable name from a MessageFwdHeader.
// It prefers the user's display name (looked up from the users index) over the
// raw FromName string that Telegram provides for channels and hidden accounts.
func fwdName(fwd tg.MessageFwdHeader, users map[int64]*tg.User, channels map[int64]*tg.Channel) string {
	if peer, ok := fwd.GetFromID(); ok {
		switch p := peer.(type) {
		case *tg.PeerUser:
			if u, ok := users[p.UserID]; ok {
				name := strings.TrimSpace(u.FirstName + " " + u.LastName)
				if name != "" {
					return name
				}
			}
		case *tg.PeerChannel:
			if c, ok := channels[p.ChannelID]; ok && c.Title != "" {
				return c.Title
			}
		}
	}
	if name, ok := fwd.GetFromName(); ok && name != "" {
		return name
	}
	return ""
}
