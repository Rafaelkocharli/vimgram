package ui

import "strings"

// chatCacheKey identifies a rendered history snapshot. If it is unchanged
// between frames, the flattened lines can be reused verbatim.
type chatCacheKey struct {
	version int
	width   int
	hasMore bool
}

// chatCache memoizes the flattened history lines (see chatLines).
type chatCache struct {
	key   chatCacheKey
	lines []string
}

// chatLines returns the flattened history of a chat buffer at the given width,
// memoized. Rendering every message through lipgloss on each frame was the
// dominant allocation source; the cache makes repeated frames essentially free.
func (m Model) chatLines(b *buffer, width int) []string {
	// While loading older messages the top line animates a spinner, so the
	// output changes every frame — skip the cache to keep it live.
	if b.loadingMore {
		return m.computeChatLines(b, width)
	}
	key := chatCacheKey{version: b.msgVersion, width: width, hasMore: b.hasMore}
	if b.cache != nil && b.cache.lines != nil && b.cache.key == key {
		return b.cache.lines
	}
	lines := m.computeChatLines(b, width)
	if b.cache != nil {
		b.cache.key = key
		b.cache.lines = lines
	}
	return lines
}

// computeChatLines flattens the buffer's whole loaded history into a flat
// slice of visual lines (one wrapped message can produce several). The first
// line is always a "top of history" marker.
func (m Model) computeChatLines(b *buffer, width int) []string {
	chatName, selfName := m.chatNames(b)

	lines := make([]string, 0, len(b.messages)+1)
	lines = append(lines, m.historyTopLine(b))
	for _, msg := range b.messages {
		rendered := renderMessage(msg, chatName, selfName, width)
		lines = append(lines, strings.Split(rendered, "\n")...)
	}
	return lines
}

// historyTopLine is the single marker line shown above the oldest message.
func (m Model) historyTopLine(b *buffer) string {
	switch {
	case b.loadingMore:
		return m.spin.View() + dimStyle.Render(" loading older messages...")
	case !b.hasMore:
		return dimStyle.Render("— start of chat —")
	default:
		return dimStyle.Render("↑ k — load older")
	}
}

// chatViewport returns exactly chatBodyHeight lines for a window's message
// area, bottom-anchored, padding the top when content is short.
func (m Model) chatViewport(b *buffer, w *window, width int) []string {
	all := m.chatLines(b, width)
	height := m.chatBodyHeight()
	total := len(all)

	bottom := total - w.lineOffset
	if bottom > total {
		bottom = total
	}
	if bottom < 0 {
		bottom = 0
	}
	top := bottom - height

	out := make([]string, 0, height)
	if top < 0 {
		for i := 0; i < -top; i++ {
			out = append(out, "")
		}
		top = 0
	}
	out = append(out, all[top:bottom]...)

	for len(out) < height {
		out = append(out, "")
	}
	if len(out) > height {
		out = out[len(out)-height:]
	}
	return out
}

// maxLineOffset is the furthest a window can scroll up for a given buffer/width.
func (m Model) maxLineOffset(b *buffer, width int) int {
	return clampMin(len(m.chatLines(b, width))-m.chatBodyHeight(), 0)
}

// padBody pads a short content block to chatBodyHeight lines, bottom-anchored.
func (m Model) padBody(content []string) []string {
	height := m.chatBodyHeight()
	if len(content) >= height {
		return content[len(content)-height:]
	}
	out := make([]string, 0, height)
	for i := 0; i < height-len(content); i++ {
		out = append(out, "")
	}
	return append(out, content...)
}
