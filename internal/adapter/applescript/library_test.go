package applescript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

func TestParseNames(t *testing.T) {
	t.Parallel()

	got, err := parseNames([]byte(`["Aretha Franklin","Daft Punk","Utsu-P"]`))

	require.NoError(t, err)
	assert.Equal(t, []string{"Aretha Franklin", "Daft Punk", "Utsu-P"}, got)
}

func TestNamesScriptUsesField(t *testing.T) {
	t.Parallel()

	assert.Contains(t, namesScript("artist"), ".tracks.artist()")
	assert.Contains(t, namesScript("album"), ".tracks.album()")
}

func TestParseAlbums(t *testing.T) {
	t.Parallel()

	got, err := parseAlbums([]byte(`[{"album":"Discovery","artist":"Daft Punk"},{"album":"Mixtape","artist":"Various Artists"}]`))

	require.NoError(t, err)
	assert.Equal(t, []music.Album{
		{Name: "Discovery", Artist: "Daft Punk"},
		{Name: "Mixtape", Artist: "Various Artists"},
	}, got)
}

func TestAlbumsScriptGroupsArtists(t *testing.T) {
	t.Parallel()

	assert.Contains(t, albumsScript, ".tracks")
	assert.Contains(t, albumsScript, "'Various Artists'")
}

func TestPlayerArtistsAndAlbums(t *testing.T) {
	t.Parallel()

	t.Run("artists", func(t *testing.T) {
		t.Parallel()
		fake := &fakeRunner{out: []byte(`["Daft Punk"]`)}
		p := newPlayer(fake)

		got, err := p.Artists(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []string{"Daft Punk"}, got)
		assert.Equal(t, namesScript("artist"), fake.calls[0].script)
	})

	t.Run("albums", func(t *testing.T) {
		t.Parallel()
		fake := &fakeRunner{out: []byte(`[{"album":"Discovery","artist":"Daft Punk"}]`)}
		p := newPlayer(fake)

		got, err := p.Albums(context.Background())

		require.NoError(t, err)
		assert.Equal(t, []music.Album{{Name: "Discovery", Artist: "Daft Punk"}}, got)
		assert.Equal(t, albumsScript, fake.calls[0].script)
	})
}
