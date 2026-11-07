package applescript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
