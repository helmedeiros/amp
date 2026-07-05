package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/app"
	"github.com/helmedeiros/amp/internal/music"
)

// fakePlayer is a hand-rolled in-memory Player for testing the service in
// isolation from any real engine.
type fakePlayer struct {
	status       music.Status
	statusErr    error
	volumeSet    *music.Volume
	shuffleSet   *bool
	repeatSet    *music.RepeatMode
	calls        []string
	setVolErr    error
	searchQuery  string
	searchLimit  int
	searchResult []music.Track
	playlists    []music.Playlist
	names        []string
	albums       []music.Album
	positionSet  float64
	playStart    int
	playedName   string
}

func (f *fakePlayer) Status(context.Context) (music.Status, error) {
	f.calls = append(f.calls, "Status")
	return f.status, f.statusErr
}

func (f *fakePlayer) Open(context.Context) error { f.calls = append(f.calls, "Open"); return nil }

func (f *fakePlayer) SaveArtwork(_ context.Context, path string) error {
	f.calls = append(f.calls, "SaveArtwork")
	f.playedName = path
	return nil
}

func (f *fakePlayer) Search(_ context.Context, query string, limit int) ([]music.Track, error) {
	f.calls = append(f.calls, "Search")
	f.searchQuery, f.searchLimit = query, limit
	return f.searchResult, nil
}

func (f *fakePlayer) PlaySearch(_ context.Context, query string, limit, start int) error {
	f.calls = append(f.calls, "PlaySearch")
	f.searchQuery, f.searchLimit, f.playStart = query, limit, start
	return nil
}

func (f *fakePlayer) PlayPlaylist(_ context.Context, name string) error {
	f.calls = append(f.calls, "PlayPlaylist")
	f.playedName = name
	return nil
}

func (f *fakePlayer) PlayAlbum(_ context.Context, name string) error {
	f.calls = append(f.calls, "PlayAlbum")
	f.playedName = name
	return nil
}

func (f *fakePlayer) PlayQueueAt(_ context.Context, index int) error {
	f.calls = append(f.calls, "PlayQueueAt")
	f.playStart = index
	return nil
}

func (f *fakePlayer) Queue(context.Context) ([]music.Track, error) {
	f.calls = append(f.calls, "Queue")
	return f.searchResult, nil
}

func (f *fakePlayer) QueueAdd(_ context.Context, query string, limit int) (int, error) {
	f.calls = append(f.calls, "QueueAdd")
	f.searchQuery, f.searchLimit = query, limit
	return len(f.searchResult), nil
}

func (f *fakePlayer) QueueClear(context.Context) error {
	f.calls = append(f.calls, "QueueClear")
	return nil
}

func (f *fakePlayer) Playlists(context.Context) ([]music.Playlist, error) {
	f.calls = append(f.calls, "Playlists")
	return f.playlists, nil
}

func (f *fakePlayer) Artists(context.Context) ([]string, error) {
	f.calls = append(f.calls, "Artists")
	return f.names, nil
}

func (f *fakePlayer) Albums(context.Context) ([]music.Album, error) {
	f.calls = append(f.calls, "Albums")
	return f.albums, nil
}
func (f *fakePlayer) Play(context.Context) error  { f.calls = append(f.calls, "Play"); return nil }
func (f *fakePlayer) Pause(context.Context) error { f.calls = append(f.calls, "Pause"); return nil }
func (f *fakePlayer) Stop(context.Context) error  { f.calls = append(f.calls, "Stop"); return nil }
func (f *fakePlayer) Next(context.Context) error  { f.calls = append(f.calls, "Next"); return nil }
func (f *fakePlayer) Previous(context.Context) error {
	f.calls = append(f.calls, "Previous")
	return nil
}

func (f *fakePlayer) TogglePlayPause(context.Context) error {
	f.calls = append(f.calls, "TogglePlayPause")
	return nil
}

func (f *fakePlayer) SetVolume(_ context.Context, v music.Volume) error {
	f.calls = append(f.calls, "SetVolume")
	if f.setVolErr != nil {
		return f.setVolErr
	}
	f.volumeSet = &v
	return nil
}

