package ui

const (
	// Layout constants used by the fixed-height, bottom-pinned layouts.
	chatListChrome = 2 // header(1) + status(1)
	chatViewChrome = 3 // header(1) + input(1) + status(1)
)

// visibleRows returns the number of dialog rows that fit in the chat list
// body (everything between the header and the status line).
func (m *Model) visibleRows() int {
	h := m.heightOrDefault()
	v := h - chatListChrome
	if v < 1 {
		return 1
	}
	return v
}

// chatBodyHeight returns the number of terminal lines reserved for the
// message area in the chat view. The header, input, status line, and
// (when a reply is pending) the reply banner each take one line.
func (m *Model) chatBodyHeight() int {
	h := m.heightOrDefault()
	extra := 0
	if m.authed {
		if b := m.activeBuffer(); b != nil && b.kind == bufChat &&
			(b.replyToID != 0 || b.editMsgID != 0) {
			extra = 1
		}
	}
	lines := h - chatViewChrome - extra
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

// adjustListOffset keeps the active window's chat-list cursor in view.
func (m *Model) adjustListOffset() {
	rows := m.visibleRows()
	w := m.activeWindow()
	if w.cursor < w.listOffset {
		w.listOffset = w.cursor
	}
	if w.cursor >= w.listOffset+rows {
		w.listOffset = w.cursor - rows + 1
	}
}
