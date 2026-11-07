package applescript

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

func TestParseStatusPlaying(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"running": true,
		"state": "playing",
		"volume": 60,
		"shuffle": true,
		"repeat": "all",
		"elapsed": 117.0,
		"track": {"name": "Gorgon", "artist": "Utsu-P", "album": "Unknown Album", "duration": 255.0}
	}`)

	got, err := parseStatus(raw)

	require.NoError(t, err)
	assert.Equal(t, music.Playing, got.State)
	assert.Equal(t, 60, got.Volume.Int())
	assert.True(t, got.Shuffle)
	assert.Equal(t, music.RepeatAll, got.Repeat)
	assert.Equal(t, 117*time.Second, got.Elapsed)
	assert.Equal(t, "Gorgon", got.Track.Name)
	assert.Equal(t, "Utsu-P", got.Track.Artist)
	assert.Equal(t, "Unknown Album", got.Track.Album)
	assert.Equal(t, 255*time.Second, got.Track.Duration)
}

func TestParseStatusStoppedHasNoTrack(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"running": true, "state": "stopped", "volume": 50, "shuffle": false, "repeat": "off"}`)

	got, err := parseStatus(raw)

	require.NoError(t, err)
	assert.Equal(t, music.Stopped, got.State)
	assert.False(t, got.HasTrack())
}

func TestParseStatusNullTrackHasNoTrack(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"running": true, "state": "paused", "volume": 50, "repeat": "off", "track": null}`)

	got, err := parseStatus(raw)

	require.NoError(t, err)
	assert.False(t, got.HasTrack())
}

func TestParseStatusNotRunning(t *testing.T) {
	t.Parallel()

	got, err := parseStatus([]byte(`{"running": false}`))

	require.ErrorIs(t, err, ErrNotRunning)
	assert.False(t, got.HasTrack())
}

func TestParseStatusFractionalSecondsTruncate(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"running": true, "state": "playing", "volume": 10, "repeat": "off",
		"elapsed": 1.9, "track": {"name": "x", "duration": 2.5}}`)

	got, err := parseStatus(raw)

	require.NoError(t, err)
	assert.Equal(t, 1900*time.Millisecond, got.Elapsed)
	assert.Equal(t, 2500*time.Millisecond, got.Track.Duration)
}

func TestParseStatusErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{name: "malformed json", raw: `{not json`},
		{name: "unknown state", raw: `{"running": true, "state": "rewinding", "repeat": "off"}`},
		{name: "unknown repeat", raw: `{"running": true, "state": "playing", "repeat": "sometimes"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseStatus([]byte(tt.raw))
			require.Error(t, err)
		})
	}
}

func TestParseStatusSource(t *testing.T) {
	t.Parallel()

	const track = `"track": {"name": "Derezzed", "artist": "Daft Punk", "album": "TRON", "duration": 104.0}`
	tests := []struct {
		name   string
		source string
		track  string
		want   music.Source
	}{
		{
			name: "amp queue", source: `{"class": "userPlaylist", "name": "amp queue"}`, track: track,
			want: music.Source{Kind: music.SourceQueue, Name: "amp queue"},
		},
		{
			name: "user playlist", source: `{"class": "userPlaylist", "name": "Chill"}`, track: track,
			want: music.Source{Kind: music.SourcePlaylist, Name: "Chill"},
		},
		{
			name: "subscription playlist", source: `{"class": "subscriptionPlaylist", "name": "Today's Hits"}`, track: track,
			want: music.Source{Kind: music.SourcePlaylist, Name: "Today's Hits"},
		},
		{
			name: "library", source: `{"class": "libraryPlaylist", "name": "Library"}`, track: track,
			want: music.Source{Kind: music.SourceLibrary, Name: "Library"},
		},
		{
			name: "streaming", source: `null`, track: track,
			want: music.Source{Kind: music.SourceExternal},
		},
		{
			name: "no track", source: `{"class": "userPlaylist", "name": "Chill"}`, track: `"track": null`,
			want: music.Source{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := []byte(`{"running": true, "state": "playing", "volume": 50, "repeat": "off", ` +
				tt.track + `, "source": ` + tt.source + `}`)

			got, err := parseStatus(raw)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Source)
		})
	}
}
