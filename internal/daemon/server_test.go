package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

type staticReader struct{ s music.Status }

func (r staticReader) Status(context.Context) (music.Status, error) { return r.s, nil }

// serveOn runs a socket server for d on a temp Unix socket and returns the path.
func serveOn(t *testing.T, d *Daemon) string {
	t.Helper()

	// Unix socket paths are length-limited (~104 chars on macOS), so keep them
	// short in /tmp rather than under the long test temp dir.
	f, err := os.CreateTemp("/tmp", "amps")
	require.NoError(t, err)
	sock := f.Name()
	_ = f.Close()
	require.NoError(t, os.Remove(sock))
	t.Cleanup(func() { _ = os.Remove(sock) })

	ln, err := net.Listen("unix", sock)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = Serve(ctx, ln, d) }()
	t.Cleanup(cancel)

	return sock
}

func dial(t *testing.T, sock string) net.Conn {
	t.Helper()
	conn, err := net.Dial("unix", sock)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestServerStatus(t *testing.T) {
	t.Parallel()

	d := New(staticReader{music.Status{State: music.Playing, Volume: music.NewVolume(60)}}, time.Hour)
	d.pollOnce(context.Background()) // warm the cache

	conn := dial(t, serveOn(t, d))
	require.NoError(t, json.NewEncoder(conn).Encode(Request{Cmd: "status"}))

	var resp Response
	require.NoError(t, json.NewDecoder(conn).Decode(&resp))
	require.NotNil(t, resp.Status)
	assert.Equal(t, "playing", resp.Status.State)
	assert.Equal(t, 60, resp.Status.Volume)
}

func TestServerPingAndUnknown(t *testing.T) {
	t.Parallel()

	sock := serveOn(t, New(staticReader{}, time.Hour))

	c1 := dial(t, sock)
	require.NoError(t, json.NewEncoder(c1).Encode(Request{Cmd: "ping"}))
	var pong Response
	require.NoError(t, json.NewDecoder(c1).Decode(&pong))
	assert.True(t, pong.Pong)

	c2 := dial(t, sock)
	require.NoError(t, json.NewEncoder(c2).Encode(Request{Cmd: "bogus"}))
	var unknown Response
	require.NoError(t, json.NewDecoder(c2).Decode(&unknown))
	assert.Contains(t, unknown.Error, "bogus")
}

func TestServerSubscribeStreamsChanges(t *testing.T) {
	t.Parallel()

	d := New(&scriptedReader{seq: []music.Status{
		{State: music.Paused, Volume: music.NewVolume(20)},
		{State: music.Playing, Volume: music.NewVolume(20)},
	}}, time.Hour)
	d.pollOnce(context.Background()) // cache the paused status

	conn := dial(t, serveOn(t, d))
	require.NoError(t, json.NewEncoder(conn).Encode(Request{Cmd: "subscribe"}))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	dec := json.NewDecoder(conn)

	var first Response
	require.NoError(t, dec.Decode(&first))
	assert.Equal(t, "status", first.Event)
	assert.Equal(t, "paused", first.Status.State)

	d.pollOnce(context.Background()) // playing -> change pushed

	var next Response
	require.NoError(t, dec.Decode(&next))
	assert.Equal(t, "playing", next.Status.State)
}
