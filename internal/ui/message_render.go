package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"vimgram/internal/app"
)

const (
	minBodyWidth = 10
	// groupGap is the silence after which a message from the same sender gets
	// a fresh "[HH-MM] Name" header instead of being grouped with the previous.
	groupGap = 15 * time.Minute
)

// startsNewGroup reports whether cur should begin a new header block: when it
// is the first message, the sender changed, more than groupGap elapsed since
// the previous message, or cur is a reply/forward (those always carry a header
// so the hint can be shown inline).
func startsNewGroup(prev *app.Message, cur app.Message) bool {
	if cur.ReplyToID != 0 || cur.ForwardedFrom != "" {
		return true
	}
	if prev == nil {
		return true
	}
	if senderKey(*prev) != senderKey(cur) {
		return true
	}
	return cur.Date.Sub(prev.Date) > groupGap
}

// senderKey identifies who sent a message for grouping purposes.
func senderKey(msg app.Message) string {
	if msg.Out {
		return "\x00out"
	}
	if msg.From != "" {
		return "in:" + msg.From
	}
	return "in:" // a 1:1 chat has a single incoming sender
}

// senderName is the display name shown in a group header.
func senderName(msg app.Message, chatName, selfName string) string {
	if msg.From != "" {
		return msg.From
	}
	if msg.Out {
		if selfName != "" {
			return selfName
		}
		return "You"
	}
	return chatName
}

// renderMessageBlock renders one message as visual lines. When withHeader is
// true it is preceded by a "[HH-MM] Name" line; consecutive messages from the
// same sender omit the header and just show their (wrapped) text.
func renderMessageBlock(cur app.Message, chatName, selfName string, width int, withHeader bool) []string {
	var lines []string
	if withHeader {
		ts := cur.Date.Local().Format("15:04")
		name := senderName(cur, chatName, selfName)
		nameStyled := nameStyle(cur.NameColor, name).Render(name)
		header := dimStyle.Render("["+ts+"] ") + nameStyled
		room := width - lipgloss.Width(header) - 2
		if cur.ForwardedFrom != "" {
			header += "  " + dimStyle.Render(forwardHint(cur.ForwardedFrom, room))
		} else if cur.ReplyToID != 0 {
			header += "  " + dimStyle.Render(replyHint(cur.ReplyPreview, room))
		}
		lines = append(lines, header)
	}

	avail := clampMin(width, minBodyWidth)
	for _, chunk := range wrapText(bodyOrPlaceholder(cur.Text), avail) {
		lines = append(lines, msgTextStyle.Render(chunk))
	}
	return lines
}

// nameStyle returns the bold colored style for a sender name. It uses the
// Telegram palette index when known (idx >= 0); otherwise it derives a stable
// index from the name so different users still get different colors.
func nameStyle(idx int, name string) lipgloss.Style {
	if idx < 0 {
		idx = hashName(name)
	}
	c := namePalette[((idx%len(namePalette))+len(namePalette))%len(namePalette)]
	return lipgloss.NewStyle().Bold(true).Foreground(c)
}

// forwardHint renders the dim "forwarded from name" suffix for a header line.
func forwardHint(from string, room int) string {
	const prefix = "forwarded from "
	if room < len(prefix)+1 {
		return "fwd"
	}
	max := room - len(prefix)
	r := []rune(from)
	if len(r) > max {
		from = string(r[:max-1]) + "…"
	}
	return prefix + from
}

// replyHint renders the dim "replies to "snippet…"" suffix for a header,
// fitting within the remaining width of the header line. Unknown previews
// fall back to "…" so the hint is still visible.
func replyHint(preview string, room int) string {
	if room < 12 {
		room = 12
	}
	body := preview
	if body == "" {
		body = "…"
	}
	const prefix = `replies to "`
	const suffix = `"`
	max := room - len(prefix) - len(suffix)
	if max < 3 {
		// Not enough room for content; degrade to a marker we can still show.
		return "↩"
	}
	r := []rune(body)
	if len(r) > max {
		body = string(r[:max-1]) + "…"
	}
	return prefix + body + suffix
}

func hashName(s string) int {
	h := 0
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}

func bodyOrPlaceholder(text string) string {
	t := singleLine(text)
	if t == "" {
		return "(empty)"
	}
	return t
}

// wrapText performs a simple word-wrap to the given visual width.
// Single words longer than width are hard-cut by runes.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var lines []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			lines = append(lines, string(cur))
			cur = cur[:0]
		}
	}

	for _, word := range strings.Fields(s) {
		wr := []rune(word)
		if len(wr) > width {
			flush()
			for len(wr) > width {
				lines = append(lines, string(wr[:width]))
				wr = wr[width:]
			}
			if len(wr) > 0 {
				cur = append(cur, wr...)
			}
			continue
		}
		needed := len(wr)
		if len(cur) > 0 {
			needed++
		}
		if len(cur)+needed > width {
			flush()
			cur = append(cur, wr...)
			continue
		}
		if len(cur) > 0 {
			cur = append(cur, ' ')
		}
		cur = append(cur, wr...)
	}
	flush()
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
