// Package daemonclient connects to amd over its Unix socket and exposes the
// daemon's cached status to the CLI (and later the TUI), with a Controller
// decorator that transparently falls back to direct engine access.
package daemonclient

import (
	"context"
	"encoding/json"
	"errors"
	"net"

	"github.com/helmedeiros/amp/internal/daemon"
	"github.com/helmedeiros/amp/internal/music"
)

// Client talks to amd over its Unix socket.
type Client struct {
	socket string
}

// New returns a Client for the given socket path.
func New(socket string) *Client {
	return &Client{socket: socket}
}

// Status reads the daemon's cached status. It returns an error (fast) when the
// daemon is not running, so callers can fall back to direct access.
func (c *Client) Status(ctx context.Context) (music.Status, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	if err != nil {
		return music.Status{}, err
	}
	defer func() { _ = conn.Close() }()

	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	if err := json.NewEncoder(conn).Encode(daemon.Request{Cmd: "status"}); err != nil {
		return music.Status{}, err
	}

	var resp daemon.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return music.Status{}, err
	}
	if resp.Error != "" {
		return music.Status{}, errors.New(resp.Error)
	}
	if resp.Status == nil {
		return music.Status{}, nil
	}
	return resp.Status.ToStatus(), nil
}

// Subscribe opens a streaming connection and delivers the current status plus
// every change on the returned channel until ctx is cancelled or the daemon
// closes the connection. It errors (fast) when the daemon is not running.
func (c *Client) Subscribe(ctx context.Context) (<-chan music.Status, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.socket)
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(conn).Encode(daemon.Request{Cmd: "subscribe"}); err != nil {
		_ = conn.Close()
		return nil, err
	}

	out := make(chan music.Status)
	go func() {
		defer close(out)
		defer func() { _ = conn.Close() }()
		go func() { <-ctx.Done(); _ = conn.Close() }() // unblock the decoder on cancel

		dec := json.NewDecoder(conn)
		for {
			var resp daemon.Response
			if err := dec.Decode(&resp); err != nil {
				return
			}
			if resp.Status == nil {
				continue
			}
			select {
			case out <- resp.Status.ToStatus():
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
