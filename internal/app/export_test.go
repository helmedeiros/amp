package app

import "time"

// SetAlbumSyncTimingForTest overrides the album-sync poll interval and timeout,
// returning a function that restores the previous values.
func SetAlbumSyncTimingForTest(poll, timeout time.Duration) (restore func()) {
	prevPoll, prevTimeout := albumSyncPoll, albumSyncTimeout
	albumSyncPoll, albumSyncTimeout = poll, timeout
	return func() { albumSyncPoll, albumSyncTimeout = prevPoll, prevTimeout }
}