func (f *fakePlayer) SetPosition(_ context.Context, seconds float64) error {
	f.calls = append(f.calls, "SetPosition")
	f.positionSet = seconds
	return nil
}

func (f *fakePlayer) SetShuffle(_ context.Context, enabled bool) error {
	f.calls = append(f.calls, "SetShuffle")
	f.shuffleSet = &enabled
	return nil
}

func (f *fakePlayer) SetRepeat(_ context.Context, mode music.RepeatMode) error {
	f.calls = append(f.calls, "SetRepeat")
	f.repeatSet = &mode
	return nil
}

// memStore is an in-memory VolumeStore for tests.
type memStore struct {
	level int
	ok    bool
}

func (m *memStore) Save(level int) error {
	m.level, m.ok = level, true
	return nil
}

func (m *memStore) Load() (int, bool, error) {
	return m.level, m.ok, nil
}

func TestServiceStatusPassesThrough(t *testing.T) {
	t.Parallel()

	want := music.Status{State: music.Playing, Track: music.Track{Name: "Gorgon"}}
	fake := &fakePlayer{status: want}
	svc := app.NewService(fake, &memStore{})

	got, err := svc.Status(context.Background())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestServiceSearchTrimsAndDelegates(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{searchResult: []music.Track{{Name: "Gorgon"}}}
	svc := app.NewService(fake, &memStore{})

	got, err := svc.Search(context.Background(), "  utsu  ", 10)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "utsu", fake.searchQuery, "query is trimmed before search")
	assert.Equal(t, 10, fake.searchLimit)
}

func TestServiceSearchRejectsEmptyQuery(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	_, err := svc.Search(context.Background(), "   ", 10)

	require.Error(t, err)
	assert.NotContains(t, fake.calls, "Search")
}

func TestServicePlayQueryResumesOnEmpty(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	res, err := svc.PlayQuery(context.Background(), "  ", 50)

	require.NoError(t, err)
	assert.Equal(t, "resume", res.Kind)
	assert.Equal(t, []string{"Play"}, fake.calls)
}

func TestServicePlayQueryPrefersPlaylist(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{playlists: []music.Playlist{{Name: "Chill"}}}
	svc := app.NewService(fake, &memStore{})

	res, err := svc.PlayQuery(context.Background(), "chill", 50) // case-insensitive

	require.NoError(t, err)
	assert.Equal(t, "playlist", res.Kind)
	assert.Equal(t, "Chill", res.Label)
	assert.Equal(t, "Chill", fake.playedName)
	assert.Equal(t, []string{"Playlists", "PlayPlaylist"}, fake.calls)
}

func TestServicePlayQueryFallsBackToAlbum(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{albums: []music.Album{{Name: "Discovery"}}} // no playlist match; albums list
	svc := app.NewService(fake, &memStore{})

	res, err := svc.PlayQuery(context.Background(), "discovery", 50)

	require.NoError(t, err)
	assert.Equal(t, "album", res.Kind)
	assert.Equal(t, "Discovery", res.Label)
	assert.Equal(t, []string{"Playlists", "Albums", "PlayAlbum"}, fake.calls)
}

func TestServicePlayQueryFallsBackToTrackSearch(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{searchResult: []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}}}
	svc := app.NewService(fake, &memStore{})

	res, err := svc.PlayQuery(context.Background(), "gorgon", 50)

	require.NoError(t, err)
	assert.Equal(t, "track", res.Kind)
	assert.Equal(t, "Utsu-P — Gorgon", res.Label)
	assert.Equal(t, []string{"Playlists", "Albums", "Search", "PlaySearch"}, fake.calls)
	assert.Equal(t, 0, fake.playStart)
}

func TestServicePlayQueryNoMatchErrors(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{} // no playlists, albums, or tracks
	svc := app.NewService(fake, &memStore{})

	_, err := svc.PlayQuery(context.Background(), "nope", 50)

	require.Error(t, err)
	assert.NotContains(t, fake.calls, "PlaySearch")
}

func TestServicePlaySearchTrimsAndDelegates(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	require.NoError(t, svc.PlaySearch(context.Background(), "  utsu  ", 50, 3))

	assert.Equal(t, "utsu", fake.searchQuery)
	assert.Equal(t, 50, fake.searchLimit)
	assert.Equal(t, 3, fake.playStart)
}

