package port

import (
	"context"

	"github.com/helmedeiros/amp/internal/music"
)

// SoundCloud is the driven port for importing tracks from SoundCloud into the
// library (via external tools). Optional: nil unless the tools are wired.
type SoundCloud interface {
	// List returns the tracks at a profile, set, or track URL.
	List(ctx context.Context, url string) ([]music.SoundCloudTrack, error)
	// Fetch downloads trackURL, writes a tagged audio file into destDir, and
	// returns its path.
	Fetch(ctx context.Context, trackURL, destDir string, tag music.Attribution) (path string, err error)
}

// ImportResult summarises a SoundCloud import.
type ImportResult struct {
	Imported []string // "Artist — Title" of newly added tracks
	Skipped  int      // already in the library
	Failed   int      // download or add errors
}
