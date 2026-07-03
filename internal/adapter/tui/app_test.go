package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// stubController implements only the list-fetching methods the app uses.
type stubController struct {
	port.Controller
	queue     []music.Track
	playlists []music.Playlist
	artists   []string
	albums    []string
}

func (s stubController) Queue(context.Context) ([]music.Track, error) { return s.queue, nil }
func (s stubController) Playlists(context.Context) ([]music.Playlist, error) {
	return s.playlists, nil
}
func (s stubController) Artists(context.Context) ([]string, error) { return s.artists, nil }
func (s stubController) Albums(context.Context) ([]string, error)  { return s.albums, nil }

func playingStatus() music.Status {
	return music.Status{
		State:   music.Playing,
		Volume:  music.NewVolume(60),
		Elapsed: 117 * time.Second,
		Track:   music.Track{Name: "Gorgon", Artist: "Utsu-P", Album: "Unknown Album", Duration: 255 * time.Second},
	}
}

func newTestApp(ctrl port.Controller) app {
	return newApp(context.Background(), ctrl, nil)
}

func TestFetchTab(t *testing.T) {
	t.Parallel()

	ctrl := stubController{
		queue:     []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}},
		playlists: []music.Playlist{{Name: "Chill", Count: 42}},
		artists:   []string{"Daft Punk"},
		albums:    []string{"Discovery"},
	}

	q, _ := fetchTab(context.Background(), ctrl, tabQueue)
	assert.Equal(t, []string{"Utsu-P — Gorgon"}, q)

	p, _ := fetchTab(context.Background(), ctrl, tabPlaylists)
	assert.Equal(t, []string{"Chill  (42)"}, p)

	a, _ := fetchTab(context.Background(), ctrl, tabArtists)
	assert.Equal(t, []string{"Daft Punk"}, a)

	al, _ := fetchTab(context.Background(), ctrl, tabAlbums)
	assert.Equal(t, []string{"Discovery"}, al)
}

func TestAppHeaderUpdatesFromStatus(t *testing.T) {
	t.Parallel()

	next, _ := newTestApp(stubController{}).Update(statusMsg(playingStatus()))
	view := next.(app).View()

	assert.Contains(t, view, "PLAYING")
	assert.Contains(t, view, "Utsu-P — Gorgon")
	assert.Contains(t, view, "01:57")
	assert.Contains(t, view, "vol 60%")
}

func TestAppShowsTabItems(t *testing.T) {
	t.Parallel()

	m := newTestApp(stubController{})
	next, _ := m.Update(tabItemsMsg{tab: tabQueue, items: []string{"Utsu-P — Gorgon", "Utsu-P — Vulgar"}})
	view := next.(app).View()

	assert.Contains(t, view, "1 Queue")
	assert.Contains(t, view, "Utsu-P — Gorgon")
	assert.Contains(t, view, "Utsu-P — Vulgar")
}

func TestAppSwitchesTabsAndLoads(t *testing.T) {
	t.Parallel()

	m := newTestApp(stubController{})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = next.(app)

	assert.Equal(t, tabPlaylists, m.active)
	require.NotNil(t, cmd, "switching to an unloaded tab should load it")
	assert.IsType(t, tabItemsMsg{}, cmd(), "load command yields tab items")
}

func TestAppListNavigation(t *testing.T) {
	t.Parallel()

	m := newTestApp(stubController{})
	next, _ := m.Update(tabItemsMsg{tab: tabQueue, items: []string{"a", "b", "c"}})
	m = next.(app)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = next.(app)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = next.(app)

	assert.Equal(t, 2, m.lists[tabQueue].Cursor())
}

func TestAppQuits(t *testing.T) {
	t.Parallel()

	next, cmd := newTestApp(stubController{}).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.True(t, next.(app).quitting)
	assert.NotNil(t, cmd)
}
