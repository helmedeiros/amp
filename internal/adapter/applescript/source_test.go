package applescript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

func TestSourceScripts(t *testing.T) {
	t.Parallel()

	assert.Contains(t, currentPlaylistTracksScript, "Music.currentPlaylist.tracks")
	assert.NotContains(t, currentPlaylistTracksScript, "play(", "reading must not touch playback")

	album := albumTracksScript(`Say "Hi"`)
	assert.Contains(t, album, `whose({album: "Say \"Hi\""})`)
	assert.Contains(t, album, "(x.disc - y.disc) || (x.num - y.num)")
	assert.NotContains(t, album, "play(")
}

func TestPlayerSourceTracks(t *testing.T) {
	t.Parallel()

	fp := &fakeRunner{out: []byte(`[{"name":"Derezzed","artist":"Daft Punk","album":"TRON","duration":104}]`)}
	tracks, err := newPlayer(fp).CurrentPlaylistTracks(context.Background())
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "Derezzed", tracks[0].Name)
	assert.Equal(t, javaScript, fp.calls[0].lang)

	fa := &fakeRunner{out: []byte(`[{"name":"The Grid","album":"TRON","disc":1,"num":2}]`)}
	tracks, err = newPlayer(fa).AlbumTracks(context.Background(), "TRON")
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "The Grid", tracks[0].Name)
	assert.Contains(t, fa.calls[0].script, `whose({album: "TRON"})`)
}

func TestTakeOverScript(t *testing.T) {
	t.Parallel()

	pl := takeOverScript(music.Source{Kind: music.SourcePlaylist, Name: `My "Chill"`}, "", 3, 42.5)
	assert.Contains(t, pl, `const kind = "playlist", name = "My \"Chill\"", album = "", start = 3, pos = 42.500000, qn = "amp queue";`)
	assert.Contains(t, pl, "Music.currentPlaylist")
	assert.Contains(t, pl, "((start % n) + n) % n", "the format verbs are escaped")
	assert.Contains(t, pl, "pl.play();")
	assert.Contains(t, pl, "Music.playerPosition = pos")

	al := takeOverScript(music.Source{Kind: music.SourceExternal}, "TRON", 0, 0)
	assert.Contains(t, al, `const kind = "external", name = "", album = "TRON"`)
	assert.Contains(t, al, "t.discNumber() * 1000 + t.trackNumber()", "same order as AlbumTracks")
}

func TestPlayerTakeOver(t *testing.T) {
	t.Parallel()

	f := &fakeRunner{out: []byte(`{"queued":12}`)}
	n, err := newPlayer(f).TakeOver(context.Background(), music.Source{Kind: music.SourcePlaylist, Name: "Chill"}, "", 2, 0)

	require.NoError(t, err)
	assert.Equal(t, 12, n)
	assert.Equal(t, javaScript, f.calls[0].lang)

	_, err = newPlayer(&fakeRunner{out: []byte(`nope`)}).TakeOver(context.Background(), music.Source{}, "", 0, 0)
	require.Error(t, err)
}
