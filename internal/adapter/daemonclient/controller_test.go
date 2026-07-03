package daemonclient_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/adapter/daemonclient"
	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// innerStub is a Controller whose only meaningful method is Status. Embedding
// the interface satisfies the rest (they must not be called in these tests).
type innerStub struct {
	port.Controller
	status music.Status
	called bool
}

func (s *innerStub) Status(context.Context) (music.Status, error) {
	s.called = true
	return s.status, nil
}

type fakeDaemon struct {
	status music.Status
	err    error
}

func (f fakeDaemon) Status(context.Context) (music.Status, error) { return f.status, f.err }

func TestControllerPrefersDaemon(t *testing.T) {
	t.Parallel()

	inner := &innerStub{status: music.Status{State: music.Stopped}}
	daemon := fakeDaemon{status: music.Status{State: music.Playing, Volume: music.NewVolume(70)}}
	ctrl := daemonclient.NewController(inner, daemon)

	got, err := ctrl.Status(context.Background())

	require.NoError(t, err)
	assert.Equal(t, music.Playing, got.State)
	assert.Equal(t, 70, got.Volume.Int())
	assert.False(t, inner.called, "daemon hit means no direct read")
}

func TestControllerFallsBackWhenDaemonFails(t *testing.T) {
	t.Parallel()

	inner := &innerStub{status: music.Status{State: music.Paused}}
	daemon := fakeDaemon{err: errors.New("no daemon")}
	ctrl := daemonclient.NewController(inner, daemon)

	got, err := ctrl.Status(context.Background())

	require.NoError(t, err)
	assert.Equal(t, music.Paused, got.State)
	assert.True(t, inner.called, "fell back to the direct controller")
}
