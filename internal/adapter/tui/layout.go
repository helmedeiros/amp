package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/helmedeiros/amp/internal/music"
)

const headerBarWidth = 40

type screen struct {
	status    music.Status
	hasStatus bool
	catalogOn bool
	width     int
}

type layout interface {
	header(t theme, s screen) string
	body(t theme, s screen, l list) string
}

type flatLayout struct{}

func (flatLayout) header(t theme, sc screen) string {
	s := sc.status
	if !sc.hasStatus {
		return t.dim.Render("connecting…")
	}

	var b strings.Builder
	b.WriteString(t.state(s.State).Render(strings.ToUpper(s.State.String())))
	if s.HasTrack() {
		fmt.Fprintf(&b, "\n%s", t.track.Render(artistTitle(s.Track)))
		if s.Track.Album != "" {
			fmt.Fprintf(&b, "\n%s", t.dim.Render(s.Track.Album))
		}
		fmt.Fprintf(&b, "\n%s %s %s  %s",
			clock(s.Elapsed), t.bar(s.Progress(), headerBarWidth), clock(s.Track.Duration),
			t.dim.Render(fmt.Sprintf("%d%%", int(math.Round(s.Progress()*100)))),
		)
	}
	fmt.Fprintf(&b, "\n%s", t.dim.Render(fmt.Sprintf("vol %d%%", s.Volume.Int())))
	if s.Shuffle {
		b.WriteString(t.dim.Render("  shuffle"))
	}
	if s.Repeat != music.RepeatOff {
		b.WriteString(t.dim.Render("  repeat " + s.Repeat.String()))
	}
	tag := "  · Apple Music off"
	if sc.catalogOn {
		tag = "  · Apple Music ✓"
	}
	b.WriteString(t.dim.Render(tag))
	return b.String()
}

func (flatLayout) body(t theme, _ screen, l list) string { return l.View(t) }

func (t theme) loadBar(frame, width int) string {
	if width < 8 {
		width = 8
	}
	const win = 6
	span := 2 * (width - win)
	p := frame % span
	pos := p
	if p > width-win {
		pos = span - p
	}
	var b strings.Builder
	for i := 0; i < width; i++ {
		if i >= pos && i < pos+win {
			b.WriteString(t.glyphs.loadOn)
		} else {
			b.WriteString(t.glyphs.loadOff)
		}
	}
	return b.String()
}

func (t theme) bar(fraction float64, width int) string {
	if width <= 0 {
		return ""
	}
	fraction = math.Max(0, math.Min(1, fraction))
	filled := int(math.Round(fraction * float64(width)))
	return strings.Repeat(t.glyphs.fill, filled) + strings.Repeat(t.glyphs.empty, width-filled)
}
