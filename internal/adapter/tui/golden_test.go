package tui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

func goldenQueue() []music.Track {
	return []music.Track{
		{Name: "Smells Like Teen Spirit", Artist: "Nirvana", Album: "Nevermind", Duration: 301 * time.Second},
		{Name: "Heart-Shaped Box", Artist: "Nirvana", Album: "Nirvana", Duration: 279 * time.Second},
		{Name: "About a Girl", Artist: "Nirvana", Album: "Bleach", Duration: 168 * time.Second},
		{Name: "Come As You Are", Artist: "Nirvana", Album: "Nevermind", Duration: 219 * time.Second},
	}
}

func apply(m app, msgs ...tea.Msg) app {
	for _, msg := range msgs {
		next, _ := m.Update(msg)
		m = next.(app)
	}
	return m
}

func goldenApp(st music.Status) app {
	m := newTestApp(&stubController{queue: goldenQueue(), catalogEnabled: true})
	return apply(m, tea.WindowSizeMsg{Width: 100, Height: 20}, m.loadTab(tabQueue)(), statusMsg(st))
}

func TestGoldenFlatViews(t *testing.T) {
	t.Parallel()

	playing := music.Status{
		State: music.Playing, Volume: music.NewVolume(12), Elapsed: 14 * time.Second,
		Track: goldenQueue()[1], Shuffle: true, Repeat: music.RepeatAll,
	}
	paused := playing
	paused.State = music.Paused

	cases := map[string]func() string{
		"flat_connecting": func() string {
			return apply(newTestApp(&stubController{}), tea.WindowSizeMsg{Width: 100, Height: 20}).View()
		},
		"flat_playing": func() string { return goldenApp(playing).View() },
		"flat_paused":  func() string { return goldenApp(paused).View() },
		"flat_working": func() string {
			m := goldenApp(playing)
			m.working, m.workFrame, m.workLabel = true, 3, "loading"
			return m.View()
		},
		"flat_filter": func() string {
			m := goldenApp(playing)
			m.active = tabArtists
			return apply(m,
				tabItemsMsg{tab: tabArtists, items: []string{"Nirvana", "Mogwai"}, values: []string{"Nirvana", "Mogwai"}},
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")},
				tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")},
			).View()
		},
		"flat_picker": func() string {
			m := goldenApp(playing)
			m.picking, m.pickArtist = true, "Nirvana"
			m.pickAlbums = []music.CatalogAlbum{{Name: "Bleach", TrackCount: 13}, {Name: "In Utero", TrackCount: 12}}
			m.pickChecked = []bool{true, false}
			return m.View()
		},
	}
	for name, render := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertGolden(t, name, render())
		})
	}
}
