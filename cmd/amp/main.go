// Command amp controls Apple Music from the terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/helmedeiros/amp/internal/adapter/applescript"
	"github.com/helmedeiros/amp/internal/adapter/cli"
	"github.com/helmedeiros/amp/internal/adapter/daemonclient"
	"github.com/helmedeiros/amp/internal/adapter/store"
	"github.com/helmedeiros/amp/internal/app"
	"github.com/helmedeiros/amp/internal/daemon"
)

func main() {
	svc := app.NewService(applescript.New(), store.NewFile(volumeStatePath()))

	// Serve status from the daemon when it is running (fast, cached); every
	// other call, and status when the daemon is down, goes direct.
	ctrl := daemonclient.NewController(svc, daemonclient.New(daemon.SocketPath()))

	root := cli.NewRootCmd(ctrl)

	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "amp:", err)
		os.Exit(1)
	}
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
