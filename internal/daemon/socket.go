package daemon

import (
	"os"
	"path/filepath"
)

// SocketPath returns the Unix socket amd listens on and clients connect to.
// It honours AMP_SOCKET, then $XDG_RUNTIME_DIR, falling back to the temp dir.
func SocketPath() string {
	if p := os.Getenv("AMP_SOCKET"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "amp.sock")
}
