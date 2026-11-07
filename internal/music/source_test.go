package music_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/helmedeiros/amp/internal/music"
)

func TestParseSourceKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  music.SourceKind
	}{
		{input: "queue", want: music.SourceQueue},
		{input: "playlist", want: music.SourcePlaylist},
		{input: "library", want: music.SourceLibrary},
		{input: "external", want: music.SourceExternal},
		{input: " Playlist\n", want: music.SourcePlaylist},
		{input: "", want: music.SourceNone},
		{input: "radio", want: music.SourceNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, music.ParseSourceKind(tt.input))
		})
	}
}

func TestSourceKindStringRoundTrips(t *testing.T) {
	t.Parallel()

	for _, k := range []music.SourceKind{music.SourceNone, music.SourceQueue, music.SourcePlaylist, music.SourceLibrary, music.SourceExternal} {
		assert.Equal(t, k, music.ParseSourceKind(k.String()))
	}
}

func TestSourceIsExternal(t *testing.T) {
	t.Parallel()

	assert.False(t, music.Source{}.IsExternal(), "unknown source is not followed")
	assert.False(t, music.Source{Kind: music.SourceQueue, Name: "amp queue"}.IsExternal())
	assert.True(t, music.Source{Kind: music.SourcePlaylist, Name: "Chill"}.IsExternal())
	assert.True(t, music.Source{Kind: music.SourceLibrary, Name: "Library"}.IsExternal())
	assert.True(t, music.Source{Kind: music.SourceExternal}.IsExternal())
}

func TestSourceLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", music.Source{}.Label())
	assert.Equal(t, "amp queue", music.Source{Kind: music.SourceQueue, Name: "amp queue"}.Label())
	assert.Equal(t, "playlist “Chill”", music.Source{Kind: music.SourcePlaylist, Name: "Chill"}.Label())
	assert.Equal(t, "library", music.Source{Kind: music.SourceLibrary, Name: "Library"}.Label())
	assert.Equal(t, "Apple Music", music.Source{Kind: music.SourceExternal}.Label())
}
