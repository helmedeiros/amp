package applescript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

func TestQueueScripts(t *testing.T) {
	t.Parallel()

	assert.Contains(t, queueTracksScript(), `Music.userPlaylists.byName("amp queue")`)
	assert.Contains(t, queueClearScript(), "Music.delete(pl.tracks)")

	at := playQueueAtScript(3)
	assert.Contains(t, at, "(((3) % n)")
	assert.Contains(t, at, "pl.play();")
}

func TestPlayerQueueOps(t *testing.T) {
	t.Parallel()

	fq := &fakeRunner{out: []byte(`[{"name":"Gorgon","artist":"Utsu-P"}]`)}
	tracks, err := newPlayer(fq).Queue(context.Background())
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "Gorgon", tracks[0].Name)

	fa := &fakeRunner{outs: [][]byte{[]byte(library), []byte(`{"queued":2}`)}}
	n, err := newPlayer(fa).QueueAdd(context.Background(), "daft punk", 50)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	require.Len(t, fa.calls, 2)
	assert.Contains(t, fa.calls[1].script, `const want = ["P2","P1"];`)
	assert.NotContains(t, fa.calls[1].script, "Music.delete(pl.tracks)")
	assert.Contains(t, fa.calls[1].script, "if (false && n > 0) pl.play();")

	fc := &fakeRunner{out: []byte(`{"cleared":true}`)}
	require.NoError(t, newPlayer(fc).QueueClear(context.Background()))
	assert.Equal(t, javaScript, fc.calls[0].lang)
}

func TestPlayPlaylistScript(t *testing.T) {
	t.Parallel()

	script := playPlaylistScript(`My "Chill"`)

	assert.Contains(t, script, `const src = Music.userPlaylists.byName("My \"Chill\"");`)
	assert.Contains(t, script, `Music.userPlaylists.byName("amp queue")`)
	assert.Contains(t, script, "Music.duplicate(src.tracks, {to: pl})") // bulk copy
	assert.Contains(t, script, "pl.play();")
}

func TestPlayAlbumScript(t *testing.T) {
	t.Parallel()

	script := playAlbumScript("Discovery")

	assert.Contains(t, script, `const want = "Discovery";`)
	assert.Contains(t, script, "only: 'albums'")
	assert.Contains(t, script, "a.trackNumber() - b.trackNumber()")
	assert.Contains(t, script, "t.trackCount()")
	assert.Contains(t, script, "queued: tracks.length, total: total")
	assert.Contains(t, script, `Music.userPlaylists.byName("amp queue")`)
	assert.Contains(t, script, "pl.play();")
}

func TestAddFileAndTrackExistsScripts(t *testing.T) {
	t.Parallel()

	add := addFileScript("/Music/x.mp3", "SoundCloud")
	assert.Contains(t, add, `Music.userPlaylists.byName("SoundCloud")`)
	assert.Contains(t, add, `Music.add([Path("/Music/x.mp3")], {to: pl})`)

	ex := trackExistsScript("O ere", "Platoon")
	assert.Contains(t, ex, `whose({name: "O ere"})`)
	assert.Contains(t, ex, `const want = "Platoon"`)
	assert.Contains(t, ex, `t.artist() === want`)
}

func TestPlayerTrackExists(t *testing.T) {
	t.Parallel()

	fp := &fakeRunner{out: []byte(`{"exists":true}`)}
	got, err := newPlayer(fp).TrackExists(context.Background(), "O ere", "Platoon")
	require.NoError(t, err)
	assert.True(t, got)
	assert.Contains(t, fp.calls[0].script, `"O ere"`)
}

func TestPlayerPlayPlaylistAndAlbum(t *testing.T) {
	t.Parallel()

	fp := &fakeRunner{}
	require.NoError(t, newPlayer(fp).PlayPlaylist(context.Background(), "Chill"))
	assert.Equal(t, javaScript, fp.calls[0].lang)
	assert.Contains(t, fp.calls[0].script, `"Chill"`)

	fa := &fakeRunner{out: []byte(`{"queued":2,"total":11}`)}
	cov, err := newPlayer(fa).PlayAlbum(context.Background(), "Discovery")
	require.NoError(t, err)
	assert.Contains(t, fa.calls[0].script, `"Discovery"`)
	assert.Equal(t, music.AlbumCoverage{Queued: 2, Total: 11}, cov)
	assert.True(t, cov.Partial(), "2 of 11 tracks is a partial album")
}
