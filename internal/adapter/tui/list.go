package tui

import "strings"

// list is a scrollable, cursor-driven string list used for the TUI tabs.
type list struct {
	items  []string
	cursor int
	offset int
	height int
}

func newList() list { return list{height: 10} }

// SetItems replaces the contents, keeping the cursor in range.
func (l *list) SetItems(items []string) {
	l.items = items
	if l.cursor > len(items)-1 {
		l.cursor = len(items) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
	l.clamp()
}

// SetHeight sets how many rows are visible.
func (l *list) SetHeight(h int) {
	if h > 0 {
		l.height = h
	}
	l.clamp()
}

func (l *list) MoveUp() {
	if l.cursor > 0 {
		l.cursor--
		l.clamp()
	}
}

func (l *list) MoveDown() {
	if l.cursor < len(l.items)-1 {
		l.cursor++
		l.clamp()
	}
}

// Cursor returns the selected index (0 when empty).
func (l *list) Cursor() int { return l.cursor }

// Len returns the number of items.
func (l *list) Len() int { return len(l.items) }

func (l *list) clamp() {
	switch {
	case l.cursor < l.offset:
		l.offset = l.cursor
	case l.cursor >= l.offset+l.height:
		l.offset = l.cursor - l.height + 1
	}
	if l.offset < 0 {
		l.offset = 0
	}
}

func (l list) View() string {
	if len(l.items) == 0 {
		return dimStyle.Render("  (empty)")
	}

	end := l.offset + l.height
	if end > len(l.items) {
		end = len(l.items)
	}

	var b strings.Builder
	for i := l.offset; i < end; i++ {
		if i == l.cursor {
			b.WriteString("▶ " + selectedStyle.Render(l.items[i]) + "\n")
			continue
		}
		b.WriteString("  " + l.items[i] + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
