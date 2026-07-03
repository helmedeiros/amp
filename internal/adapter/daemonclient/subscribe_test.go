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

// streamServer accepts a subscribe request and writes the given responses.
func streamServer(t *testing.T, responses []daemon.Response) string {
	t.Helper()

	f, err := os.CreateTemp("/tmp", "amps")
	require.NoError(t, err)
	sock := f.Name()
	_ = f.Close()
	require.NoError(t, os.Remove(sock))
	t.Cleanup(func() { _ = os.Remove(sock) })

	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		var req daemon.Request
		_ = json.NewDecoder(conn).Decode(&req)
		enc := json.NewEncoder(conn)
		for _, r := range responses {
			if enc.Encode(r) != nil {
				break
			}
		}
	}()

	return sock
}

func statusResp(state string, vol int) daemon.Response {
	return daemon.Response{Event: "status", Status: &daemon.StatusDTO{State: state, Volume: vol}}
}

func TestClientSubscribe(t *testing.T) {
	t.Parallel()

	sock := streamServer(t, []daemon.Response{statusResp("paused", 20), statusResp("playing", 20)})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := daemonclient.New(sock).Subscribe(ctx)
	require.NoError(t, err)

	assert.Equal(t, music.Paused, (<-ch).State)
	assert.Equal(t, music.Playing, (<-ch).State)
}

func TestClientSubscribeNoDaemon(t *testing.T) {
	t.Parallel()

	_, err := daemonclient.New("/tmp/amp-missing.sock").Subscribe(context.Background())
	require.Error(t, err)
}
