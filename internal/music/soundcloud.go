package music

// SoundCloudTrack is a track discovered on a SoundCloud profile or set.
type SoundCloudTrack struct {
	URL      string
	Title    string
	Uploader string
}

// Attribution is the artist/album/name an imported track is tagged with.
type Attribution struct {
	Name   string
	Artist string
	Album  string
}
