// Package daemon implements amd: a background process that polls Music.app,
// caches the latest status, and serves it (plus change events) to clients over
// a Unix socket using newline-delimited JSON.
package daemon

import (
	"time"

	"github.com/helmedeiros/amp/internal/music"
)

// Request is a single client command.
type Request struct {
	Cmd string `json:"cmd"` // "status", "subscribe", or "ping"
}

// Response is a single server message. For subscribe, the server sends one
// Response per change with Event set to "status".
type Response struct {
	Event  string     `json:"event,omitempty"`
	Status *StatusDTO `json:"status,omitempty"`
	Pong   bool       `json:"pong,omitempty"`
	Error  string     `json:"error,omitempty"`
}

// StatusDTO is the wire form of a player snapshot.
type StatusDTO struct {
	State          string    `json:"state"`
	Volume         int       `json:"volume"`
	Shuffle        bool      `json:"shuffle"`
	Repeat         string    `json:"repeat"`
	ElapsedSeconds float64   `json:"elapsed_seconds"`
	Track          *TrackDTO `json:"track,omitempty"`
}

// TrackDTO is the wire form of a track.
type TrackDTO struct {
	Name            string  `json:"name"`
	Artist          string  `json:"artist"`
	Album           string  `json:"album"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// ToStatusDTO converts a domain status to its wire form.
func ToStatusDTO(s music.Status) StatusDTO {
	dto := StatusDTO{
		State:          s.State.String(),
		Volume:         s.Volume.Int(),
		Shuffle:        s.Shuffle,
		Repeat:         s.Repeat.String(),
		ElapsedSeconds: s.Elapsed.Seconds(),
	}
	if s.HasTrack() {
		dto.Track = &TrackDTO{
			Name:            s.Track.Name,
			Artist:          s.Track.Artist,
			Album:           s.Track.Album,
			DurationSeconds: s.Track.Duration.Seconds(),
		}
	}
	return dto
}

// ToStatus converts a wire status back to the domain type. Unknown state or
// repeat values default to stopped/off rather than erroring.
func (dto StatusDTO) ToStatus() music.Status {
	state, _ := music.ParsePlayerState(dto.State)
	repeat, _ := music.ParseRepeatMode(dto.Repeat)

	s := music.Status{
		State:   state,
		Volume:  music.NewVolume(dto.Volume),
		Shuffle: dto.Shuffle,
		Repeat:  repeat,
		Elapsed: seconds(dto.ElapsedSeconds),
	}
	if dto.Track != nil {
		s.Track = music.Track{
			Name:     dto.Track.Name,
			Artist:   dto.Track.Artist,
			Album:    dto.Track.Album,
			Duration: seconds(dto.Track.DurationSeconds),
		}
	}
	return s
}

func seconds(f float64) time.Duration {
	return time.Duration(f * float64(time.Second))
}
