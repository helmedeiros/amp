package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/helmedeiros/amp/internal/music"
)

// Reader is the narrow driven port the daemon needs: a single status read.
// The AppleScript adapter's Player satisfies it.
type Reader interface {
	Status(ctx context.Context) (music.Status, error)
}

// Daemon polls a Reader on an interval, caches the latest status, and fans out
// change events to subscribers.
type Daemon struct {
	reader   Reader
	interval time.Duration

	mu     sync.RWMutex
	last   music.Status
	hasVal bool
	subs   map[chan music.Status]struct{}
}

// New creates a Daemon that polls reader every interval.
func New(reader Reader, interval time.Duration) *Daemon {
	return &Daemon{
		reader:   reader,
		interval: interval,
		subs:     make(map[chan music.Status]struct{}),
	}
}

// Run polls until ctx is cancelled, returning ctx.Err().
func (d *Daemon) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	d.pollOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			d.pollOnce(ctx)
		}
	}
}

// pollOnce reads the status and, when it changed, updates the cache and
// notifies subscribers. A read error leaves the cache untouched.
func (d *Daemon) pollOnce(ctx context.Context) {
	s, err := d.reader.Status(ctx)
	if err != nil {
		return
	}

	d.mu.Lock()
	changed := !d.hasVal || d.last != s
	d.last, d.hasVal = s, true
	var targets []chan music.Status
	if changed {
		targets = make([]chan music.Status, 0, len(d.subs))
		for ch := range d.subs {
			targets = append(targets, ch)
		}
	}
	d.mu.Unlock()

	for _, ch := range targets {
		send(ch, s)
	}
}

// Status returns the cached status and whether one has been read yet.
func (d *Daemon) Status() (music.Status, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.last, d.hasVal
}

// Subscribe returns a channel that receives the current status (if known) and
// every subsequent change, plus a function to unsubscribe.
func (d *Daemon) Subscribe() (<-chan music.Status, func()) {
	ch := make(chan music.Status, 1)

	d.mu.Lock()
	d.subs[ch] = struct{}{}
	cur, has := d.last, d.hasVal
	d.mu.Unlock()

	if has {
		send(ch, cur)
	}

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			d.mu.Lock()
			delete(d.subs, ch)
			d.mu.Unlock()
			close(ch)
		})
	}
	return ch, unsub
}

// send delivers the latest status without blocking: a full (buffer 1) channel
// is drained first so the subscriber always sees the newest value.
func send(ch chan music.Status, s music.Status) {
	select {
	case ch <- s:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- s:
		default:
		}
	}
}
