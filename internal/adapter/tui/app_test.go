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
	albums    []music.Album

	searchResult []music.Track

	calls      []string
	playedName string
	playedIdx  int
}

func (s *stubController) Queue(context.Context) ([]music.Track, error) { return s.queue, nil }
func (s *stubController) Playlists(context.Context) ([]music.Playlist, error) {
	return s.playlists, nil
}
func (s *stubController) Artists(context.Context) ([]string, error)     { return s.artists, nil }
func (s *stubController) Albums(context.Context) ([]music.Album, error) { return s.albums, nil }

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
		playlists: []music.Playlist{{Name: "Chill", Count: 42, Artists: []string{"Daft Punk"}}},
		artists:   []string{"Daft Punk"},
		albums:    []music.Album{{Name: "Discovery", Artist: "Daft Punk"}},
	}

	q, _, qk, _ := fetchTab(context.Background(), ctrl, tabQueue)
	assert.Equal(t, []string{"Utsu-P — Gorgon"}, q)
	assert.Equal(t, []string{"utsu-p  gorgon"}, qk, "queue key carries artist+album+title")

	p, pv, pk, _ := fetchTab(context.Background(), ctrl, tabPlaylists)
	assert.Equal(t, []string{"Chill  (42)"}, p)
	assert.Equal(t, []string{"Chill"}, pv, "playlist action value is the name")
	assert.Equal(t, []string{"chill daft punk"}, pk, "playlist key carries its artists")

	a, av, _, _ := fetchTab(context.Background(), ctrl, tabArtists)
	assert.Equal(t, []string{"Daft Punk"}, a)
	assert.Equal(t, []string{"Daft Punk"}, av)

	al, alv, alk, _ := fetchTab(context.Background(), ctrl, tabAlbums)
	assert.Equal(t, []string{"Daft Punk — Discovery"}, al, "album line shows artist")
	assert.Equal(t, []string{"Discovery"}, alv, "album play target is the name")
	assert.Equal(t, []string{"discovery daft punk"}, alk)
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
	assert.IsType(t, queuePlayedMsg{}, cmd())
	assert.Equal(t, []string{"PlayQueueAt"}, ctrl.calls)
	assert.Equal(t, 1, ctrl.playedIdx)
}

func TestAppPlayJumpsToQueueTab(t *testing.T) {
	t.Parallel()

	// Play a playlist from the Playlists tab...
	ctrl := &stubController{}
	m := newTestApp(ctrl)
	m.active = tabPlaylists
	next, _ := m.Update(tabItemsMsg{tab: tabPlaylists, items: []string{"Chill  (42)"}, values: []string{"Chill"}})
	m = next.(app)

	// Enter yields a queuePlayedMsg; handling it moves to the Queue tab.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	next, reload := m.Update(cmd())
	m = next.(app)

	assert.Equal(t, tabQueue, m.active, "a play jumps to the Queue tab")
	require.NotNil(t, reload, "and refreshes the queue")
	assert.IsType(t, tabItemsMsg{}, reload())
}

func TestAppSlashFiltersTabAndPlaysMappedItem(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	m := newTestApp(ctrl)
	m.active = tabArtists
	next, _ := m.Update(tabItemsMsg{
		tab:    tabArtists,
		items:  []string{"Daft Punk", "Justice", "Daft Hands"},
		values: []string{"Daft Punk", "Justice", "Daft Hands"},
	})
	m = next.(app)

	// / starts a filter on a non-Search tab; typing narrows the list.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = next.(app)
	require.True(t, m.filtering)
	for _, r := range "hands" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(app)
	}
	assert.Equal(t, 1, m.lists[tabArtists].Len(), "only matching rows remain")
	view := m.View()
	assert.Contains(t, view, "Daft Hands")
	assert.NotContains(t, view, "Justice")

	// Enter keeps the filtered view and leaves edit mode.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(app)
	assert.False(t, m.filtering)

	// Playing the highlighted row plays the ORIGINAL item, not the filtered index.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []string{"PlayQuery"}, ctrl.calls)
	assert.Equal(t, "Daft Hands", ctrl.playedName)
}

func TestAppFilterQueuePlaysMappedIndex(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{}
	m := newTestApp(ctrl) // Queue is the default active tab
	next, _ := m.Update(tabItemsMsg{tab: tabQueue, items: []string{"A song", "B song", "C tune"}})
	m = next.(app)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = next.(app)
	for _, r := range "tune" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(app)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // keep filter
	m = next.(app)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // play highlighted
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, []string{"PlayQueueAt"}, ctrl.calls)
	assert.Equal(t, 2, ctrl.playedIdx, `"C tune" is index 2 in the full queue`)
}

func TestAppFilterPlaylistByArtist(t *testing.T) {
	t.Parallel()

	// Two playlists whose NAMES don't contain "daft", but one contains a Daft
	// Punk track. Filtering by the artist should surface that playlist.
	ctrl := &stubController{
		playlists: []music.Playlist{
			{Name: "Road Trip", Count: 10, Artists: []string{"Daft Punk", "Justice"}},
			{Name: "Focus", Count: 8, Artists: []string{"Nils Frahm"}},
		},
	}
	m := newTestApp(ctrl)
	m.active = tabPlaylists
	next, _ := m.Update(m.loadTab(tabPlaylists)())
	m = next.(app)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = next.(app)
	for _, r := range "daft" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(app)
	}

	assert.Equal(t, 1, m.lists[tabPlaylists].Len(), "only the playlist containing the artist remains")
	view := m.View()
	assert.Contains(t, view, "Road Trip")
	assert.NotContains(t, view, "Focus")

	// Enter keeps it; playing plays that playlist by name.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(app)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()
	assert.Equal(t, "Road Trip", ctrl.playedName)
}

func TestAppFilterAlbumByArtist(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{
		albums: []music.Album{
			{Name: "Discovery", Artist: "Daft Punk"},
			{Name: "Immunity", Artist: "Jon Hopkins"},
		},
	}
	m := newTestApp(ctrl)
	m.active = tabAlbums
	next, _ := m.Update(m.loadTab(tabAlbums)())
	m = next.(app)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = next.(app)
	for _, r := range "hopkins" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(app)
	}

	assert.Equal(t, 1, m.lists[tabAlbums].Len())
	assert.Contains(t, m.View(), "Jon Hopkins — Immunity")
}

func TestAppEscClearsFilter(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})
	m.active = tabArtists
	next, _ := m.Update(tabItemsMsg{tab: tabArtists, items: []string{"Daft Punk", "Justice"}, values: []string{"Daft Punk", "Justice"}})
	m = next.(app)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = next.(app)
	for _, r := range "daft" {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(app)
	}
	assert.Equal(t, 1, m.lists[tabArtists].Len())

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // esc while typing clears
	m = next.(app)
	assert.False(t, m.filtering)
	assert.Equal(t, "", m.filterQuery)
	assert.Equal(t, 2, m.lists[tabArtists].Len(), "full list restored")
}

func TestAppSwitchTabClearsFilter(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})
	next, _ := m.Update(tabItemsMsg{tab: tabQueue, items: []string{"A song", "B tune"}})
	m = next.(app)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = next.(app)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = next.(app)
	require.Equal(t, 1, m.lists[tabQueue].Len())
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // keep filter, exit editing
	m = next.(app)

	// Switch away and back: the filter is gone and the full list is shown.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}) // Playlists
	m = next.(app)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")}) // back to Queue
	m = next.(app)
	assert.Equal(t, "", m.filterQuery)
	assert.Equal(t, 2, m.lists[tabQueue].Len(), "full queue restored after switching tabs")
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
