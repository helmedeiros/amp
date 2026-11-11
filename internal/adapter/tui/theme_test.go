package tui

import (
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

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
