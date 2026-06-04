// Package app holds the domain layer: framework-agnostic types used across
// the application (UI, telegram adapter, etc).
package app

import (
	"fmt"
	"time"
)

// MediaKind classifies the type of media attached to a message.
type MediaKind int

const (
	MediaNone    MediaKind = iota
	MediaPhoto             // 📷
	MediaVideo             // 🎥
	MediaVoice             // 🎤
	MediaAudio             // 🎵
	MediaFile              // 📎
	MediaSticker           // 🙂
	MediaGIF               // 🎞
)

// Media holds metadata about a media attachment.
// Location is an opaque handle owned by the telegram layer; other layers
// must pass it back unchanged when requesting a download.
type Media struct {
	Kind      MediaKind
	FileID    int64  // Telegram file ID — used as the cache key
	Location  any    // tg.InputFileLocationClass, opaque outside telegram package
	Width     int    // photo / video
	Height    int    // photo / video
	DurationS int    // seconds: video / voice / audio / gif
	FileName  string // file attachments
	FileSize  int64  // bytes
	MimeType  string // for extension detection
}

// Label returns the human-readable one-line description shown in chat.
func (md Media) Label() string {
	switch md.Kind {
	case MediaPhoto:
		if md.Width > 0 && md.Height > 0 {
			return fmt.Sprintf("📷 Photo (%dx%d)", md.Width, md.Height)
		}
		return "📷 Photo"
	case MediaVideo:
		if md.DurationS > 0 {
			return "🎥 Video (" + fmtDuration(md.DurationS) + ")"
		}
		return "🎥 Video"
	case MediaVoice:
		if md.DurationS > 0 {
			return "🎤 Voice Message (" + fmtDuration(md.DurationS) + ")"
		}
		return "🎤 Voice Message"
	case MediaAudio:
		if md.DurationS > 0 {
			return "🎵 Audio (" + fmtDuration(md.DurationS) + ")"
		}
		return "🎵 Audio"
	case MediaFile:
		if md.FileName != "" && md.FileSize > 0 {
			return fmt.Sprintf("📎 %s (%s)", md.FileName, fmtSize(md.FileSize))
		}
		if md.FileName != "" {
			return "📎 " + md.FileName
		}
		return "📎 File"
	case MediaSticker:
		return "🙂 Sticker"
	case MediaGIF:
		if md.DurationS > 0 {
			return "🎞 GIF (" + fmtDuration(md.DurationS) + ")"
		}
		return "🎞 GIF"
	}
	return ""
}

// Ext returns the file extension (with dot) for the media based on MimeType,
// falling back to a sensible default for each kind.
func (md Media) Ext() string {
	switch md.MimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/ogg", "audio/opus":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "audio/aac":
		return ".aac"
	case "application/pdf":
		return ".pdf"
	}
	switch md.Kind {
	case MediaPhoto:
		return ".jpg"
	case MediaVideo, MediaGIF:
		return ".mp4"
	case MediaVoice:
		return ".ogg"
	case MediaAudio:
		return ".mp3"
	}
	return ""
}

func fmtDuration(s int) string {
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func fmtSize(b int64) string {
	const kb = 1024
	const mb = 1024 * kb
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// VimMode is the editor-style modal state.
type VimMode int

const (
	ModeNormal VimMode = iota // base mode (zero value)
	ModeVisual
	ModeEdit
	ModeCommand
)

// Label returns a short human label like " NORMAL ".
func (v VimMode) Label() string {
	switch v {
	case ModeVisual:
		return " VISUAL "
	case ModeEdit:
		return " INSERT "
	case ModeCommand:
		return " COMMAND "
	default:
		return " NORMAL "
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
	Archived bool // true when the dialog is in the Telegram archive folder
}

// Message is one entry in a chat history.
type Message struct {
	ID   int
	Out  bool
	From string // sender's display name; empty for 1:1 incoming
	Text string
	Date time.Time
	// NameColor is the Telegram peer-color palette index for the sender's
	// name, or -1 when unknown.
	NameColor int
	// ReplyToID is the id of the message this one replies to (0 if not a reply).
	ReplyToID int
	// ReplyPreview is a short snippet of the replied-to message, when known.
	ReplyPreview string
	// ForwardedFrom is the name of the original sender/chat when the message
	// was forwarded; empty if the message is not a forward.
	ForwardedFrom string
	// Media holds metadata when the message contains an attachment.
	// Kind == MediaNone means plain text.
	Media Media
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