func TestServicePlaySearchRejectsBadInput(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	require.Error(t, svc.PlaySearch(context.Background(), "   ", 50, 0))
	require.Error(t, svc.PlaySearch(context.Background(), "utsu", 50, -1))
	assert.NotContains(t, fake.calls, "PlaySearch")
}

func TestServiceQueueOps(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{searchResult: []music.Track{{Name: "Gorgon"}, {Name: "Vulgar"}}}
	svc := app.NewService(fake, &memStore{})
	ctx := context.Background()

	tracks, err := svc.Queue(ctx)
	require.NoError(t, err)
	assert.Len(t, tracks, 2)

	n, err := svc.QueueAdd(ctx, "  utsu  ", 50)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, "utsu", fake.searchQuery, "query is trimmed")

	require.NoError(t, svc.QueueClear(ctx))

	assert.Equal(t, []string{"Queue", "QueueAdd", "QueueClear"}, fake.calls)
}

func TestServiceQueueAddRejectsEmpty(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	_, err := svc.QueueAdd(context.Background(), "  ", 50)

	require.Error(t, err)
	assert.NotContains(t, fake.calls, "QueueAdd")
}

func TestServicePlaylistsDelegates(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{playlists: []music.Playlist{{Name: "Chill", Count: 42}}}
	svc := app.NewService(fake, &memStore{})

	got, err := svc.Playlists(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Chill", got[0].Name)
	assert.Equal(t, []string{"Playlists"}, fake.calls)
}

func TestServiceLibraryBrowsersDelegate(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{
		names:  []string{"Daft Punk"},
		albums: []music.Album{{Name: "Discovery", Artist: "Daft Punk"}},
	}
	svc := app.NewService(fake, &memStore{})

	artists, err := svc.Artists(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"Daft Punk"}, artists)

	albums, err := svc.Albums(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []music.Album{{Name: "Discovery", Artist: "Daft Punk"}}, albums)

	assert.Equal(t, []string{"Artists", "Albums"}, fake.calls)
}

func TestServiceSeekAbsolute(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	pos, err := svc.Seek(context.Background(), music.SeekAbsolute, 42)

	require.NoError(t, err)
	assert.Equal(t, 42*time.Second, pos)
	assert.InDelta(t, 42, fake.positionSet, 0.001)
	assert.NotContains(t, fake.calls, "Status", "absolute seek needs no read")
}

func TestServiceSeekRelativeReadsCurrent(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{status: music.Status{
		Elapsed: 100 * time.Second,
		Track:   music.Track{Duration: 240 * time.Second},
	}}
	svc := app.NewService(fake, &memStore{})

	pos, err := svc.Seek(context.Background(), music.SeekRelative, -10)

	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, pos)
	assert.Equal(t, []string{"Status", "SetPosition"}, fake.calls)
}

func TestServiceSeekPercent(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{status: music.Status{Track: music.Track{Duration: 200 * time.Second}}}
	svc := app.NewService(fake, &memStore{})

	pos, err := svc.Seek(context.Background(), music.SeekPercent, 25)

	require.NoError(t, err)
	assert.Equal(t, 50*time.Second, pos)
}

func TestServiceSeekClampsToTrackBounds(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{status: music.Status{
		Elapsed: 230 * time.Second,
		Track:   music.Track{Duration: 240 * time.Second},
	}}
	svc := app.NewService(fake, &memStore{})

	pos, err := svc.Seek(context.Background(), music.SeekRelative, 60) // 230+60 > 240

	require.NoError(t, err)
	assert.Equal(t, 240*time.Second, pos, "clamped to duration")
}

func TestServiceOpenDelegates(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	require.NoError(t, svc.Open(context.Background()))
	assert.Equal(t, []string{"Open"}, fake.calls)
}

func TestServiceTransportDelegates(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})
	ctx := context.Background()

	require.NoError(t, svc.Play(ctx))
	require.NoError(t, svc.Pause(ctx))
	require.NoError(t, svc.Toggle(ctx))
	require.NoError(t, svc.Stop(ctx))
	require.NoError(t, svc.Next(ctx))
	require.NoError(t, svc.Previous(ctx))

	assert.Equal(t,
		[]string{"Play", "Pause", "TogglePlayPause", "Stop", "Next", "Previous"},
		fake.calls,
	)
}

