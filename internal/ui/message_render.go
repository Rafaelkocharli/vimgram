package ui

import (
	"strings"
	"time"

	"vimgram/internal/app"
)

const (
	minBodyWidth = 10
	// groupGap is the silence after which a message from the same sender gets
	// a fresh "[HH-MM] Name" header instead of being grouped with the previous.
	groupGap = 15 * time.Minute
)

// startsNewGroup reports whether cur should begin a new header block: when it
// is the first message, the sender changed, or more than groupGap elapsed
// since the previous message.
func startsNewGroup(prev *app.Message, cur app.Message) bool {
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
	bodyStyle := inMsgStyle
	if cur.Out {
		bodyStyle = outMsgStyle
	}

	var lines []string
	if withHeader {
		ts := cur.Date.Local().Format("15-04")
		name := senderName(cur, chatName, selfName)
		lines = append(lines, dimStyle.Render("["+ts+"] ")+bodyStyle.Bold(true).Render(name))
	}

	avail := clampMin(width, minBodyWidth)
	for _, chunk := range wrapText(bodyOrPlaceholder(cur.Text), avail) {
		lines = append(lines, bodyStyle.Render(chunk))
	}
	return lines
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
