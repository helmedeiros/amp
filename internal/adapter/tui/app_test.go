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

// stubController implements the methods the app uses and records actions.
type stubController struct {
	port.Controller
	queue     []music.Track
	playlists []music.Playlist
	artists   []string
	albums    []string

	searchResult []music.Track

	calls      []string
	playedName string
	playedIdx  int
}

func (s *stubController) Queue(context.Context) ([]music.Track, error) { return s.queue, nil }
func (s *stubController) Playlists(context.Context) ([]music.Playlist, error) {
	return s.playlists, nil
}
func (s *stubController) Artists(context.Context) ([]string, error) { return s.artists, nil }
func (s *stubController) Albums(context.Context) ([]string, error)  { return s.albums, nil }

func (s *stubController) PlayQueueAt(_ context.Context, index int) error {
	s.calls = append(s.calls, "PlayQueueAt")
	s.playedIdx = index
	return nil
}

func (s *stubController) PlayQuery(_ context.Context, query string, _ int) (port.PlayResult, error) {
	s.calls = append(s.calls, "PlayQuery")
	s.playedName = query
	return port.PlayResult{}, nil
}

func (s *stubController) Toggle(context.Context) error {
	s.calls = append(s.calls, "Toggle")
	return nil
}

func (s *stubController) Search(_ context.Context, query string, _ int) ([]music.Track, error) {
	s.calls = append(s.calls, "Search")
	s.playedName = query
	return s.searchResult, nil
}

func (s *stubController) PlaySearch(_ context.Context, query string, _, index int) error {
	s.calls = append(s.calls, "PlaySearch")
	s.playedName, s.playedIdx = query, index
	return nil
}

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

	ctrl := &stubController{
		queue:     []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}},
		playlists: []music.Playlist{{Name: "Chill", Count: 42}},
		artists:   []string{"Daft Punk"},
		albums:    []string{"Discovery"},
	}

	q, _, _ := fetchTab(context.Background(), ctrl, tabQueue)
	assert.Equal(t, []string{"Utsu-P — Gorgon"}, q)

	p, pv, _ := fetchTab(context.Background(), ctrl, tabPlaylists)
	assert.Equal(t, []string{"Chill  (42)"}, p)
	assert.Equal(t, []string{"Chill"}, pv, "playlist action value is the name")

	a, av, _ := fetchTab(context.Background(), ctrl, tabArtists)
	assert.Equal(t, []string{"Daft Punk"}, a)
	assert.Equal(t, []string{"Daft Punk"}, av)

	al, _, _ := fetchTab(context.Background(), ctrl, tabAlbums)
	assert.Equal(t, []string{"Discovery"}, al)
}

func TestAppEnterPlaysQueueByIndex(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	m := newTestApp(ctrl)
	next, _ := m.Update(tabItemsMsg{tab: tabQueue, items: []string{"a", "b", "c"}})
	m = next.(app)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // cursor 1
	m = next.(app)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.IsType(t, actionDoneMsg{}, cmd())
	assert.Equal(t, []string{"PlayQueueAt"}, ctrl.calls)
	assert.Equal(t, 1, ctrl.playedIdx)
}

func TestAppEnterPlaysArtistByName(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	m := newTestApp(ctrl)
	m.active = tabArtists
	next, _ := m.Update(tabItemsMsg{tab: tabArtists, items: []string{"Daft Punk"}, values: []string{"Daft Punk"}})
	m = next.(app)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []string{"PlayQuery"}, ctrl.calls)
	assert.Equal(t, "Daft Punk", ctrl.playedName)
}

func TestAppSearchTabDoesNotTrapNavigation(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")}) // to Search
	m = next.(app)

	assert.Equal(t, tabSearch, m.active)
	assert.False(t, m.searchEditing, "switching to Search must not trap number keys")

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}) // back to Queue
	assert.Equal(t, tabQueue, next.(app).active)
}

func TestAppSlashStartsSearchEdit(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	m = next.(app)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = next.(app)

	assert.True(t, m.searchEditing, "/ enters edit mode")
	assert.Contains(t, m.hint(), "esc cancel")

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // esc leaves edit mode
	assert.False(t, next.(app).searchEditing)
}

func TestAppSearchTypeAndSubmit(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{searchResult: []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}}}
	m := newTestApp(ctrl)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")}) // Search tab
	m = next.(app)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}) // start typing
	m = next.(app)
	require.True(t, m.searchEditing)

	// type "utsu"
	for _, r := range "utsu" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(app)
	}
	assert.Equal(t, "utsu", m.searchQuery)

	// submit
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(app)
	assert.False(t, m.searchEditing, "enter leaves edit mode")
	require.NotNil(t, cmd)

	// the search command populates the results
	res := cmd().(searchResultsMsg)
	assert.Equal(t, []string{"Utsu-P — Gorgon"}, res.items)
	assert.Equal(t, "utsu", ctrl.playedName)

	next, _ = m.Update(res)
	m = next.(app)
	assert.Contains(t, m.View(), "Utsu-P — Gorgon")
}

func TestAppSearchResultEnterPlaysByIndex(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	m := newTestApp(ctrl)
	m.active = tabSearch
	m.searchQuery = "utsu"
	next, _ := m.Update(searchResultsMsg{items: []string{"a", "b", "c"}})
	m = next.(app)
	// not editing anymore; move and play
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = next.(app)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []string{"PlaySearch"}, ctrl.calls)
	assert.Equal(t, "utsu", ctrl.playedName)
	assert.Equal(t, 1, ctrl.playedIdx)
}

func TestAppSpaceToggles(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	_, cmd := newTestApp(ctrl).Update(tea.KeyMsg{Type: tea.KeySpace})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []string{"Toggle"}, ctrl.calls)
}

func TestAppHeaderUpdatesFromStatus(t *testing.T) {
	t.Parallel()

	next, _ := newTestApp(&stubController{}).Update(statusMsg(playingStatus()))
	view := next.(app).View()

	assert.Contains(t, view, "PLAYING")
	assert.Contains(t, view, "Utsu-P — Gorgon")
	assert.Contains(t, view, "01:57")
	assert.Contains(t, view, "vol 60%")
}

func TestAppShowsTabItems(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})
	next, _ := m.Update(tabItemsMsg{tab: tabQueue, items: []string{"Utsu-P — Gorgon", "Utsu-P — Vulgar"}})
	view := next.(app).View()

	assert.Contains(t, view, "1 Queue")
	assert.Contains(t, view, "Utsu-P — Gorgon")
	assert.Contains(t, view, "Utsu-P — Vulgar")
}

func TestAppSwitchesTabsAndLoads(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = next.(app)

	assert.Equal(t, tabPlaylists, m.active)
	require.NotNil(t, cmd, "switching to an unloaded tab should load it")
	assert.IsType(t, tabItemsMsg{}, cmd(), "load command yields tab items")
}

func TestAppListNavigation(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})
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

	next, cmd := newTestApp(&stubController{}).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.True(t, next.(app).quitting)
	assert.NotNil(t, cmd)
}
