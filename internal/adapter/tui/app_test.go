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
	playResult port.PlayResult

	catalogEnabled bool
	artistAlbums   []music.CatalogAlbum
	addedIDs       []string
}

func (s *stubController) CatalogEnabled() bool { return s.catalogEnabled }
func (s *stubController) ArtistCatalogAlbums(_ context.Context, _ string) ([]music.CatalogAlbum, error) {
	s.calls = append(s.calls, "ArtistCatalogAlbums")
	return s.artistAlbums, nil
}
func (s *stubController) AddCatalogAlbums(_ context.Context, ids []string) (int, error) {
	s.calls = append(s.calls, "AddCatalogAlbums")
	s.addedIDs = ids
	return len(ids), nil
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
	return s.playResult, nil
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

// drainBatch runs a (possibly batched) command and returns the messages its
// sub-commands produce, so tests can inspect an async result inside a tea.Batch.
func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			if c != nil {
				out = append(out, c())
			}
		}
		return out
	}
	return []tea.Msg{msg}
}

func TestAppArtistAddAlbumsPickerFlow(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{
		artists:        []string{"The Kooks"},
		catalogEnabled: true,
		artistAlbums: []music.CatalogAlbum{
			{ID: "listen", Name: "Listen", TrackCount: 11},
			{ID: "junk", Name: "Junk of the Heart", TrackCount: 12},
		},
	}
	m := newTestApp(ctrl)
	m.active = tabArtists
	next, _ := m.Update(m.loadTab(tabArtists)())
	m = next.(app)

	// 'a' starts the fetch (loading bar) and asks the controller for albums.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(app)
	require.NotNil(t, cmd)
	assert.True(t, m.working)
	msgs := drainBatch(cmd)
	var albums artistAlbumsMsg
	for _, msg := range msgs {
		if a, ok := msg.(artistAlbumsMsg); ok {
			albums = a
		}
	}
	require.Equal(t, "The Kooks", albums.artist)
	assert.Contains(t, ctrl.calls, "ArtistCatalogAlbums")

	// Delivering the albums opens the picker with everything preselected.
	next, _ = m.Update(albums)
	m = next.(app)
	require.True(t, m.picking)
	view := m.View()
	assert.Contains(t, view, "Add The Kooks albums from Apple Music")
	assert.Contains(t, view, "Listen")
	assert.Contains(t, view, "[x]")

	// Untick the first album (cursor starts at 0), then confirm.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(app)
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(app)
	assert.False(t, m.picking)
	assert.True(t, m.working)
	require.NotNil(t, cmd)
	drainBatch(cmd) // runs the add
	assert.Equal(t, []string{"junk"}, ctrl.addedIDs, "only the still-checked album is added")

	// The added-result message shows a self-clearing confirmation.
	next, _ = m.Update(albumsAddedMsg{n: 1})
	m = next.(app)
	assert.False(t, m.working)
	assert.Contains(t, m.View(), "added 1 album")
}

func TestAppArtistAddAlbumsNeedsCatalog(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{artists: []string{"The Kooks"}, catalogEnabled: false}
	m := newTestApp(ctrl)
	m.active = tabArtists
	next, _ := m.Update(m.loadTab(tabArtists)())
	m = next.(app)

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = next.(app)
	assert.False(t, m.picking)
	assert.False(t, m.working)
	assert.Contains(t, m.View(), "amp auth apple-music")
}

func TestAlbumFeedback(t *testing.T) {
	t.Parallel()

	// Complete album -> success flash, no warning.
	n, f := albumFeedback(port.PlayResult{Kind: "album", Label: "In Utero", Album: music.AlbumCoverage{Queued: 12, Total: 12}})
	assert.Empty(t, n)
	assert.Contains(t, f, "✓ In Utero — all 12 tracks")

	// Still partial after a catalog fill -> "still syncing", no flash.
	n, f = albumFeedback(port.PlayResult{Kind: "album", Label: "X", Album: music.AlbumCoverage{Queued: 2, Total: 11}, AlbumFilled: true})
	assert.Contains(t, n, "still syncing")
	assert.Empty(t, f)

	// Partial with no catalog configured -> points at amp auth.
	n, f = albumFeedback(port.PlayResult{Kind: "album", Label: "X", Album: music.AlbumCoverage{Queued: 2, Total: 11}})
	assert.Contains(t, n, "amp auth apple-music")
	assert.Empty(t, f)

	// Unknown total (or non-album) -> nothing.
	n, f = albumFeedback(port.PlayResult{Kind: "album", Album: music.AlbumCoverage{Queued: 2, Total: 0}})
	assert.Empty(t, n)
	assert.Empty(t, f)
}

func TestAppAlbumPlayShowsLoadingBarThenFlash(t *testing.T) {
	t.Parallel()

	ctrl := &stubController{albums: []music.Album{{Name: "In Utero", Artist: "Nirvana"}}}
	m := newTestApp(ctrl)
	m.active = tabAlbums
	next, _ := m.Update(m.loadTab(tabAlbums)())
	m = next.(app)

	// Enter starts the loading bar (an animated block track) while the fill runs.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(app)
	require.NotNil(t, cmd)
	assert.True(t, m.working)
	assert.Contains(t, m.View(), "completing album via Apple Music")
	assert.Contains(t, m.View(), "▓")

	// When the play resolves complete, the bar gives way to a success flash.
	next, _ = m.Update(queuePlayedMsg{flash: "✓ In Utero — all 12 tracks in your library"})
	m = next.(app)
	assert.False(t, m.working)
	assert.Contains(t, m.View(), "all 12 tracks")

	// The flash clears itself.
	next, _ = m.Update(flashClearMsg{})
	m = next.(app)
	assert.NotContains(t, m.View(), "all 12 tracks")
}

func TestAppAlbumPartialWarningRendersAndClearsOnTabSwitch(t *testing.T) {
	t.Parallel()

	m := newTestApp(&stubController{})
	next, _ := m.Update(queuePlayedMsg{notice: "only 1 of 13 tracks of \"X\" are in your library"})
	m = next.(app)
	assert.Contains(t, m.View(), "only 1 of 13 tracks")

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m = next.(app)
	assert.NotContains(t, m.View(), "only 1 of 13 tracks")
}

func TestAppQueueMarksAndFollowsPlayingTrack(t *testing.T) {
	t.Parallel()

	first := music.Track{Name: "Jacqueline", Artist: "Franz Ferdinand", Album: "Franz Ferdinand"}
	second := music.Track{Name: "Take Me Out", Artist: "Franz Ferdinand", Album: "Franz Ferdinand"}
	ctrl := &stubController{queue: []music.Track{first, second}}

	m := newTestApp(ctrl) // Queue is the default active tab
	next, _ := m.Update(m.loadTab(tabQueue)())
	m = next.(app)

	// First track plays: it is marked and the cursor sits on it.
	playing := music.Status{State: music.Playing, Track: first}
	next, _ = m.Update(statusMsg(playing))
	m = next.(app)
	assert.Equal(t, 0, m.lists[tabQueue].Cursor())
	assert.Equal(t, 0, m.lists[tabQueue].marker)

	// Auto-advance to the second track moves the marker and the cursor with it,
	// even though the user never touched j/k.
	next, _ = m.Update(statusMsg(music.Status{State: music.Playing, Track: second}))
	m = next.(app)
	assert.Equal(t, 1, m.lists[tabQueue].Cursor(), "cursor follows the playing track")
	assert.Equal(t, 1, m.lists[tabQueue].marker)
	assert.Contains(t, m.View(), "♪ ")
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
