package applescript

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const library = `{
	"id": ["P1", "P2", "P3"],
	"name": ["Daft Punk Is Playing at My House", "One More Time", "Gorgon"],
	"artist": ["LCD Soundsystem", "Daft Punk", "Utsu-P"],
	"album": ["LCD Soundsystem", "Discovery", "X"],
	"duration": [313.0, 320.0, 255.0],
	"plays": [9, 1, 0],
	"number": [1, 1, 4]
}`

func TestParseTracks(t *testing.T) {
	t.Parallel()

	raw := []byte(`[
		{"name": "Gorgon", "artist": "Utsu-P", "album": "X", "duration": 255.0},
		{"name": "Vulgar", "artist": "Utsu-P", "album": "X", "duration": 215.0}
	]`)

	got, err := parseTracks(raw)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Gorgon", got[0].Name)
	assert.Equal(t, 255*time.Second, got[0].Duration)
	assert.Equal(t, "Vulgar", got[1].Name)
}

func TestParseTracksEmpty(t *testing.T) {
	t.Parallel()

	got, err := parseTracks([]byte(`[]`))

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseLibraryTracks(t *testing.T) {
	t.Parallel()

	got, err := parseLibraryTracks([]byte(library))

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "P2", got[1].ID)
	assert.Equal(t, "One More Time", got[1].Name)
	assert.Equal(t, 320*time.Second, got[1].Duration)
	assert.Equal(t, 9, got[0].Plays)
	assert.Equal(t, 4, got[2].TrackNumber)
}

func TestParseLibraryTracksNotRunning(t *testing.T) {
	t.Parallel()

	got, err := parseLibraryTracks([]byte(`{}`))

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPlayerSearchRanksBestMatchFirst(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{out: []byte(library)}
	p := newPlayer(fake)

	got, err := p.Search(context.Background(), "daft punk", 10)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "One More Time", got[0].Name)
	assert.Equal(t, "Daft Punk Is Playing at My House", got[1].Name)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, javaScript, fake.calls[0].lang)
	assert.Equal(t, libraryTracksScript, fake.calls[0].script)
}

func TestPlayerSearchAppliesLimitAfterRanking(t *testing.T) {
	t.Parallel()

	got, err := newPlayer(&fakeRunner{out: []byte(library)}).Search(context.Background(), "daft punk", 1)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "One More Time", got[0].Name)
}

func TestPlayerPlaySearchReplaysTheListShown(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{outs: [][]byte{[]byte(library), []byte(`{"queued":2}`)}}
	p := newPlayer(fake)
	_, err := p.Search(context.Background(), "daft punk", 50)
	require.NoError(t, err)

	err = p.PlaySearch(context.Background(), "daft punk", 50, 1)

	require.NoError(t, err)
	require.Len(t, fake.calls, 2)
	script := fake.calls[1].script
	assert.Contains(t, script, `const want = ["P1","P2"];`)
	assert.Contains(t, script, "Music.delete(pl.tracks);")
	assert.Contains(t, script, `Music.userPlaylists.byName("amp queue")`)
	assert.Contains(t, script, "if (true && n > 0) pl.play();")
}

func TestPlayerPlaySearchRanksWhenNotSearchedYet(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{outs: [][]byte{[]byte(library), []byte(`{"queued":2}`)}}

	err := newPlayer(fake).PlaySearch(context.Background(), "daft punk", 50, 0)

	require.NoError(t, err)
	require.Len(t, fake.calls, 2)
	assert.Contains(t, fake.calls[1].script, `const want = ["P2","P1"];`)
}

func TestPlayerPlaySearchNoMatchLeavesQueue(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{out: []byte(library)}

	err := newPlayer(fake).PlaySearch(context.Background(), "metallica", 50, 0)

	require.NoError(t, err)
	assert.Len(t, fake.calls, 1)
}

func TestRotate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []int{3, 1, 2}, rotate([]int{1, 2, 3}, 2))
	assert.Equal(t, []int{3, 1, 2}, rotate([]int{1, 2, 3}, -1))
	assert.Empty(t, rotate([]int{}, 4))
}
