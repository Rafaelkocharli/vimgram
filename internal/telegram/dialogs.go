package telegram

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"vimgram/internal/app"
)

const dialogsPageSize = 100
const maxDialogPages = 50 // safety cap: up to ~5000 dialogs

// fetchDialogs iterates over MessagesGetDialogs pages until no more dialogs
// are returned, then sorts the result by last-message date (newest first).
func fetchDialogs(ctx context.Context, client *telegram.Client) ([]app.Dialog, error) {
	var (
		all  []app.Dialog
		seen = make(map[string]bool)

		offsetDate                 int
		offsetID                   int
		offsetPeer tg.InputPeerClass = &tg.InputPeerEmpty{}
	)

	for page := 0; page < maxDialogPages; page++ {
		raw, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			Limit:      dialogsPageSize,
			OffsetPeer: offsetPeer,
			OffsetDate: offsetDate,
			OffsetID:   offsetID,
		})
		if err != nil {
			return nil, fmt.Errorf("dialogs page %d: %w", page, err)
		}

		entries, cursor, raw_count, err := parseDialogsPage(raw)
		if err != nil {
			return nil, err
		}

		added := 0
		for _, e := range entries {
			if seen[e.Key] {
				continue
			}
			seen[e.Key] = true
			all = append(all, e)
			added++
		}

		if raw_count < dialogsPageSize || added == 0 || cursor.peer == nil {
			break
		}
		offsetDate = cursor.date
		offsetID = cursor.id
		offsetPeer = cursor.peer
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].LastDate > all[j].LastDate })
	return all, nil
}

// pageCursor is the offset for the next paginated request.
type pageCursor struct {
	date int
	id   int
	peer tg.InputPeerClass
}

// parseDialogsPage converts one MessagesGetDialogs response into Dialog
// entries plus the cursor needed for the next page.
func parseDialogsPage(raw tg.MessagesDialogsClass) ([]app.Dialog, pageCursor, int, error) {
	var (
		dialogs  []tg.DialogClass
		chats    []tg.ChatClass
		users    []tg.UserClass
		messages []tg.MessageClass
	)
	switch d := raw.(type) {
	case *tg.MessagesDialogs:
		dialogs, chats, users, messages = d.Dialogs, d.Chats, d.Users, d.Messages
	case *tg.MessagesDialogsSlice:
		dialogs, chats, users, messages = d.Dialogs, d.Chats, d.Users, d.Messages
	default:
		return nil, pageCursor{}, 0, fmt.Errorf("unexpected dialogs response: %T", raw)
	}

	byUser := indexUsers(users)
	byChat, byChannel := indexChatsAndChannels(chats)
	lastByPeer := indexLastMessages(messages)

	entries := make([]app.Dialog, 0, len(dialogs))
	var lastTopMsgID int
	var lastInputPeer tg.InputPeerClass

	for _, d := range dialogs {
		dd, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}
		entry, ok := buildDialogEntry(dd, byUser, byChat, byChannel)
		if !ok {
			continue
		}
		if last, ok := lastByPeer[entry.Key]; ok {
			entry.LastDate = last.Date
			entry.LastMsg = previewText(last)
		}
		entries = append(entries, entry)

		lastTopMsgID = dd.TopMessage
		lastInputPeer = entry.Peer.(tg.InputPeerClass)
	}

	cursor := pageCursor{}
	if len(entries) > 0 {
		cursor.date = entries[len(entries)-1].LastDate
		cursor.id = lastTopMsgID
		cursor.peer = lastInputPeer
	}
	return entries, cursor, len(dialogs), nil
}

func indexUsers(users []tg.UserClass) map[int64]*tg.User {
	out := make(map[int64]*tg.User, len(users))
	for _, u := range users {
		if uu, ok := u.(*tg.User); ok {
			out[uu.ID] = uu
		}
	}
	return out
}

func indexChatsAndChannels(chats []tg.ChatClass) (map[int64]*tg.Chat, map[int64]*tg.Channel) {
	byChat := make(map[int64]*tg.Chat)
	byChannel := make(map[int64]*tg.Channel)
	for _, c := range chats {
		switch cc := c.(type) {
		case *tg.Chat:
			byChat[cc.ID] = cc
		case *tg.Channel:
			byChannel[cc.ID] = cc
		}
	}
	return byChat, byChannel
}

func indexLastMessages(messages []tg.MessageClass) map[string]*tg.Message {
	out := make(map[string]*tg.Message)
	for _, m := range messages {
		mm, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		k := PeerKey(mm.PeerID)
		if cur, ok := out[k]; !ok || mm.Date > cur.Date {
			out[k] = mm
		}
	}
	return out
}

func buildDialogEntry(
	d *tg.Dialog,
	users map[int64]*tg.User,
	chats map[int64]*tg.Chat,
	channels map[int64]*tg.Channel,
) (app.Dialog, bool) {
	entry := app.Dialog{Unread: d.UnreadCount, Key: PeerKey(d.Peer)}
	switch p := d.Peer.(type) {
	case *tg.PeerUser:
		u, ok := users[p.UserID]
		if !ok {
			return entry, false
		}
		entry.Kind = app.KindDM
		entry.Title = displayName(u.FirstName, u.LastName)
		if entry.Title == "" {
			entry.Title = "(no name)"
		}
		entry.UserID = u.ID
		entry.Status = mapStatus(u.Status)
		entry.Peer = &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}
	case *tg.PeerChat:
		c, ok := chats[p.ChatID]
		if !ok {
			return entry, false
		}
		entry.Kind = app.KindGroup
		entry.Title = c.Title
		entry.Peer = &tg.InputPeerChat{ChatID: c.ID}
	case *tg.PeerChannel:
		c, ok := channels[p.ChannelID]
		if !ok {
			return entry, false
		}
		if c.Broadcast {
			entry.Kind = app.KindChannel
		} else {
			entry.Kind = app.KindGroup
		}
		entry.Title = c.Title
		entry.Peer = &tg.InputPeerChannel{ChannelID: c.ID, AccessHash: c.AccessHash}
	default:
		return entry, false
	}
	return entry, true
}

func displayName(first, last string) string {
	return strings.TrimSpace(first + " " + last)
}

func previewText(m *tg.Message) string {
	t := strings.ReplaceAll(m.Message, "\n", " ")
	t = strings.ReplaceAll(t, "\r", " ")
	t = strings.TrimSpace(t)
	if t == "" {
		return "[media]"
	}
	return t
}
