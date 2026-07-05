// Package port declares the interfaces (ports) the application depends on.
// Adapters in internal/adapter implement these; the application never imports
// an adapter directly.
package port

import (
	"context"

	"github.com/helmedeiros/amp/internal/music"
)

// Player is the driven port for controlling the music engine (Music.app).
// Implementations translate these calls into engine operations; the
// application layer depends only on this interface.
type Player interface {
	// Status reads a snapshot of the current player state.
	Status(ctx context.Context) (music.Status, error)

	// Open launches the music application.
	Open(ctx context.Context) error
	// SaveArtwork writes the current track's album artwork to path.
	SaveArtwork(ctx context.Context, path string) error
	// Artwork returns the current track's album artwork bytes.
	Artwork(ctx context.Context) ([]byte, error)

	// Search returns library tracks matching query, up to limit (<= 0 for all).
	Search(ctx context.Context, query string, limit int) ([]music.Track, error)
	// PlaySearch loads the search results into the queue rotated so the track at
	// start is first, and plays from the top.
	PlaySearch(ctx context.Context, query string, limit, start int) error
	// PlayPlaylist plays the named user playlist.
	PlayPlaylist(ctx context.Context, name string) error
	// PlayAlbum loads the named album into the queue in track order and plays it.
	PlayAlbum(ctx context.Context, name string) error
	// PlayQueueAt plays the queue starting at the given index.
	PlayQueueAt(ctx context.Context, index int) error
	// Queue returns the tracks currently in the queue.
	Queue(ctx context.Context) ([]music.Track, error)
	// QueueAdd appends the search results to the queue and returns how many were
	// added.
	QueueAdd(ctx context.Context, query string, limit int) (int, error)
	// QueueClear empties the queue.
	QueueClear(ctx context.Context) error
	// Playlists returns the user's playlists.
	Playlists(ctx context.Context) ([]music.Playlist, error)
	// Artists returns the distinct, sorted artist names in the library.
	Artists(ctx context.Context) ([]string, error)
	// Albums returns the distinct, sorted album names in the library.
	Albums(ctx context.Context) ([]string, error)

	// Play resumes or starts playback.
	Play(ctx context.Context) error
	// Pause halts playback, keeping the current track loaded.
	Pause(ctx context.Context) error
	// TogglePlayPause flips between playing and paused.
	TogglePlayPause(ctx context.Context) error
	// Stop halts playback and unloads the current track.
	Stop(ctx context.Context) error
	// Next advances to the next track.
	Next(ctx context.Context) error
	// Previous returns to the previous track.
	Previous(ctx context.Context) error

	// SetVolume sets the sound volume.
	SetVolume(ctx context.Context, v music.Volume) error
	// SetPosition moves the player to an absolute position, in seconds.
	SetPosition(ctx context.Context, seconds float64) error
	// SetShuffle enables or disables shuffle.
	SetShuffle(ctx context.Context, enabled bool) error
	// SetRepeat sets the repeat mode.
	SetRepeat(ctx context.Context, mode music.RepeatMode) error
}
