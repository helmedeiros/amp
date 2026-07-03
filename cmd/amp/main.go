// Command amp controls Apple Music from the terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/helmedeiros/amp/internal/adapter/applescript"
	"github.com/helmedeiros/amp/internal/adapter/cli"
	"github.com/helmedeiros/amp/internal/adapter/daemonclient"
	"github.com/helmedeiros/amp/internal/adapter/store"
	"github.com/helmedeiros/amp/internal/app"
	"github.com/helmedeiros/amp/internal/daemon"
	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

func main() {
	svc := app.NewService(applescript.New(), store.NewFile(volumeStatePath()))

	// Serve status from the daemon when it is running (fast, cached); every
	// other call, and status when the daemon is down, goes direct.
	client := daemonclient.New(daemon.SocketPath())
	ctrl := daemonclient.NewController(svc, client)

	// The TUI subscribes to the daemon's event stream, or polls when it is down.
	stream := func(ctx context.Context) (<-chan music.Status, error) {
		if ch, err := client.Subscribe(ctx); err == nil {
			return ch, nil
		}
		return pollStatus(ctx, ctrl, time.Second), nil
	}

	root := cli.NewRootCmd(ctrl, stream)

	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "amp:", err)
		os.Exit(1)
	}
}

// pollStatus emits the controller's status on an interval, for the TUI when the
// daemon is not running.
func pollStatus(ctx context.Context, ctrl port.Controller, interval time.Duration) <-chan music.Status {
	ch := make(chan music.Status)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		emit := func() {
			if s, err := ctrl.Status(ctx); err == nil {
				select {
				case ch <- s:
				case <-ctx.Done():
				}
			}
		}
		emit()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				emit()
			}
		}
	}()
	return ch
}

// volumeStatePath returns where the pre-mute volume is remembered, under the
// user config dir, falling back to the working directory if it is unavailable.
func volumeStatePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ".am-volume"
	}
	return filepath.Join(dir, "amp", "volume")
}
