package applescript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaySearchScript(t *testing.T) {
	t.Parallel()

	script := playSearchScript(`the "one"`, 25, 3)

	assert.Contains(t, script, `Music.search(lib, {for: "the \"one\"", only: 'all'})`)
	assert.Contains(t, script, "const limit = 25;") // limit
	assert.Contains(t, script, "raw.slice(0, limit)")
	assert.Contains(t, script, "res.push(t)")         // filters unreadable tracks
	assert.Contains(t, script, "(((3) % res.length)") // rotation by start
	assert.Contains(t, script, `Music.userPlaylists.byName("amp queue")`)
	assert.Contains(t, script, "pl.play();")
}

func TestPlayerPlaySearchRunsJavaScript(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{out: []byte(`{"queued":5}`)}
	p := newPlayer(fake)

	err := p.PlaySearch(context.Background(), "daft", 50, 2)

	require.NoError(t, err)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, javaScript, fake.calls[0].lang)
	assert.Contains(t, fake.calls[0].script, `"daft"`)
}

func TestQueueScripts(t *testing.T) {
	t.Parallel()

	assert.Contains(t, queueTracksScript(), `Music.userPlaylists.byName("amp queue")`)
	assert.Contains(t, queueClearScript(), "Music.delete(pl.tracks)")

	at := playQueueAtScript(3)
	assert.Contains(t, at, "(((3) % n)")
	assert.Contains(t, at, "pl.play();")

	add := queueAddScript(`green "day"`, 30)
	assert.Contains(t, add, `{for: "green \"day\"", only: 'all'}`)
	assert.Contains(t, add, "const limit = 30;")
	assert.Contains(t, add, "Music.duplicate(t, {to: pl})")
	assert.NotContains(t, add, "pl.play()") // add must not start playback
}

func TestPlayerQueueOps(t *testing.T) {
	t.Parallel()

	fq := &fakeRunner{out: []byte(`[{"name":"Gorgon","artist":"Utsu-P"}]`)}
	tracks, err := newPlayer(fq).Queue(context.Background())
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	assert.Equal(t, "Gorgon", tracks[0].Name)

	fa := &fakeRunner{out: []byte(`{"added":7}`)}
	n, err := newPlayer(fa).QueueAdd(context.Background(), "daft", 50)
	require.NoError(t, err)
	assert.Equal(t, 7, n)

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
	assert.Contains(t, script, `Music.userPlaylists.byName("amp queue")`)
	assert.Contains(t, script, "pl.play();")
}

func TestPlayerPlayPlaylistAndAlbum(t *testing.T) {
	t.Parallel()

	fp := &fakeRunner{}
	require.NoError(t, newPlayer(fp).PlayPlaylist(context.Background(), "Chill"))
	assert.Equal(t, javaScript, fp.calls[0].lang)
	assert.Contains(t, fp.calls[0].script, `"Chill"`)

	fa := &fakeRunner{}
	require.NoError(t, newPlayer(fa).PlayAlbum(context.Background(), "Discovery"))
	assert.Contains(t, fa.calls[0].script, `"Discovery"`)
}
