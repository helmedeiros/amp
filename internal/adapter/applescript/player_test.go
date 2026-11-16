package applescript

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

type call struct {
	lang   language
	script string
}

type fakeRunner struct {
	out   []byte
	outs  [][]byte
	err   error
	calls []call
}

func (f *fakeRunner) Run(_ context.Context, lang language, script string) ([]byte, error) {
	f.calls = append(f.calls, call{lang: lang, script: script})
	if len(f.outs) > 0 {
		out := f.outs[0]
		f.outs = f.outs[1:]
		return out, f.err
	}
	return f.out, f.err
}

func TestPlayerStatusRunsJavaScriptAndParses(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{out: []byte(`{"running":true,"state":"playing","volume":60,"repeat":"off"}`)}
	p := newPlayer(fake)

	got, err := p.Status(context.Background())

	require.NoError(t, err)
	assert.Equal(t, music.Playing, got.State)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, javaScript, fake.calls[0].lang)
	assert.Equal(t, statusScript, fake.calls[0].script)
}

func TestPlayerStatusSurfacesRunnerError(t *testing.T) {
	t.Parallel()

	boom := errors.New("osascript: not allowed")
	p := newPlayer(&fakeRunner{err: boom})

	_, err := p.Status(context.Background())

	require.ErrorIs(t, err, boom)
}

func TestPlayerTransportSendsAppleScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		act  func(*Player, context.Context) error
		want string
	}{
		{name: "open", act: (*Player).Open, want: tellMusic("activate")},
		{name: "play", act: (*Player).Play, want: tellMusic("play")},
		{name: "pause", act: (*Player).Pause, want: tellMusic("pause")},
		{name: "toggle", act: (*Player).TogglePlayPause, want: tellMusic("playpause")},
		{name: "stop", act: (*Player).Stop, want: tellMusic("stop")},
		{name: "next", act: (*Player).Next, want: tellMusic("next track")},
		{name: "previous", act: (*Player).Previous, want: tellMusic("previous track")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeRunner{}
			p := newPlayer(fake)

			require.NoError(t, tt.act(p, context.Background()))

			require.Len(t, fake.calls, 1)
			assert.Equal(t, appleScript, fake.calls[0].lang)
			assert.Equal(t, tt.want, fake.calls[0].script)
		})
	}
}

func TestPlayerSetVolumeScript(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{}
	p := newPlayer(fake)

	require.NoError(t, p.SetVolume(context.Background(), music.NewVolume(42)))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, appleScript, fake.calls[0].lang)
	assert.Equal(t, tellMusic("set sound volume to 42"), fake.calls[0].script)
}

func TestPlayerSetPositionScript(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{}
	p := newPlayer(fake)

	require.NoError(t, p.SetPosition(context.Background(), 90.5))

	require.Len(t, fake.calls, 1)
	assert.Equal(t, tellMusic("set player position to 90.5"), fake.calls[0].script)
}

func TestPlayerSetShuffleScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		enabled bool
		want    string
	}{
		{enabled: true, want: tellMusic("set shuffle enabled to true")},
		{enabled: false, want: tellMusic("set shuffle enabled to false")},
	}

	for _, tt := range tests {
		fake := &fakeRunner{}
		p := newPlayer(fake)

		require.NoError(t, p.SetShuffle(context.Background(), tt.enabled))

		require.Len(t, fake.calls, 1)
		assert.Equal(t, appleScript, fake.calls[0].lang)
		assert.Equal(t, tt.want, fake.calls[0].script)
	}
}

func TestPlayerSetRepeatScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode music.RepeatMode
		want string
	}{
		{mode: music.RepeatOff, want: tellMusic("set song repeat to off")},
		{mode: music.RepeatOne, want: tellMusic("set song repeat to one")},
		{mode: music.RepeatAll, want: tellMusic("set song repeat to all")},
	}

	for _, tt := range tests {
		fake := &fakeRunner{}
		p := newPlayer(fake)

		require.NoError(t, p.SetRepeat(context.Background(), tt.mode))

		require.Len(t, fake.calls, 1)
		assert.Equal(t, tt.want, fake.calls[0].script)
	}
}
