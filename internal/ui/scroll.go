package ui

const (
	// Layout estimates used by visible-area calculations.
	chatListChrome = 6 // header + footer + counter + paddings
	chatViewChrome = 8 // header + input + status + footer + scroll-hints + padding
)

// visibleRows returns the number of one-line rows that fit in the chat list.
func (m *Model) visibleRows() int {
	h := m.heightOrDefault()
	v := h - chatListChrome
	if v < 3 {
		return 3
	}
	return v
}

// visibleMessages returns the number of message lines that fit on the chat
// screen. With word-wrap, one message may span multiple lines.
func (m *Model) visibleMessages() int {
	h := m.heightOrDefault()
	lines := h - chatViewChrome
	if lines < 3 {
		return 3
	}
	return lines
}

func (m *Model) heightOrDefault() int {
	if m.height > 0 {
		return m.height
	}
	return 24
}

func (m *Model) adjustListOffset() {
	rows := m.visibleRows()
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}
	if m.cursor >= m.listOffset+rows {
		m.listOffset = m.cursor - rows + 1
	}
}
