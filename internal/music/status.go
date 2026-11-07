package music

import "time"

// Track is a single song in the Music library.
type Track struct {
	Name     string
	Artist   string
	Album    string
	Duration time.Duration
}

// IsZero reports whether the track carries no information.
func (t Track) IsZero() bool {
	return t == Track{}
}

// Status is a snapshot of the player, as read in a single status query.
type Status struct {
	State   PlayerState
	Track   Track
	Elapsed time.Duration
	Volume  Volume
	Shuffle bool
	Repeat  RepeatMode
	Source  Source
}

// HasTrack reports whether a track is currently loaded.
func (s Status) HasTrack() bool {
	return !s.Track.IsZero()
}

// Progress returns how far the current track has played, as a fraction in
// [0, 1]. It is 0 when the track has no known duration.
func (s Status) Progress() float64 {
	if s.Track.Duration <= 0 {
		return 0
	}

	p := float64(s.Elapsed) / float64(s.Track.Duration)
	if p > 1 {
		return 1
	}
	if p < 0 {
		return 0
	}
	return p
}