func TestServiceSetVolumeClamps(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	got, err := svc.SetVolume(context.Background(), 250)

	require.NoError(t, err)
	assert.Equal(t, 100, got.Int())
	require.NotNil(t, fake.volumeSet)
	assert.Equal(t, 100, fake.volumeSet.Int())
}

func TestServiceAdjustVolumeReadsThenSets(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{status: music.Status{Volume: music.NewVolume(95)}}
	svc := app.NewService(fake, &memStore{})

	got, err := svc.AdjustVolume(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 100, got.Int(), "95 + 10 clamps to 100")
	require.NotNil(t, fake.volumeSet)
	assert.Equal(t, 100, fake.volumeSet.Int())
	assert.Equal(t, []string{"Status", "SetVolume"}, fake.calls)
}

func TestServiceAdjustVolumeSurfacesStatusError(t *testing.T) {
	t.Parallel()

	boom := errors.New("osascript failed")
	fake := &fakePlayer{statusErr: boom}
	svc := app.NewService(fake, &memStore{})

	_, err := svc.AdjustVolume(context.Background(), 10)

	require.ErrorIs(t, err, boom)
	assert.NotContains(t, fake.calls, "SetVolume", "must not set volume if read failed")
}

func TestServiceSetShuffle(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	require.NoError(t, svc.SetShuffle(context.Background(), true))

	require.NotNil(t, fake.shuffleSet)
	assert.True(t, *fake.shuffleSet)
}

func TestServiceToggleShuffleFlipsCurrent(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{status: music.Status{Shuffle: true}}
	svc := app.NewService(fake, &memStore{})

	now, err := svc.ToggleShuffle(context.Background())

	require.NoError(t, err)
	assert.False(t, now, "true should flip to false")
	require.NotNil(t, fake.shuffleSet)
	assert.False(t, *fake.shuffleSet)
	assert.Equal(t, []string{"Status", "SetShuffle"}, fake.calls)
}

func TestServiceToggleShuffleSurfacesReadError(t *testing.T) {
	t.Parallel()

	boom := errors.New("read failed")
	fake := &fakePlayer{statusErr: boom}
	svc := app.NewService(fake, &memStore{})

	_, err := svc.ToggleShuffle(context.Background())

	require.ErrorIs(t, err, boom)
	assert.NotContains(t, fake.calls, "SetShuffle")
}

func TestServiceSetRepeat(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{})

	require.NoError(t, svc.SetRepeat(context.Background(), music.RepeatAll))

	require.NotNil(t, fake.repeatSet)
	assert.Equal(t, music.RepeatAll, *fake.repeatSet)
}

func TestServiceMuteSavesLevelAndZeroes(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{status: music.Status{Volume: music.NewVolume(80)}}
	store := &memStore{}
	svc := app.NewService(fake, store)

	require.NoError(t, svc.Mute(context.Background()))

	assert.Equal(t, 80, store.level, "prior level remembered")
	assert.True(t, store.ok)
	require.NotNil(t, fake.volumeSet)
	assert.Equal(t, 0, fake.volumeSet.Int())
}

func TestServiceMuteWhenAlreadySilentIsNoop(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{status: music.Status{Volume: music.NewVolume(0)}}
	store := &memStore{}
	svc := app.NewService(fake, store)

	require.NoError(t, svc.Mute(context.Background()))

	assert.False(t, store.ok, "must not overwrite remembered level when already muted")
	assert.NotContains(t, fake.calls, "SetVolume")
}

func TestServiceUnmuteRestoresSavedLevel(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	store := &memStore{level: 80, ok: true}
	svc := app.NewService(fake, store)

	got, err := svc.Unmute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 80, got.Int())
	require.NotNil(t, fake.volumeSet)
	assert.Equal(t, 80, fake.volumeSet.Int())
}

func TestServiceUnmuteFallsBackToDefault(t *testing.T) {
	t.Parallel()

	fake := &fakePlayer{}
	svc := app.NewService(fake, &memStore{}) // nothing stored

	got, err := svc.Unmute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, app.DefaultUnmuteVolume, got.Int())
}
