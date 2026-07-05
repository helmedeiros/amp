package port

import (
	"context"
	"time"

	"github.com/helmedeiros/amp/internal/music"
)

// PlayResult reports what a smart play resolved to.
type PlayResult struct {
	Kind  string // "resume", "playlist", "album", or "track"
	Label string // playlist/album name or "Artist — Title"; empty for resume
}

// Controller is the driving port: the use-case surface that driving adapters
// (the CLI, later the TUI) depend on. The application's Service implements it.
type Controller interface {
	Status(ctx context.Context) (music.Status, error)

	// Open launches the music application.
	Open(ctx context.Context) error
	// SaveArtwork writes the current track's album artwork to path.
	SaveArtwork(ctx context.Context, path string) error
	// Artwork returns the current track's album artwork bytes.
	Artwork(ctx context.Context) ([]byte, error)

	// PlayQuery resumes playback when query is empty, otherwise resolves it to a
	// playlist, album, or track search and plays it.
	PlayQuery(ctx context.Context, query string, limit int) (PlayResult, error)
	// Search returns library tracks matching query, up to limit (<= 0 for all).
	Search(ctx context.Context, query string, limit int) ([]music.Track, error)
	// PlaySearch plays the search results starting at the chosen index, queueing
	// the rest.
	PlaySearch(ctx context.Context, query string, limit, start int) error
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

	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Toggle(ctx context.Context) error
	Stop(ctx context.Context) error
	Next(ctx context.Context) error
	Previous(ctx context.Context) error

	// SetVolume sets an absolute level and returns the applied volume.
	SetVolume(ctx context.Context, level int) (music.Volume, error)
	// AdjustVolume shifts the level by delta and returns the new volume.
	AdjustVolume(ctx context.Context, delta int) (music.Volume, error)

	// Seek moves the player position per mode/value and returns the new position.
	Seek(ctx context.Context, mode music.SeekMode, value float64) (time.Duration, error)

	// SetShuffle enables or disables shuffle.
	SetShuffle(ctx context.Context, enabled bool) error
	// ToggleShuffle flips shuffle and returns the new value.
	ToggleShuffle(ctx context.Context) (bool, error)
	// SetRepeat sets the repeat mode.
	SetRepeat(ctx context.Context, mode music.RepeatMode) error

	// Mute silences playback, remembering the current level.
	Mute(ctx context.Context) error
	// Unmute restores the remembered level and returns the applied volume.
	Unmute(ctx context.Context) (music.Volume, error)
}
