package daemonclient

import (
	"context"

	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// statusSource reads a status snapshot (the daemon client satisfies it).
type statusSource interface {
	Status(ctx context.Context) (music.Status, error)
}

// Controller decorates another Controller, serving Status from the daemon when
// it is reachable and transparently falling back to the wrapped controller
// (direct engine access) otherwise. Every other call passes straight through.
type Controller struct {
	port.Controller
	daemon statusSource
}

// NewController wraps inner so that Status prefers the daemon.
func NewController(inner port.Controller, daemon statusSource) *Controller {
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
