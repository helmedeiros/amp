package music

// Playlist is a named collection of tracks in the library.
type Playlist struct {
	Name  string
	Count int
	// Artists are the distinct track artists in the playlist, used for
	// artist-aware filtering (find playlists that contain a given artist).
	Artists []string
}
