package daemonclient_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/adapter/daemonclient"
	"github.com/helmedeiros/amp/internal/daemon"
	"github.com/helmedeiros/amp/internal/music"
)

// fakeServer accepts one connection and replies with resp to any request.
func fakeServer(t *testing.T, resp daemon.Response) string {
	t.Helper()

	f, err := os.CreateTemp("/tmp", "amps") // short path for the socket limit
	require.NoError(t, err)
	sock := f.Name()
	_ = f.Close()
	require.NoError(t, os.Remove(sock))
	t.Cleanup(func() { _ = os.Remove(sock) })

	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			var req daemon.Request
			_ = json.NewDecoder(conn).Decode(&req)
			_ = json.NewEncoder(conn).Encode(resp)
			_ = conn.Close()
		}
	}()

	return sock
}

func TestClientStatus(t *testing.T) {
	t.Parallel()

	sock := fakeServer(t, daemon.Response{Status: &daemon.StatusDTO{
		State:  "playing",
		Volume: 60,
		Track:  &daemon.TrackDTO{Name: "Gorgon", Artist: "Utsu-P"},
	}})

	got, err := daemonclient.New(sock).Status(context.Background())

	require.NoError(t, err)
	assert.Equal(t, music.Playing, got.State)
	assert.Equal(t, 60, got.Volume.Int())
	assert.Equal(t, "Gorgon", got.Track.Name)
}

func TestClientStatusNoDaemon(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := daemonclient.New("/tmp/amp-does-not-exist.sock").Status(ctx)

	require.Error(t, err, "a missing daemon must error so callers can fall back")
}

func TestClientStatusServerError(t *testing.T) {
	t.Parallel()

	sock := fakeServer(t, daemon.Response{Error: "boom"})

	_, err := daemonclient.New(sock).Status(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
