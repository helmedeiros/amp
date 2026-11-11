package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/helmedeiros/amp/internal/music"
)

const (
	tapeWidth   = 64
	deckInner   = tapeWidth - 4
	labelInner  = deckInner - 4
	reelWidth   = 7
	tapeWindow  = 24
	volumeLEDs  = 5
	counterSpan = 1000
)

var reelFrames = []string{"◐", "◓", "◑", "◒"}

type tapeInfo struct {
	brand   string
	kind    string
	minutes int
}

type tapeLayout struct{}

func tapeTheme(r *lipgloss.Renderer) theme {
	color := func(c string) lipgloss.Style { return r.NewStyle().Foreground(lipgloss.Color(c)) }
	return theme{
		name:   "tape",
		layout: tapeLayout{},
		glyphs: glyphs{cursor: "▸", marker: "♪", fill: "▒", empty: "·", loadOn: "▓", loadOff: "░"},
		tape:   tapeInfo{brand: "AMP", kind: "TYPE I (NORMAL)", minutes: 90},
		states: map[music.PlayerState]lipgloss.Style{
			music.Playing: color("#E8B04B").Bold(true),
			music.Paused:  color("#D9822B").Bold(true),
			music.Stopped: color("#8A8A8A").Bold(true),
		},
		track:    color("#F2EEE3").Bold(true),
		dim:      r.NewStyle().Faint(true),
		accent:   color("#C8553D").Bold(true),
		selected: color("#E8B04B").Bold(true),
		current:  color("#E8B04B"),
		notice:   color("#D9822B"),
	}
}

func (tapeLayout) header(t theme, sc screen) string {
	if !sc.hasStatus || sc.width < tapeWidth {
		return flatLayout{}.header(t, sc)
	}
	s := sc.status
	if s.State == music.Paused {
		return spine(t, sc)
	}
	rows := []string{
		"╭" + strings.Repeat("─", tapeWidth-2) + "╮",
		deckRow(spread(t.track.Render(t.tape.brand), deckStatus(t, sc), deckInner)),
		deckRow("┌" + strings.Repeat("─", deckInner-2) + "┐"),
	}
	for _, r := range cassette(t, s, sideOf(sc.tracks, t.tape.minutes, sc.playing)) {
		rows = append(rows, deckRow("│ "+fit(r, labelInner)+" │"))
	}
	rows = append(rows,
		deckRow("└"+strings.Repeat("─", deckInner-2)+"┘"),
		deckRow(center(transport(t, s.State), deckInner)),
		"╰"+strings.Repeat("─", tapeWidth-2)+"╯",
	)
	return strings.Join(rows, "\n")
}

func (tapeLayout) body(t theme, sc screen, l list) string {
	if sc.status.State != music.Paused || sc.tab != tabQueue || sc.width < tapeWidth || len(sc.tracks) == 0 || len(sc.tracks) != l.Len() || l.height <= jcardChrome {
		return l.View(t)
	}
	return jcard(t, l, sc.tracks)
}

func spine(t theme, sc screen) string {
	s := sc.status
	title := "— no tape —"
	if s.HasTrack() {
		title = strings.ToUpper(artistTitle(s.Track))
	}
	label := t.accent.Render(sideOf(sc.tracks, t.tape.minutes, sc.playing)) + "  " + t.track.Render(truncate(title, deckInner-24))
	state := t.state(s.State).Render("▮▮ PAUSED") + " " + clock(s.Elapsed)
	brand := "─ " + t.tape.brand + " "
	length := fmt.Sprintf(" C%d ─", t.tape.minutes)
	return strings.Join([]string{
		"╭" + brand + strings.Repeat("─", tapeWidth-2-lipgloss.Width(brand)-lipgloss.Width(length)) + length + "╮",
		deckRow(spread(label, state, deckInner)),
		"╰" + strings.Repeat("─", tapeWidth-2) + "╯",
	}, "\n")
}

