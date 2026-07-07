package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/adapter/cli"
	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// fakeController records calls and returns canned data.
type fakeController struct {
	status       music.Status
	calls        []string
	setLevel     int
	adjustBy     int
	volRet       music.Volume
	shuffleSet   bool
	shuffleRet   bool
	repeatSet    music.RepeatMode
	searchQuery  string
	searchLimit  int
	searchResult []music.Track
	playlists    []music.Playlist
	names        []string
	albums       []music.Album
	seekMode     music.SeekMode
	seekValue    float64
	seekRet      time.Duration
	playStart    int
	playQuery    string
	playRet      port.PlayResult
}

func (f *fakeController) Status(context.Context) (music.Status, error) {
	f.calls = append(f.calls, "Status")
	return f.status, nil
}
func (f *fakeController) Open(context.Context) error { f.calls = append(f.calls, "Open"); return nil }

func (f *fakeController) SaveArtwork(_ context.Context, path string) error {
	f.calls = append(f.calls, "SaveArtwork")
	f.playQuery = path
	return nil
}

func (f *fakeController) PlayQuery(_ context.Context, query string, limit int) (port.PlayResult, error) {
	f.calls = append(f.calls, "PlayQuery")
	f.playQuery, f.searchLimit = query, limit
	return f.playRet, nil
}

func (f *fakeController) Search(_ context.Context, query string, limit int) ([]music.Track, error) {
	f.calls = append(f.calls, "Search")
	f.searchQuery, f.searchLimit = query, limit
	return f.searchResult, nil
}

func (f *fakeController) PlaySearch(_ context.Context, query string, limit, start int) error {
	f.calls = append(f.calls, "PlaySearch")
	f.searchQuery, f.searchLimit, f.playStart = query, limit, start
	return nil
}

func (f *fakeController) PlayQueueAt(_ context.Context, index int) error {
	f.calls = append(f.calls, "PlayQueueAt")
	f.playStart = index
	return nil
}

func (f *fakeController) Queue(context.Context) ([]music.Track, error) {
	f.calls = append(f.calls, "Queue")
	return f.searchResult, nil
}

func (f *fakeController) QueueAdd(_ context.Context, query string, limit int) (int, error) {
	f.calls = append(f.calls, "QueueAdd")
	f.searchQuery, f.searchLimit = query, limit
	return len(f.searchResult), nil
}

func (f *fakeController) QueueClear(context.Context) error {
	f.calls = append(f.calls, "QueueClear")
	return nil
}

func (f *fakeController) Playlists(context.Context) ([]music.Playlist, error) {
	f.calls = append(f.calls, "Playlists")
	return f.playlists, nil
}

func (f *fakeController) Artists(context.Context) ([]string, error) {
	f.calls = append(f.calls, "Artists")
	return f.names, nil
}

func (f *fakeController) Albums(context.Context) ([]music.Album, error) {
	f.calls = append(f.calls, "Albums")
	return f.albums, nil
}
func (f *fakeController) CatalogEnabled() bool { return false }
func (f *fakeController) ArtistCatalogAlbums(context.Context, string) ([]music.CatalogAlbum, error) {
	return nil, nil
}
func (f *fakeController) AddCatalogAlbums(context.Context, []string) (int, error) { return 0, nil }
func (f *fakeController) Play(context.Context) error                              { f.calls = append(f.calls, "Play"); return nil }
func (f *fakeController) Pause(context.Context) error                             { f.calls = append(f.calls, "Pause"); return nil }
func (f *fakeController) Toggle(context.Context) error {
	f.calls = append(f.calls, "Toggle")
	return nil
}
func (f *fakeController) Stop(context.Context) error { f.calls = append(f.calls, "Stop"); return nil }
func (f *fakeController) Next(context.Context) error { f.calls = append(f.calls, "Next"); return nil }
func (f *fakeController) Previous(context.Context) error {
	f.calls = append(f.calls, "Previous")
	return nil
}

func (f *fakeController) SetVolume(_ context.Context, level int) (music.Volume, error) {
	f.calls = append(f.calls, "SetVolume")
	f.setLevel = level
	return f.volRet, nil
}

func (f *fakeController) AdjustVolume(_ context.Context, delta int) (music.Volume, error) {
	f.calls = append(f.calls, "AdjustVolume")
	f.adjustBy = delta
	return f.volRet, nil
}

func (f *fakeController) Seek(_ context.Context, mode music.SeekMode, value float64) (time.Duration, error) {
	f.calls = append(f.calls, "Seek")
	f.seekMode, f.seekValue = mode, value
	return f.seekRet, nil
}

func (f *fakeController) SetShuffle(_ context.Context, enabled bool) error {
	f.calls = append(f.calls, "SetShuffle")
	f.shuffleSet = enabled
	return nil
}

func (f *fakeController) ToggleShuffle(context.Context) (bool, error) {
	f.calls = append(f.calls, "ToggleShuffle")
	return f.shuffleRet, nil
}

func (f *fakeController) SetRepeat(_ context.Context, mode music.RepeatMode) error {
	f.calls = append(f.calls, "SetRepeat")
	f.repeatSet = mode
	return nil
}

