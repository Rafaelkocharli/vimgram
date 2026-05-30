package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"vimgram/internal/app"
)

const minBodyWidth = 10

// renderMessage produces a single (possibly multi-line) string for a message
// with the inline format "[HH:MM] Sender: text" and a hanging indent for
// any wrapped tail lines.
func renderMessage(msg app.Message, chatName, selfName string, width int) string {
	sender := pickSender(msg, chatName, selfName)
	body := bodyOrPlaceholder(msg.Text)

	prefix := buildPrefix(msg.Date.Local().Format("15:04"), sender, msg.Out)
	prefixWidth := lipgloss.Width(prefix)
	avail := clampMin(width-prefixWidth-1, minBodyWidth)

	chunks := wrapText(body, avail)
	if len(chunks) == 0 {
		return prefix
	}

	var out strings.Builder
	indent := strings.Repeat(" ", prefixWidth)
	for i, c := range chunks {
		if i == 0 {
			out.WriteString(prefix + c)
		} else {
			out.WriteString("\n" + indent + c)
		}
	}
	return out.String()
}

func pickSender(msg app.Message, chatName, selfName string) string {
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

func bodyOrPlaceholder(text string) string {
	t := singleLine(text)
	if t == "" {
		return "(empty)"
	}
	return t
}

func buildPrefix(timestamp, sender string, outgoing bool) string {
	timeS := dimStyle.Render("[" + timestamp + "]")
	var nameS string
	if outgoing {
		nameS = outMsgStyle.Bold(true).Render(sender)
	} else {
		nameS = inMsgStyle.Bold(true).Render(sender)
	}
	return timeS + " " + nameS + ": "
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
