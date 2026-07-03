// Command amd is the amp daemon: it polls Music.app, caches the latest status,
// and serves it (plus change events) to clients over a Unix socket.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/helmedeiros/amp/internal/adapter/applescript"
	"github.com/helmedeiros/amp/internal/daemon"
)

const pollInterval = 700 * time.Millisecond

func main() {
	sock := daemon.SocketPath()
	_ = os.Remove(sock) // clear any stale socket from a previous run

	ln, err := net.Listen("unix", sock)
	if err != nil {
		fmt.Fprintln(os.Stderr, "amd:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	d := daemon.New(applescript.New(), pollInterval)
	go func() { _ = d.Run(ctx) }()

	fmt.Fprintf(os.Stderr, "amd listening on %s\n", sock)
	_ = daemon.Serve(ctx, ln, d)
	_ = os.Remove(sock)
}