func (f *fakeController) Mute(context.Context) error {
	f.calls = append(f.calls, "Mute")
	return nil
}

func (f *fakeController) Unmute(context.Context) (music.Volume, error) {
	f.calls = append(f.calls, "Unmute")
	return f.volRet, nil
}

func run(t *testing.T, ctrl *fakeController, args ...string) string {
	t.Helper()

	var out bytes.Buffer
	cmd := cli.NewRootCmd(ctrl, nil)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	require.NoError(t, cmd.Execute())
	return out.String()
}

func TestStatusCommandHuman(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{status: music.Status{
		State: music.Playing,
		Track: music.Track{Name: "Gorgon", Artist: "Utsu-P"},
	}}

	out := run(t, ctrl, "status")

	assert.Contains(t, out, "playing  Utsu-P — Gorgon")
	assert.Equal(t, []string{"Status"}, ctrl.calls)
}

func TestPlayCommandResume(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{playRet: port.PlayResult{Kind: "resume"}}

	out := run(t, ctrl, "play")

	assert.Equal(t, []string{"PlayQuery"}, ctrl.calls)
	assert.Equal(t, "", ctrl.playQuery)
	assert.Empty(t, out, "resume prints nothing")
}

func TestPlayCommandQuery(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{playRet: port.PlayResult{Kind: "playlist", Label: "Chill"}}

	out := run(t, ctrl, "play", "chill")

	assert.Equal(t, []string{"PlayQuery"}, ctrl.calls)
	assert.Equal(t, "chill", ctrl.playQuery)
	assert.Contains(t, out, "▶ playlist: Chill")
}

func TestNowCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{status: music.Status{
		State: music.Playing,
		Track: music.Track{Name: "Gorgon", Artist: "Utsu-P"},
	}}

	out := run(t, ctrl, "now")

	assert.Equal(t, "Utsu-P — Gorgon\n", out)
	assert.Equal(t, []string{"Status"}, ctrl.calls)
}

func TestSearchCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{searchResult: []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}}}

	out := run(t, ctrl, "search", "utsu", "p")

	assert.Equal(t, []string{"Search"}, ctrl.calls)
	assert.Equal(t, "utsu p", ctrl.searchQuery, "args are joined into the query")
	assert.Equal(t, 50, ctrl.searchLimit, "default limit")
	assert.Contains(t, out, "Utsu-P — Gorgon")
}

func TestSearchCommandJSONAndLimit(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{searchResult: []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}}}

	out := run(t, ctrl, "search", "--json", "--limit", "5", "utsu")

	assert.Equal(t, 5, ctrl.searchLimit)
	assert.Contains(t, out, `"name":"Gorgon"`)
}

func TestQueueShowCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{searchResult: []music.Track{{Name: "Gorgon", Artist: "Utsu-P"}}}
	out := run(t, ctrl, "queue")

	assert.Equal(t, []string{"Queue"}, ctrl.calls)
	assert.Contains(t, out, "Utsu-P — Gorgon")
}

func TestQueueShowEmpty(t *testing.T) {
	t.Parallel()

	out := run(t, &fakeController{}, "queue")
	assert.Contains(t, out, "queue is empty")
}

func TestQueueAddCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{searchResult: []music.Track{{Name: "a"}, {Name: "b"}}}
	out := run(t, ctrl, "queue", "add", "daft", "punk")

	assert.Equal(t, []string{"QueueAdd"}, ctrl.calls)
	assert.Equal(t, "daft punk", ctrl.searchQuery)
	assert.Contains(t, out, "added 2 to queue")
}

func TestQueueClearCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{}
	out := run(t, ctrl, "queue", "clear")

	assert.Equal(t, []string{"QueueClear"}, ctrl.calls)
	assert.Contains(t, out, "queue cleared")
}

func TestPlaylistsCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{playlists: []music.Playlist{{Name: "Chill", Count: 42}}}

	out := run(t, ctrl, "playlists")

	assert.Equal(t, []string{"Playlists"}, ctrl.calls)
	assert.Contains(t, out, "Chill  (42)")
}

func TestPlaylistsCommandJSON(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{playlists: []music.Playlist{{Name: "Chill", Count: 42}}}

	out := run(t, ctrl, "playlists", "--json")

	assert.Contains(t, out, `"name":"Chill"`)
}

func TestLibraryArtistsCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{names: []string{"Daft Punk", "Utsu-P"}}

	out := run(t, ctrl, "library", "artists")

	assert.Equal(t, []string{"Artists"}, ctrl.calls)
	assert.Contains(t, out, "Daft Punk")
	assert.Contains(t, out, "Utsu-P")
}

func TestLibraryAlbumsCommandJSON(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{albums: []music.Album{{Name: "Discovery", Artist: "Daft Punk"}}}

	out := run(t, ctrl, "library", "albums", "--json")

	assert.Equal(t, []string{"Albums"}, ctrl.calls)
	assert.JSONEq(t, `[{"name":"Discovery","artist":"Daft Punk"}]`, strings.TrimSpace(out))
}

func TestArtworkCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{}
	out := run(t, ctrl, "artwork", "-o", "art.jpg")

	assert.Equal(t, []string{"SaveArtwork"}, ctrl.calls)
	assert.Contains(t, ctrl.playQuery, "art.jpg", "path passed through (absolute)")
	assert.Contains(t, out, "art.jpg")
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	out := run(t, &fakeController{}, "--version")

	assert.Contains(t, out, "amp version")
}

func TestStatusCommandJSON(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{status: music.Status{State: music.Paused, Volume: music.NewVolume(33)}}

	out := run(t, ctrl, "status", "--json")

	assert.Contains(t, out, `"state":"paused"`)
	assert.Contains(t, out, `"volume":33`)
}

func TestTransportCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arg  string
		want string
	}{
		{arg: "open", want: "Open"},
		{arg: "pause", want: "Pause"},
		{arg: "toggle", want: "Toggle"},
		{arg: "stop", want: "Stop"},
		{arg: "next", want: "Next"},
		{arg: "prev", want: "Previous"},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			t.Parallel()

			ctrl := &fakeController{}
			run(t, ctrl, tt.arg)
			assert.Equal(t, []string{tt.want}, ctrl.calls)
		})
	}
}

func TestVolCommandAbsolute(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{volRet: music.NewVolume(42)}

	out := run(t, ctrl, "vol", "42")

	assert.Equal(t, []string{"SetVolume"}, ctrl.calls)
	assert.Equal(t, 42, ctrl.setLevel)
	assert.Contains(t, out, "42")
}

func TestVolCommandRelative(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{volRet: music.NewVolume(70)}

	run(t, ctrl, "vol", "+10")

	assert.Equal(t, []string{"AdjustVolume"}, ctrl.calls)
	assert.Equal(t, 10, ctrl.adjustBy)
}

func TestSeekCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{seekRet: 90 * time.Second}

	out := run(t, ctrl, "seek", "1:30")

	assert.Equal(t, []string{"Seek"}, ctrl.calls)
	assert.Equal(t, music.SeekAbsolute, ctrl.seekMode)
	assert.InDelta(t, 90, ctrl.seekValue, 0.001)
	assert.Contains(t, out, "position 01:30")
}

func TestSeekCommandRelativeNegative(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{seekRet: 80 * time.Second}

	// A leading-dash argument must reach the command, not be parsed as a flag.
	run(t, ctrl, "seek", "-10")

	assert.Equal(t, music.SeekRelative, ctrl.seekMode)
	assert.InDelta(t, -10, ctrl.seekValue, 0.001)
}

func TestVolCommandRelativeNegative(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{volRet: music.NewVolume(20)}

	// A leading-dash argument must reach the command, not be parsed as a flag.
	run(t, ctrl, "vol", "-20")

	assert.Equal(t, []string{"AdjustVolume"}, ctrl.calls)
	assert.Equal(t, -20, ctrl.adjustBy)
}

func TestShuffleCommand(t *testing.T) {
	t.Parallel()

	t.Run("on", func(t *testing.T) {
		t.Parallel()
		ctrl := &fakeController{}
		run(t, ctrl, "shuffle", "on")
		assert.Equal(t, []string{"SetShuffle"}, ctrl.calls)
		assert.True(t, ctrl.shuffleSet)
	})

	t.Run("off", func(t *testing.T) {
		t.Parallel()
		ctrl := &fakeController{}
		run(t, ctrl, "shuffle", "off")
		assert.Equal(t, []string{"SetShuffle"}, ctrl.calls)
		assert.False(t, ctrl.shuffleSet)
	})

	t.Run("toggle is the default with no arg", func(t *testing.T) {
		t.Parallel()
		ctrl := &fakeController{shuffleRet: true}
		out := run(t, ctrl, "shuffle")
		assert.Equal(t, []string{"ToggleShuffle"}, ctrl.calls)
		assert.Contains(t, out, "on")
	})
}

func TestRepeatCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arg  string
		want music.RepeatMode
	}{
		{arg: "off", want: music.RepeatOff},
		{arg: "one", want: music.RepeatOne},
		{arg: "all", want: music.RepeatAll},
	}

	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			t.Parallel()
			ctrl := &fakeController{}
			run(t, ctrl, "repeat", tt.arg)
			assert.Equal(t, []string{"SetRepeat"}, ctrl.calls)
			assert.Equal(t, tt.want, ctrl.repeatSet)
		})
	}
}

func TestMuteCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{}
	out := run(t, ctrl, "mute")

	assert.Equal(t, []string{"Mute"}, ctrl.calls)
	assert.Contains(t, out, "muted")
}

func TestUnmuteCommand(t *testing.T) {
	t.Parallel()

	ctrl := &fakeController{volRet: music.NewVolume(80)}
	out := run(t, ctrl, "unmute")

	assert.Equal(t, []string{"Unmute"}, ctrl.calls)
	assert.Contains(t, out, "vol 80%")
}

func TestRepeatCommandRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd := cli.NewRootCmd(&fakeController{}, nil)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"repeat", "sometimes"})

	require.Error(t, cmd.Execute())
}
