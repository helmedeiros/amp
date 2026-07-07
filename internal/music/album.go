package music

// Album is a named album and the artist it is attributed to. Artist is the
// single track artist when the album's tracks agree, or "Various Artists" when
// they differ (a mixed album); it may be empty when unknown.
type Album struct {
	Name   string
	Artist string
}

// VariousArtists is the Artist value used for albums whose tracks span more
// than one artist.
const VariousArtists = "Various Artists"

// CatalogAlbum is an album offered by the Apple Music catalog (not necessarily
// in the library). It is used by the per-artist "add albums" flow.
type CatalogAlbum struct {
	ID         string
	Name       string
	TrackCount int
}

// AlbumCoverage reports how much of an album made it into the queue: how many
// tracks were queued from the library versus how many the album's metadata says
// it has. Total is 0 when the metadata does not record a track count.
type AlbumCoverage struct {
	Queued int
	Total  int
}

// Partial reports whether the album's metadata claims more tracks than were
// queued — the sign that some tracks are missing from the library (e.g. an
// Apple Music album only partly added).
func (c AlbumCoverage) Partial() bool {
	return c.Queued > 0 && c.Total > c.Queued
}
