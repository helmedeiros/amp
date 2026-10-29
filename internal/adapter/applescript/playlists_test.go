package applescript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlaylists(t *testing.T) {
	t.Parallel()

	raw := []byte(`[{"name":"Favourites","count":1038,"artists":["Daft Punk","Justice"]},{"name":"Chill","count":42}]`)

	got, err := parsePlaylists(raw)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Favourites", got[0].Name)
	assert.Equal(t, 1038, got[0].Count)
	assert.Equal(t, []string{"Daft Punk", "Justice"}, got[0].Artists)
	assert.Equal(t, "Chill", got[1].Name)
}

func TestPlaylistsScriptFetchesArtists(t *testing.T) {
	t.Parallel()

	assert.Contains(t, playlistsScript, "p.tracks.artist()")
	assert.Contains(t, playlistsScript, "artists:")
}

func TestPlayerLibrarySignatureRunsCheapScript(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{out: []byte("Chill\nFocus|13655\n")}
	p := newPlayer(fake)

	got, err := p.LibrarySignature(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Chill\nFocus|13655", got)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, javaScript, fake.calls[0].lang)
	assert.Equal(t, librarySignatureScript, fake.calls[0].script)
	assert.NotContains(t, librarySignatureScript, "tracks.artist", "must not read per-playlist tracks")
}

func TestPlayerPlaylistsRunsJavaScript(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{out: []byte(`[{"name":"Chill","count":42}]`)}
	p := newPlayer(fake)

	got, err := p.Playlists(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Chill", got[0].Name)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, javaScript, fake.calls[0].lang)
	assert.Equal(t, playlistsScript, fake.calls[0].script)
}
