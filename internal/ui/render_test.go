package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"vimgram/internal/app"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		s     string
		width int
		want  []string
	}{
		{"a b c", 3, []string{"a b", "c"}},
		{"hello", 3, []string{"hel", "lo"}},   // hard cut of an over-long word
		{"", 10, []string{""}},                // empty -> single empty line
		{"one two three", 7, []string{"one two", "three"}},
		{"abc", 10, []string{"abc"}},          // fits on one line
	}
	for _, tc := range tests {
		got := wrapText(tc.s, tc.width)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("wrapText(%q,%d) = %v, want %v", tc.s, tc.width, got, tc.want)
		}
		for _, line := range got {
			if lipgloss.Width(line) > tc.width {
				t.Errorf("wrapText line %q exceeds width %d", line, tc.width)
			}
		}
	}
}

func TestTruncRunes(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"héllo", 4, "hél…"}, // rune-aware: 3 runes + ellipsis, multibyte intact
		{"hello", 1, "h"},
	}
	for _, tc := range tests {
		if got := truncRunes(tc.s, tc.n); got != tc.want {
			t.Errorf("truncRunes(%q,%d) = %q, want %q", tc.s, tc.n, got, tc.want)
		}
	}
}

func TestSingleLine(t *testing.T) {
	if got := singleLine("a\nb\r\nc  "); got != "a b  c" {
		t.Errorf("singleLine = %q", got)
	}
}

func TestSplitWidths(t *testing.T) {
	tests := []struct {
		total, n int
		want     []int
	}{
		{80, 1, []int{80}},
		{80, 2, []int{40, 40}},
		{81, 2, []int{41, 40}},
		{10, 3, []int{4, 3, 3}},
	}
	for _, tc := range tests {
		got := splitWidths(tc.total, tc.n)
		sum := 0
		for i, w := range got {
			sum += w
			if w != tc.want[i] {
				t.Errorf("splitWidths(%d,%d)[%d] = %d, want %d", tc.total, tc.n, i, w, tc.want[i])
			}
		}
		if sum != tc.total {
			t.Errorf("splitWidths(%d,%d) sums to %d, want %d", tc.total, tc.n, sum, tc.total)
		}
	}
}

func TestPadTo(t *testing.T) {
	if got := padTo("ab", 5); got != "ab   " {
		t.Errorf("padTo short = %q", got)
	}
	if got := padTo("abcde", 3); got != "abcde" {
		t.Errorf("padTo already-wide should not truncate, got %q", got)
	}
}

func TestUnreadBadge(t *testing.T) {
	tests := []struct {
		count int
		want  string // visible content (ignoring ANSI styling)
	}{
		{5, " 5 "},
		{12, " 12"},
		{345, "345"},
		{1000, "999"},
		{9999, "999"},
	}
	for _, tc := range tests {
		got := unreadBadge(tc.count)
		if lipgloss.Width(got) != 3 {
			t.Errorf("unreadBadge(%d) width = %d, want 3", tc.count, lipgloss.Width(got))
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("unreadBadge(%d) = %q, want to contain %q", tc.count, got, tc.want)
		}
	}
}

func TestRenderMessageBlockWraps(t *testing.T) {
	long := strings.Repeat("word ", 40)
	msg := app.Message{Text: long, Date: time.Unix(0, 0)}
	lines := renderMessageBlock(msg, "Alice", "Me", 40, true)
	if len(lines) < 3 {
		t.Fatalf("a long message with a header should produce several lines, got %d", len(lines))
	}
	for _, line := range lines {
		if lipgloss.Width(line) > 40 {
			t.Errorf("rendered line exceeds width: %q (w=%d)", line, lipgloss.Width(line))
		}
	}
}

func TestStartsNewGroup(t *testing.T) {
	base := app.Message{From: "Alice", Date: time.Unix(1000, 0)}
	if !startsNewGroup(nil, base) {
		t.Error("first message should start a group")
	}
	same := app.Message{From: "Alice", Date: base.Date.Add(5 * time.Minute)}
	if startsNewGroup(&base, same) {
		t.Error("same sender within 15m should not start a new group")
	}
	gap := app.Message{From: "Alice", Date: base.Date.Add(20 * time.Minute)}
	if !startsNewGroup(&base, gap) {
		t.Error("same sender after >15m should start a new group")
	}
	other := app.Message{From: "Bob", Date: base.Date.Add(time.Minute)}
	if !startsNewGroup(&base, other) {
		t.Error("a different sender should start a new group")
	}
}
