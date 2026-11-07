package cli_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/adapter/cli"
	"github.com/helmedeiros/amp/internal/music"
)

func playingStatus() music.Status {
	return music.Status{
		State:   music.Playing,
		Volume:  music.NewVolume(60),
		Shuffle: true,
		Repeat:  music.RepeatAll,
		Elapsed: 117 * time.Second,
		Track: music.Track{
			Name:     "Gorgon",
			Artist:   "Utsu-P",
			Album:    "Unknown Album",
			Duration: 255 * time.Second,
		},
	}
}

func TestRenderStatusHuman(t *testing.T) {
	t.Parallel()

	got := cli.RenderStatus(playingStatus(), cli.PlainTheme)

	assert.Contains(t, got, "playing  Utsu-P — Gorgon") // state label + aligned track
	assert.Contains(t, got, "album    Unknown Album")
	assert.Contains(t, got, "time     01:57 / 04:15") // zero-padded elapsed / duration
	assert.Contains(t, got, "━")                      // progress bar
	assert.Contains(t, got, "46%")                    // 117/255 ≈ 46%
	assert.Contains(t, got, "volume   60%")
	assert.Contains(t, got, "shuffle  on")
	assert.Contains(t, got, "repeat   all")
}

func TestRenderStatusHumanPausedMatchesPlaying(t *testing.T) {
	t.Parallel()

	// Pausing changes only the state word, not which fields are shown.
	base := playingStatus()
	playing := cli.RenderStatus(base, cli.PlainTheme)

	base.State = music.Paused
	paused := cli.RenderStatus(base, cli.PlainTheme)

	playingHead, playingRest, _ := strings.Cut(playing, "\n")
	pausedHead, pausedRest, _ := strings.Cut(paused, "\n")

	assert.Equal(t, playingRest, pausedRest, "every field below the state line must match")
	assert.Contains(t, playingHead, "playing")
	assert.Contains(t, pausedHead, "paused")
	assert.Contains(t, playingHead, "Utsu-P — Gorgon")
	assert.Contains(t, pausedHead, "Utsu-P — Gorgon")
}

func TestRenderStatusUnknownDuration(t *testing.T) {
	t.Parallel()

	s := music.Status{
		State:   music.Playing,
		Elapsed: 100 * time.Second,
		Track:   music.Track{Name: "Live Stream"}, // no duration
	}

	got := cli.RenderStatus(s, cli.PlainTheme)

	assert.Contains(t, got, "01:40 / --:--", "unknown duration shows a placeholder")
	assert.NotContains(t, got, "━", "no progress bar without a known duration")
}

func TestRenderStatusHumanStopped(t *testing.T) {
	t.Parallel()

	got := cli.RenderStatus(music.Status{State: music.Stopped, Volume: music.NewVolume(50)}, cli.PlainTheme)

	assert.Contains(t, got, "stopped")
	assert.Contains(t, got, "volume   50%")
	assert.NotContains(t, got, "time")  // no track ⇒ no time/bar
	assert.NotContains(t, got, "album") // no track ⇒ no album
	assert.NotContains(t, got, "━")
}

func TestRenderStatusJSON(t *testing.T) {
	t.Parallel()

	got := cli.RenderStatusJSON(playingStatus())

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &payload))

	assert.Equal(t, "playing", payload["state"])
	assert.Equal(t, float64(60), payload["volume"])
	assert.Equal(t, true, payload["shuffle"])
	assert.Equal(t, "all", payload["repeat"])
	assert.InDelta(t, 117.0, payload["elapsed_seconds"], 0.001)

	track, ok := payload["track"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Gorgon", track["name"])
	assert.Equal(t, "Utsu-P", track["artist"])
	assert.InDelta(t, 255.0, track["duration_seconds"], 0.001)
}

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestColorThemeAddsOnlyEscapeCodes(t *testing.T) {
	t.Parallel()

	s := playingStatus()

	colored := cli.RenderStatus(s, cli.ColorTheme())
	plain := cli.RenderStatus(s, cli.PlainTheme)

	assert.Contains(t, colored, "\x1b[", "color theme must emit ANSI codes")
	assert.Equal(t, plain, ansiPattern.ReplaceAllString(colored, ""),
		"stripping color codes must reproduce the plain layout exactly (alignment preserved)")
}

func TestColorThemeStateColors(t *testing.T) {
	t.Parallel()

	playing := cli.RenderStatus(music.Status{State: music.Playing, Track: music.Track{Name: "x"}}, cli.ColorTheme())
	paused := cli.RenderStatus(music.Status{State: music.Paused, Track: music.Track{Name: "x"}}, cli.ColorTheme())
	stopped := cli.RenderStatus(music.Status{State: music.Stopped}, cli.ColorTheme())

	assert.Contains(t, playing, "\x1b[32m", "playing is green")
	assert.Contains(t, paused, "\x1b[33m", "paused is yellow")
	assert.Contains(t, stopped, "\x1b[90m", "stopped is grey")
}

