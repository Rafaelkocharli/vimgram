package ui

const (
	// Layout estimates used by visible-area calculations.
	chatListChrome = 6 // header + footer + counter + paddings
	chatViewChrome = 3 // header(1) + input(1) + status(1)
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

// chatBodyHeight returns the number of terminal lines reserved for the
// message area in the chat view. The header, input, and status line take a
// fixed line each, so the body fills everything in between. This is what lets
// the status line stay pinned to the bottom regardless of content.
func (m *Model) chatBodyHeight() int {
	h := m.heightOrDefault()
	lines := h - chatViewChrome
	if lines < 1 {
		return 1
	}
	return lines
}

func (m *Model) heightOrDefault() int {
	if m.height > 0 {
		return m.height
	}
	return 24
}

func (m *Model) widthOrDefault() int {
	if m.width > 0 {
		return m.width
	}
	return 80
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
