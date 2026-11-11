package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

func tapeScreen(st music.Status) screen {
	return screen{status: st, hasStatus: true, catalogOn: true, width: 100}
}

func tapeStatus(state music.PlayerState, elapsed time.Duration) music.Status {
	return music.Status{State: state, Volume: music.NewVolume(60), Elapsed: elapsed, Track: goldenQueue()[1]}
}

func TestTapeDeckKeepsEveryRowTheSameWidth(t *testing.T) {
	t.Parallel()

	th := tapeTheme(ansiRenderer())
	long := tapeStatus(music.Playing, 0)
	long.Track.Name = strings.Repeat("Very Long Title ", 10)
	long.Shuffle, long.Repeat = true, music.RepeatAll
	for _, st := range []music.Status{tapeStatus(music.Playing, 14*time.Second), tapeStatus(music.Stopped, 0), long, {State: music.Stopped}} {
		for _, row := range strings.Split(th.layout.header(th, tapeScreen(st)), "\n") {
			assert.Equal(t, tapeWidth, lipgloss.Width(row), row)
		}
	}
}

func TestTapeReelsTradeTapeAsTheTrackPlays(t *testing.T) {
	t.Parallel()

	left, right := reels(0, 0, true)
	assert.Equal(t, "(((◐)))", left)
	assert.Equal(t, "   ◐   ", right)

	left, right = reels(0.5, 0, true)
	assert.Equal(t, " ((◐)) ", left)
	assert.Equal(t, " ((◐)) ", right)

	left, right = reels(1, 0, true)
	assert.Equal(t, "   ◐   ", left)
	assert.Equal(t, "(((◐)))", right)
}

func TestTapeReelsSpinOnlyWhilePlaying(t *testing.T) {
	t.Parallel()

	spin := func(sec int, playing bool) string {
		l, _ := reels(0, sec, playing)
		return strings.TrimSpace(l)
	}
	assert.Equal(t, []string{"(((◐)))", "(((◓)))", "(((◑)))", "(((◒)))", "(((◐)))"},
		[]string{spin(0, true), spin(1, true), spin(2, true), spin(3, true), spin(4, true)})
	assert.Equal(t, spin(0, false), spin(3, false))
}

func TestTapeLightsTheButtonForTheState(t *testing.T) {
	t.Parallel()

	th := tapeTheme(ansiRenderer())
	lit := func(state music.PlayerState) string {
		return th.layout.header(th, tapeScreen(tapeStatus(state, 0)))
	}
	assert.Contains(t, lit(music.Playing), th.accent.Reverse(true).Render(" ▶ PLAY "))
	assert.Contains(t, lit(music.Paused), th.accent.Reverse(true).Render(" ▮▮ PAUSE "))
	assert.Contains(t, lit(music.Stopped), th.accent.Reverse(true).Render(" ■ STOP "))
}

func TestTapeCounterFollowsElapsedTime(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "[014]", counter(14*time.Second))
	assert.Equal(t, "[999]", counter(999*time.Second))
	assert.Equal(t, "[000]", counter(1000*time.Second))
}

func TestTapeFallsBackToFlatWhenNarrow(t *testing.T) {
	t.Parallel()

	th := tapeTheme(ansiRenderer())
	sc := tapeScreen(tapeStatus(music.Playing, 14*time.Second))
	sc.width = tapeWidth - 1
	assert.Equal(t, flatLayout{}.header(th, sc), th.layout.header(th, sc))
}

func TestTapeFrameFitsTheTerminal(t *testing.T) {
	t.Parallel()

	tracks := make([]music.Track, 40)
	for i := range tracks {
		tracks[i] = music.Track{Name: fmt.Sprintf("Song %d", i), Artist: "Nirvana", Duration: 3 * time.Minute}
	}
	m := newTestApp(&stubController{queue: tracks})
	m.theme = tapeTheme(lipgloss.DefaultRenderer())
	m = apply(m, tea.WindowSizeMsg{Width: 100, Height: 24}, m.loadTab(tabQueue)(), statusMsg(music.Status{State: music.Playing, Track: tracks[30]}))

	view := m.View()
	require.Equal(t, 24, strings.Count(view, "\n")+1)
	assert.True(t, strings.HasPrefix(view, "╭"), "the deck stays on screen")
}

func TestGoldenTapeViews(t *testing.T) {
	t.Parallel()

	cases := map[string]music.Status{
		"tape_playing": tapeStatus(music.Playing, 140*time.Second),
		"tape_stopped": {State: music.Stopped, Volume: music.NewVolume(30)},
	}
	for name, st := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := goldenApp(st)
			m.theme = tapeTheme(lipgloss.DefaultRenderer())
			assertGolden(t, name, apply(m, tea.WindowSizeMsg{Width: 100, Height: 24}).View())
		})
	}
}

func TestParseThemeKeepsTapeLabelsShort(t *testing.T) {
	t.Parallel()

	_, err := parseTheme(ansiRenderer(), []byte("name = \"x\"\nlayout = \"tape\"\n[tape]\nbrand = \""+strings.Repeat("B", 21)+"\""))
	require.EqualError(t, err, "tape.brand: longer than 20 columns")

	_, err = parseTheme(ansiRenderer(), []byte("name = \"x\"\nlayout = \"tape\"\n[tape]\ntype = \""+strings.Repeat("T", 31)+"\""))
	require.EqualError(t, err, "tape.type: longer than 30 columns")
}
