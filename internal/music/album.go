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
