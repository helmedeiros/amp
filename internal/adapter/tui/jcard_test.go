package tui

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

func minutes(ms ...int) []music.Track {
	out := make([]music.Track, len(ms))
	for i, m := range ms {
		out[i] = music.Track{Name: "t", Duration: time.Duration(m) * time.Minute}
	}
	return out
}

func TestSideSplitStopsBeforeTheTrackThatDoesNotFit(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		tracks []music.Track
		want   int
	}{
		"empty":                  {tracks: nil, want: 0},
		"all fit on side A":      {tracks: minutes(10, 10), want: 2},
		"exactly half":           {tracks: minutes(20, 25, 5), want: 2},
		"next one runs over":     {tracks: minutes(20, 20, 10), want: 2},
		"first is longer than A": {tracks: minutes(50, 5), want: 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, sideSplit(c.tracks, 90))
		})
	}
}

func jcardList(n, cursor, marker, height int) list {
	l := newList()
	items := make([]string, n)
	l.SetItems(items)
	l.SetHeight(height)
	l.MoveTo(cursor)
	l.SetMarker(marker)
	return l
}

func shortTape(r *lipgloss.Renderer) theme {
	th := tapeTheme(r)
	th.tape.minutes = 10
	return th
}

func TestJCardFillsTheListHeightAtTapeWidth(t *testing.T) {
	t.Parallel()

	th := shortTape(ansiRenderer())
	card := jcard(th, jcardList(4, 0, 1, 12), goldenQueue())

	rows := strings.Split(card, "\n")
	require.Len(t, rows, 12)
	for _, row := range rows {
		assert.Equal(t, tapeWidth, lipgloss.Width(row), row)
	}
}

func TestJCardWritesEachTrackOnItsSide(t *testing.T) {
	t.Parallel()

	card := jcard(shortTape(lipgloss.NewRenderer(io.Discard)), jcardList(4, 3, 1, 8), goldenQueue())
	rows := strings.Split(card, "\n")

	assert.Contains(t, rows[1], "A  05:01")
	assert.Contains(t, rows[1], "B  11:06")
	assert.Contains(t, rows[3], "   1 SMELLS LIKE TEEN SPIRIT")
	assert.Contains(t, rows[3], "♪  1 HEART-SHAPED BOX")
	assert.Contains(t, rows[4], "┆   2 ABOUT A GIRL")
	assert.Contains(t, rows[5], "▸  3 COME AS YOU ARE")
}

func TestJCardScrollsTheCursorIntoView(t *testing.T) {
	t.Parallel()

	tracks := make([]music.Track, 30)
	for i := range tracks {
		tracks[i] = music.Track{Name: "Song", Duration: time.Minute}
	}
	tracks[29].Name = "Last"
	card := jcard(tapeTheme(lipgloss.NewRenderer(io.Discard)), jcardList(30, 29, -1, 10), tracks)

	assert.Contains(t, card, "▸ 30 LAST")
}

func TestTapeSpineWhilePaused(t *testing.T) {
	t.Parallel()

	th := shortTape(lipgloss.NewRenderer(io.Discard))
	sc := tapeScreen(tapeStatus(music.Paused, 74*time.Second))
	sc.tracks, sc.playing = goldenQueue(), 1

	spine := strings.Split(th.layout.header(th, sc), "\n")

	require.Len(t, spine, 3)
	assert.Contains(t, spine[1], "B  NIRVANA — HEART-SHAPED BOX")
	assert.Contains(t, spine[1], "▮▮ PAUSED 01:14")
}

func TestTapeDeckNamesThePlayingSide(t *testing.T) {
	t.Parallel()

	th := shortTape(lipgloss.NewRenderer(io.Discard))
	sc := tapeScreen(tapeStatus(music.Playing, 0))
	sc.tracks, sc.playing = goldenQueue(), 1

	assert.Contains(t, th.layout.header(th, sc), "│ │ B  Nirvana — Heart-Shaped Box")
}

func pausedTapeApp(t *testing.T) app {
	t.Helper()
	m := goldenApp(tapeStatus(music.Paused, 74*time.Second))
	m.theme = shortTape(lipgloss.DefaultRenderer())
	return apply(m, tea.WindowSizeMsg{Width: 100, Height: 24})
}

func TestTapePausedQueueIsAJCard(t *testing.T) {
	t.Parallel()

	m := pausedTapeApp(t)
	assertGolden(t, "tape_paused", m.View())
	assert.Equal(t, 24, strings.Count(m.View(), "\n")+1, "the card fills the terminal")
}

func TestTapePausedKeepsTheListWhenFilteredOrOnAnotherTab(t *testing.T) {
	t.Parallel()

	m := apply(pausedTapeApp(t), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	assert.NotContains(t, m.View(), "┬")
	assert.Contains(t, m.View(), "Nirvana — About a Girl")

	m = pausedTapeApp(t)
	m.active = tabPlaylists
	assert.NotContains(t, m.View(), "┬")
}

func TestTapePausedKeepsTheListWhenTooShortForACard(t *testing.T) {
	t.Parallel()

	th := tapeTheme(lipgloss.NewRenderer(io.Discard))
	sc := tapeScreen(tapeStatus(music.Paused, 0))
	sc.tab, sc.tracks = tabQueue, goldenQueue()
	l := jcardList(4, 0, 1, jcardChrome)

	assert.Equal(t, l.View(th), th.layout.body(th, sc, l))
}
