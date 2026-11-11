package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/helmedeiros/amp/internal/music"
)

const (
	sideAWidth  = (tapeWidth - 3) / 2
	sideBWidth  = tapeWidth - 3 - sideAWidth
	jcardChrome = 4
)

func sideSplit(tracks []music.Track, minutes int) int {
	half := time.Duration(minutes) * time.Minute / 2
	var used time.Duration
	for i, t := range tracks {
		if i > 0 && used+t.Duration > half {
			return i
		}
		used += t.Duration
	}
	return len(tracks)
}

func sideOf(tracks []music.Track, minutes, i int) string {
	if i >= 0 && i >= sideSplit(tracks, minutes) {
		return "B"
	}
	return "A"
}

func jcard(t theme, l list, tracks []music.Track) string {
	split := sideSplit(tracks, t.tape.minutes)
	rows := max(1, l.height-jcardChrome)
	a := side(t, l, tracks, 0, split, rows, sideAWidth)
	b := side(t, l, tracks, split, len(tracks), rows, sideBWidth)

	out := []string{
		"┌" + strings.Repeat("─", sideAWidth) + "┬" + strings.Repeat("─", sideBWidth) + "┐",
		"│" + fit(sideHeader(t, "A", tracks[:split]), sideAWidth) + "│" + fit(sideHeader(t, "B", tracks[split:]), sideBWidth) + "│",
		"├" + strings.Repeat("┈", sideAWidth) + "┼" + strings.Repeat("┈", sideBWidth) + "┤",
	}
	for r := range rows {
		out = append(out, "│"+a[r]+"┆"+b[r]+"│")
	}
	out = append(out, "└"+strings.Repeat("─", sideAWidth)+"┴"+strings.Repeat("─", sideBWidth)+"┘")
	return strings.Join(out, "\n")
}

func sideHeader(t theme, name string, tracks []music.Track) string {
	var total time.Duration
	for _, tr := range tracks {
		total += tr.Duration
	}
	return " " + t.accent.Render(name) + "  " + t.dim.Render(clock(total))
}

func side(t theme, l list, tracks []music.Track, start, end, rows, width int) []string {
	offset := 0
	for _, focus := range []int{l.cursor, l.marker} {
		if focus >= start && focus < end {
			offset = max(0, focus-start-rows+1)
			break
		}
	}
	rule := t.dim.UnsetFaint().UnsetForeground().Underline(true)
	out := make([]string, rows)
	for r := range rows {
		i := start + offset + r
		if i >= end {
			out[r] = rule.Render(strings.Repeat(" ", width))
			continue
		}
		prefix, style := " ", rule
		if i == l.marker {
			prefix, style = t.glyphs.marker, t.current.Underline(true)
		}
		if i == l.cursor {
			style = t.selected.Underline(true)
			if i != l.marker {
				prefix = t.glyphs.cursor
			}
		}
		cell := fmt.Sprintf("%s %2d %s", prefix, i-start+1, strings.ToUpper(tracks[i].Name))
		out[r] = style.Render(fit(cell, width))
	}
	return out
}