func TestRenderTracks(t *testing.T) {
	t.Parallel()

	tracks := []music.Track{
		{Name: "Gorgon", Artist: "Utsu-P", Album: "X", Duration: 255 * time.Second},
		{Name: "Solo", Artist: "Nobody"}, // no album, no duration
	}

	got := cli.RenderTracks(tracks)

	assert.Contains(t, got, "Utsu-P — Gorgon (X)  04:15")
	assert.Contains(t, got, "Nobody — Solo")
	assert.NotContains(t, got, "Solo (") // no empty album parens
	assert.Equal(t, "no matches", cli.RenderTracks(nil))
}

func TestRenderTracksJSON(t *testing.T) {
	t.Parallel()

	got := cli.RenderTracksJSON([]music.Track{{Name: "Gorgon", Artist: "Utsu-P", Duration: 255 * time.Second}})

	assert.JSONEq(t, `[{"name":"Gorgon","artist":"Utsu-P","album":"","duration_seconds":255}]`, got)
}

func TestRenderNames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Daft Punk\nUtsu-P", cli.RenderNames([]string{"Daft Punk", "Utsu-P"}, false))
	assert.Equal(t, "empty", cli.RenderNames(nil, false))
	assert.JSONEq(t, `["Daft Punk"]`, cli.RenderNames([]string{"Daft Punk"}, true))
	assert.JSONEq(t, `[]`, cli.RenderNames(nil, true))
}

func TestRenderPlaylists(t *testing.T) {
	t.Parallel()

	got := cli.RenderPlaylists([]music.Playlist{{Name: "Chill", Count: 42}, {Name: "Focus", Count: 7}})

	assert.Contains(t, got, "Chill  (42)")
	assert.Contains(t, got, "Focus  (7)")
	assert.Equal(t, "no playlists", cli.RenderPlaylists(nil))
}

func TestRenderPlaylistsJSON(t *testing.T) {
	t.Parallel()

	got := cli.RenderPlaylistsJSON([]music.Playlist{{Name: "Chill", Count: 42}})

	assert.JSONEq(t, `[{"name":"Chill","count":42}]`, got)
}

func TestRenderNow(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Utsu-P — Gorgon", cli.RenderNow(playingStatus()))
	assert.Equal(t, "nothing playing", cli.RenderNow(music.Status{State: music.Stopped}))
}

func TestProgressBar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fraction float64
		width    int
		want     string
	}{
		{name: "empty", fraction: 0, width: 5, want: "─────"},
		{name: "full", fraction: 1, width: 5, want: "━━━━━"},
		{name: "half", fraction: 0.5, width: 10, want: "━━━━━─────"},
		{name: "clamps below zero", fraction: -1, width: 4, want: "────"},
		{name: "clamps above one", fraction: 2, width: 4, want: "━━━━"},
		{name: "zero width is empty", fraction: 0.5, width: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, cli.ProgressBar(tt.fraction, tt.width))
		})
	}
}

func TestProgressBarAlwaysRendersWidthCells(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 20, len([]rune(cli.ProgressBar(0.37, 20))))
}

func TestFormatClock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   time.Duration
		want string
	}{
		{in: 0, want: "00:00"},
		{in: 9 * time.Second, want: "00:09"},
		{in: 117 * time.Second, want: "01:57"},
		{in: 255 * time.Second, want: "04:15"},
		{in: 3725 * time.Second, want: "62:05"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, cli.FormatClock(tt.in))
	}
}

func TestRenderStatusShowsSource(t *testing.T) {
	t.Parallel()

	s := playingStatus()
	assert.NotContains(t, cli.RenderStatus(s, cli.PlainTheme), "from", "unknown source is not shown")

	s.Source = music.Source{Kind: music.SourcePlaylist, Name: "Chill"}
	assert.Contains(t, cli.RenderStatus(s, cli.PlainTheme), "from     playlist “Chill”")
}

func TestRenderStatusJSONSource(t *testing.T) {
	t.Parallel()

	s := playingStatus()
	s.Source = music.Source{Kind: music.SourceExternal}

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(cli.RenderStatusJSON(s)), &payload))

	assert.Equal(t, map[string]any{"kind": "external"}, payload["source"])
}

func TestRenderStatusJSONOmitsUnknownSource(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(cli.RenderStatusJSON(playingStatus())), &payload))

	assert.NotContains(t, payload, "source")
}
