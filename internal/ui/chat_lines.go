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

// chatLines returns the flattened history (see computeChatLines), memoized.
// Rendering every message through lipgloss on each frame was the dominant
// allocation source; the cache makes repeated frames essentially free.
func (m Model) chatLines() []string {
	// While loading older messages the top line animates a spinner, so the
	// output changes every frame — skip the cache to keep it live.
	if m.loadingMore {
		return m.computeChatLines()
	}
	key := chatCacheKey{version: m.msgVersion, width: m.width, hasMore: m.hasMore}
	if m.chatCache.lines != nil && m.chatCache.key == key {
		return m.chatCache.lines
	}
	lines := m.computeChatLines()
	m.chatCache.key = key
	m.chatCache.lines = lines
	return lines
}

// computeChatLines flattens the whole loaded history into a flat slice of
// visual lines (one wrapped message can produce several). The first line is
// always a "top of history" marker so that scrolling all the way up reveals
// the load state. Scrolling operates on this flat list, giving true per-line
// movement.
func (m Model) computeChatLines() []string {
	chatName, selfName := m.chatAndSelfName()

	lines := make([]string, 0, len(m.messages)+1)
	lines = append(lines, m.historyTopLine())
	for _, msg := range m.messages {
		rendered := renderMessage(msg, chatName, selfName, m.width)
		lines = append(lines, strings.Split(rendered, "\n")...)
	}
	return lines
}

// chatLineCount returns the total number of visual lines in the history.
func (m Model) chatLineCount() int {
	return len(m.chatLines())
}

// historyTopLine is the single marker line shown above the oldest message.
func (m Model) historyTopLine() string {
	switch {
	case m.loadingMore:
		return m.spin.View() + dimStyle.Render(" loading older messages...")
	case !m.hasMore:
		return dimStyle.Render("— start of chat —")
	default:
		return dimStyle.Render("↑ k — load older")
	}
}

// chatViewport returns exactly chatBodyHeight lines for the message area,
// bottom-anchored. When there is less content than the body height, blank
// lines pad the top so messages sit just above the input — and, crucially, the
// status line never moves.
func (m Model) chatViewport() []string {
	all := m.chatLines()
	height := m.chatBodyHeight()
	total := len(all)

	bottom := total - m.lineOffset
	if bottom > total {
		bottom = total
	}
	if bottom < 0 {
		bottom = 0
	}
	top := bottom - height

	win := make([]string, 0, height)
	if top < 0 {
		for i := 0; i < -top; i++ {
			win = append(win, "")
		}
		top = 0
	}
	win = append(win, all[top:bottom]...)

	// Guard against rounding: always emit exactly `height` lines.
	for len(win) < height {
		win = append(win, "")
	}
	if len(win) > height {
		win = win[len(win)-height:]
	}
	return win
}

// maxLineOffset is the furthest we can scroll up (top of history visible).
func (m Model) maxLineOffset() int {
	return clampMin(m.chatLineCount()-m.chatBodyHeight(), 0)
}

// padBody pads an arbitrary short content block to chatBodyHeight lines,
// bottom-anchored. Used for the loading / empty states.
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
