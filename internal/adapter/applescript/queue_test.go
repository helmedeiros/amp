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
	assert.Contains(t, script, "res.slice(0, limit)")
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

func TestPlayPlaylistScript(t *testing.T) {
	t.Parallel()

	assert.Contains(t, playPlaylistScript(`My "Chill"`),
		`Music.userPlaylists.byName("My \"Chill\"").play();`)
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
