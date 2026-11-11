package tui

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/helmedeiros/amp/internal/music"
)

func ansiRenderer() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI)
	return r
}

func TestFlatThemeKeepsTheOriginalColors(t *testing.T) {
	t.Parallel()

	st := music.Status{State: music.Playing, Volume: music.NewVolume(12), Elapsed: 14 * time.Second, Track: goldenQueue()[1]}
	m := goldenApp(st)
	m.theme = flatTheme(ansiRenderer())
	assertGolden(t, "flat_playing_ansi", m.View())

	m.picking, m.pickArtist = true, "Nirvana"
	m.pickAlbums = []music.CatalogAlbum{{Name: "Bleach", TrackCount: 13}}
	m.pickChecked = []bool{true}
	assertGolden(t, "flat_picker_ansi", m.View())
}

type fakeLayout struct{ seen *screen }

func (f fakeLayout) header(_ theme, s screen) string {
	*f.seen = s
	return "HEADER"
}

func (fakeLayout) body(_ theme, _ screen, l list) string { return fmt.Sprintf("BODY %d rows", l.Len()) }

func TestAppDrawsThroughTheThemeLayout(t *testing.T) {
	t.Parallel()

	st := music.Status{State: music.Paused, Track: goldenQueue()[1]}
	m := goldenApp(st)
	var seen screen
	m.theme.layout = fakeLayout{seen: &seen}

	view := m.View()

	assert.True(t, strings.HasPrefix(view, "HEADER\n\n1 Queue"), view)
	assert.Contains(t, view, "BODY 4 rows")
	assert.Equal(t, screen{status: st, hasStatus: true, catalogOn: true, width: 100}, seen)
}

func TestThemeGlyphsDriveTheRender(t *testing.T) {
	t.Parallel()

	st := music.Status{State: music.Playing, Elapsed: 140 * time.Second, Track: goldenQueue()[1]}
	m := goldenApp(st)
	m.theme.glyphs = glyphs{cursor: ">", marker: "*", fill: "=", empty: "-", loadOn: "#", loadOff: "."}

	view := m.View()
	assert.Contains(t, view, "* Nirvana — Heart-Shaped Box")
	assert.Contains(t, view, strings.Repeat("=", 20)+strings.Repeat("-", 20)+" 04:39")

	m = apply(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Contains(t, m.View(), "> Nirvana — About a Girl")

	m.working = true
	assert.Contains(t, m.View(), "######..................")
}
