package daemonclient

import (
	"context"

	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// source reads the daemon's cached snapshots (the daemon client satisfies it).
type source interface {
	Status(ctx context.Context) (music.Status, error)
	LibrarySignature(ctx context.Context) (string, error)
}

// Controller decorates another Controller, serving Status and LibrarySignature
// from the daemon when it is reachable and transparently falling back to the
// wrapped controller (direct engine access) otherwise. Every other call passes
// straight through.
type Controller struct {
	port.Controller
	daemon source
}

// NewController wraps inner so that Status and LibrarySignature prefer the
// daemon.
func NewController(inner port.Controller, daemon source) *Controller {
	return &Controller{Controller: inner, daemon: daemon}
}

// Status returns the daemon's cached snapshot, or the wrapped controller's
// direct read when the daemon is unavailable.
func (c *Controller) Status(ctx context.Context) (music.Status, error) {
	if s, err := c.daemon.Status(ctx); err == nil {
		return s, nil
	}
	return c.Controller.Status(ctx)
}

// LibrarySignature returns the daemon's cached fingerprint, or the wrapped
// controller's direct read when the daemon is unavailable.
func (c *Controller) LibrarySignature(ctx context.Context) (string, error) {
	if sig, err := c.daemon.LibrarySignature(ctx); err == nil {
		return sig, nil
	}
	return c.Controller.LibrarySignature(ctx)
}
