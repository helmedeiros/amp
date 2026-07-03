package daemon

import (
	"context"
	"encoding/json"
	"net"

	"github.com/helmedeiros/amp/internal/music"
)

// Serve accepts connections on ln and handles each with the daemon until ctx is
// cancelled (which closes ln). It returns ctx.Err() on a clean shutdown.
func Serve(ctx context.Context, ln net.Listener, d *Daemon) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		go handleConn(ctx, conn, d)
	}
}

// handleConn serves a single request per connection. A "subscribe" request
// streams status changes until the client disconnects or the daemon stops.
func handleConn(ctx context.Context, conn net.Conn, d *Daemon) {
	defer func() { _ = conn.Close() }()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	enc := json.NewEncoder(conn)

	switch req.Cmd {
	case "ping":
		_ = enc.Encode(Response{Pong: true})

	case "status":
		dto := ToStatusDTO(currentStatus(d))
		_ = enc.Encode(Response{Status: &dto})

	case "subscribe":
		streamStatus(ctx, enc, d)

	default:
		_ = enc.Encode(Response{Error: "unknown command: " + req.Cmd})
	}
}

func streamStatus(ctx context.Context, enc *json.Encoder, d *Daemon) {
	ch, unsub := d.Subscribe()
	defer unsub()

	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-ch:
			if !ok {
				return
			}
			dto := ToStatusDTO(s)
			if err := enc.Encode(Response{Event: "status", Status: &dto}); err != nil {
				return
			}
		}
	}
}

// currentStatus returns the cached status, or a zero (stopped) status when the
// daemon has not completed its first poll.
func currentStatus(d *Daemon) music.Status {
	s, _ := d.Status()
	return s
}
