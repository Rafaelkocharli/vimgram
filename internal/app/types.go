// Package app holds the domain layer: framework-agnostic types used across
// the application (UI, telegram adapter, etc).
package app

import "time"

// VimMode is the editor-style modal state.
type VimMode int

const (
	ModeVisual VimMode = iota
	ModeEdit
	ModeCommand
)

// Label returns a short human label like " VISUAL ".
func (v VimMode) Label() string {
	switch v {
	case ModeEdit:
		return " INSERT "
	case ModeCommand:
		return " COMMAND "
	default:
		return " VISUAL "
	}
}

// AuthState reflects the stage of the sign-in flow.
type AuthState int

const (
	AuthConnecting AuthState = iota
	AuthNeedPhone
	AuthNeedCode
	AuthNeedPassword
	AuthLoading
)

// DialogKind classifies a chat.
type DialogKind string

const (
	KindDM      DialogKind = "DM"
	KindGroup   DialogKind = "Group"
	KindChannel DialogKind = "Channel"
)

// PeerRef is an opaque handle to a Telegram peer.
// The concrete type is owned by the telegram adapter; other layers must
// pass it back unchanged when requesting actions on this peer.
type PeerRef any

// UserStatus is a simplified presence state for a user.
type UserStatus int

const (
	StatusUnknown UserStatus = iota
	StatusOnline
	StatusOffline
)

// Dialog is one row in the chat list.
type Dialog struct {
	Key      string // stable identifier (peer kind + id)
	Title    string
	Kind     DialogKind
	Peer     PeerRef
	UserID   int64      // the peer's user id for DMs; 0 otherwise
	Status   UserStatus // initial presence for DMs
	LastMsg  string
	LastDate int // unix seconds
	Unread   int
}

// Message is one entry in a chat history.
type Message struct {
	ID   int
	Out  bool
	From string // sender's display name; empty for 1:1 incoming
	Text string
	Date time.Time
}

// Self is the authenticated user's profile snapshot.
type Self struct {
	ID        int64
	FirstName string
	LastName  string
}

// DisplayName joins FirstName + LastName; falls back to "you".
func (s Self) DisplayName() string {
	name := s.FirstName
	if s.LastName != "" {
		if name != "" {
			name += " "
		}
		name += s.LastName
	}
	if name == "" {
		return "you"
	}
	return name
}
