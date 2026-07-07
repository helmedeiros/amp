package tui

import "strings"

// list is a scrollable, cursor-driven string list used for the TUI tabs.
type list struct {
	items  []string
	cursor int
	offset int
	height int
	marker int // playing row (-1 = none), shown even when the cursor is elsewhere
}

func newList() list { return list{height: 10, marker: -1} }

// SetItems replaces the contents, keeping the cursor in range. The marker is
// dropped when it would fall outside the new contents; callers reset it.
func (l *list) SetItems(items []string) {
	l.items = items
	if l.cursor > len(items)-1 {
		l.cursor = len(items) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.marker > len(items)-1 {
		l.marker = -1
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

// Top moves the cursor and viewport back to the first item.
func (l *list) Top() {
	l.cursor = 0
	l.offset = 0
}

// MoveTo places the cursor on row i, scrolling it into view. Out-of-range
// indices are ignored.
func (l *list) MoveTo(i int) {
	if i < 0 || i >= len(l.items) {
		return
	}
	l.cursor = i
	l.clamp()
}

// SetMarker flags the currently-playing row (-1 clears it).
func (l *list) SetMarker(i int) {
	if i < -1 || i >= len(l.items) {
		i = -1
	}
	l.marker = i
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
		switch {
		case i == l.cursor && i == l.marker:
			b.WriteString("♪ " + selectedStyle.Render(l.items[i]) + "\n")
		case i == l.cursor:
			b.WriteString("▶ " + selectedStyle.Render(l.items[i]) + "\n")
		case i == l.marker:
			b.WriteString("♪ " + playingStyle.Render(l.items[i]) + "\n")
		default:
			b.WriteString("  " + l.items[i] + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
