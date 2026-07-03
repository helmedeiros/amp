package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/helmedeiros/amp/internal/music"
)

// scriptedReader returns a queued sequence of statuses/errors, one per call.
type scriptedReader struct {
	i    int
	seq  []music.Status
	errs []error
}

func (r *scriptedReader) Status(context.Context) (music.Status, error) {
	i := r.i
	if i >= len(r.seq) {
		i = len(r.seq) - 1
	}
	r.i++
	var err error
	if i < len(r.errs) {
		err = r.errs[i]
	}
	return r.seq[i], err
}

func playing(vol int) music.Status {
	return music.Status{State: music.Playing, Volume: music.NewVolume(vol)}
}

func TestDaemonCachesLatestStatus(t *testing.T) {
	t.Parallel()

	d := New(&scriptedReader{seq: []music.Status{playing(40)}}, time.Hour)

	_, ok := d.Status()
	assert.False(t, ok, "no status before the first poll")

	d.pollOnce(context.Background())

	got, ok := d.Status()
	require.True(t, ok)
	assert.Equal(t, 40, got.Volume.Int())
}

func TestDaemonReadErrorKeepsCache(t *testing.T) {
	t.Parallel()

	r := &scriptedReader{
		seq:  []music.Status{playing(40), playing(0)},
		errs: []error{nil, errors.New("osascript failed")},
	}
	d := New(r, time.Hour)

	d.pollOnce(context.Background()) // caches vol 40
	d.pollOnce(context.Background()) // errors, must keep vol 40

	got, _ := d.Status()
	assert.Equal(t, 40, got.Volume.Int())
}

func TestDaemonSubscribeReceivesCurrentAndChanges(t *testing.T) {
	t.Parallel()

	r := &scriptedReader{seq: []music.Status{playing(40), playing(55)}}
	d := New(r, time.Hour)
	d.pollOnce(context.Background()) // vol 40 cached

	ch, unsub := d.Subscribe()
	defer unsub()

	assert.Equal(t, 40, (<-ch).Volume.Int(), "current status delivered on subscribe")

	d.pollOnce(context.Background()) // vol 55 -> change
	assert.Equal(t, 55, (<-ch).Volume.Int())
}

func TestDaemonUnchangedStatusIsNotPushed(t *testing.T) {
	t.Parallel()

	r := &scriptedReader{seq: []music.Status{playing(40), playing(40)}}
	d := New(r, time.Hour)
	d.pollOnce(context.Background())

	ch, unsub := d.Subscribe()
	defer unsub()
	<-ch // drain the current value

	d.pollOnce(context.Background()) // same status -> no push

	select {
	case s := <-ch:
		t.Fatalf("unexpected push for unchanged status: %+v", s)
	case <-time.After(50 * time.Millisecond):
	}
}
