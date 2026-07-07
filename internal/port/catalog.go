package port

import (
	"context"

	"github.com/helmedeiros/amp/internal/music"
)

// Catalog is the driven port for the Apple Music catalog HTTP API. It resolves
// album names to catalog IDs and adds catalog albums to the user's library, so
// the app can fill in tracks that streaming left out of the local library.
// It is optional: when no credentials are configured the application skips it.
type Catalog interface {
	// ResolveAlbum returns the catalog ID of the album best matching name and
	// artist, or an empty ID when nothing matches.
	ResolveAlbum(ctx context.Context, name, artist string) (albumID string, err error)
	// ArtistAlbums returns the artist's catalog albums (deduped to one edition
	// each, singles excluded), for the per-artist "add albums" flow.
	ArtistAlbums(ctx context.Context, artist string) ([]music.CatalogAlbum, error)
	// AddAlbum adds the catalog album to the user's iCloud Music Library. It is
	// idempotent: adding an album already present is a no-op.
	AddAlbum(ctx context.Context, albumID string) error
}