func cassette(t theme, s music.Status, side string) []string {
	title, album := t.dim.Render("— no tape —"), ""
	if s.HasTrack() {
		title = t.track.Render(truncate(artistTitle(s.Track), labelInner-3))
		album = t.dim.Render(truncate(s.Track.Album, labelInner-3))
	}
	left, right := reels(s.Progress(), int(s.Elapsed.Seconds()), s.State == music.Playing)
	window := left + "   [" + t.bar(s.Progress(), tapeWindow) + "]   " + right
	clocks := "--:-- / --:--"
	if s.HasTrack() {
		clocks = clock(s.Elapsed) + " / " + clock(s.Track.Duration)
	}
	return []string{
		t.accent.Render(side) + "  " + title,
		"   " + album,
		t.accent.Render(strings.Repeat("━", labelInner)),
		center(window, labelInner),
		spread(t.dim.Render(fmt.Sprintf("%s · C%d", t.tape.kind, t.tape.minutes)), clocks+"  "+counter(s.Elapsed), labelInner),
	}
}

func deckStatus(t theme, sc screen) string {
	var parts []string
	if sc.status.Shuffle {
		parts = append(parts, "shuffle")
	}
	if sc.status.Repeat != music.RepeatOff {
		parts = append(parts, "repeat "+sc.status.Repeat.String())
	}
	if sc.catalogOn {
		parts = append(parts, "Apple Music ✓")
	} else {
		parts = append(parts, "Apple Music off")
	}
	lit := int(math.Round(float64(sc.status.Volume.Int()) / 100 * volumeLEDs))
	leds := t.current.Render(strings.Repeat("▮", lit)) + t.dim.Render(strings.Repeat("▯", volumeLEDs-lit))
	return t.dim.Render(strings.Join(parts, " · ")) + "  " + leds
}

func transport(t theme, state music.PlayerState) string {
	lit := map[music.PlayerState]string{music.Stopped: "■ STOP", music.Playing: "▶ PLAY", music.Paused: "▮▮ PAUSE"}[state]
	buttons := []string{"|◀ PREV", "■ STOP", "▶ PLAY", "▮▮ PAUSE", "▶| NEXT"}
	out := make([]string, len(buttons))
	for i, b := range buttons {
		if b == lit {
			out[i] = t.accent.Reverse(true).Render(" " + b + " ")
			continue
		}
		out[i] = t.dim.Render(" " + b + " ")
	}
	return strings.Join(out, " ")
}

func reels(progress float64, second int, spinning bool) (left, right string) {
	progress = math.Max(0, math.Min(1, progress))
	hub := reelFrames[0]
	if spinning {
		hub = reelFrames[second%len(reelFrames)]
	}
	reel := func(rings int) string {
		return center(strings.Repeat("(", rings)+hub+strings.Repeat(")", rings), reelWidth)
	}
	maxRings := (reelWidth - 1) / 2
	return reel(int(math.Round(float64(maxRings) * (1 - progress)))), reel(int(math.Round(float64(maxRings) * progress)))
}

func counter(elapsed time.Duration) string {
	return fmt.Sprintf("[%03d]", int(elapsed.Seconds())%counterSpan)
}

func deckRow(content string) string { return "│ " + fit(content, deckInner) + " │" }

func truncate(s string, w int) string { return ansi.Truncate(s, w, "…") }

func fit(s string, w int) string {
	s = truncate(s, w)
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

func spread(left, right string, w int) string {
	left = truncate(left, max(0, w-lipgloss.Width(right)-1))
	return left + strings.Repeat(" ", max(1, w-lipgloss.Width(left)-lipgloss.Width(right))) + right
}

func center(s string, w int) string {
	gap := max(0, w-lipgloss.Width(s))
	return strings.Repeat(" ", gap/2) + s + strings.Repeat(" ", gap-gap/2)
}
