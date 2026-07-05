// Package cli is the driving adapter that exposes the application as terminal
// commands. It depends on the Controller port, never on a concrete adapter.
package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/helmedeiros/amp/internal/music"
)

// progressBarWidth is how many cells the status progress bar spans.
const progressBarWidth = 30

// labelWidth aligns the value column; the longest label is "shuffle" (7).
const labelWidth = 7

// RenderStatus formats a status snapshot for humans as aligned label/value
// rows, styled by theme. Playing and paused show the same fields (only the
// state word differs); stopped/no-track collapses to the state and volume.
//
// Padding is applied to the plain label before styling, so color codes never
// disturb column alignment.
func RenderStatus(s music.Status, theme Theme) string {
	var lines []string

	pad := func(label string) string { return fmt.Sprintf("%-*s", labelWidth, label) }
	row := func(styledLabel, value string) {
		lines = append(lines, strings.TrimRight(styledLabel+"  "+value, " "))
	}

	stateLabel := theme.state(s.State, pad(s.State.String()))

	if s.HasTrack() {
		row(stateLabel, theme.title(artistTitle(s.Track)))
		if s.Track.Album != "" {
			row(theme.label(pad("album")), s.Track.Album)
		}
		row(theme.label(pad("time")), timeLine(s))
		row(theme.label(pad("volume")), fmt.Sprintf("%d%%", s.Volume.Int()))
		row(theme.label(pad("shuffle")), onOff(s.Shuffle))
		row(theme.label(pad("repeat")), s.Repeat.String())
	} else {
		row(stateLabel, "")
		row(theme.label(pad("volume")), fmt.Sprintf("%d%%", s.Volume.Int()))
	}

	return strings.Join(lines, "\n")
}

// RenderTracks formats a track list, one "Artist — Title (Album)  m:ss" per
// line, or a short placeholder when the list is empty.
func RenderTracks(tracks []music.Track) string {
	if len(tracks) == 0 {
		return "no matches"
	}

	lines := make([]string, len(tracks))
	for i, t := range tracks {
		lines[i] = trackLine(t)
	}
	return strings.Join(lines, "\n")
}

// trackLine formats one track as "Artist — Title (Album)  m:ss", omitting the
// album and duration when unknown.
func trackLine(t music.Track) string {
	line := artistTitle(t)
	if t.Album != "" {
		line += " (" + t.Album + ")"
	}
	if t.Duration > 0 {
		line += "  " + FormatClock(t.Duration)
	}
	return line
}

// tracksJSON is the stable machine-readable shape of a track list.
type tracksJSON struct {
	Name            string  `json:"name"`
	Artist          string  `json:"artist"`
	Album           string  `json:"album"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// RenderTracksJSON formats a track list as a JSON array.
func RenderTracksJSON(tracks []music.Track) string {
	payload := make([]tracksJSON, len(tracks))
	for i, t := range tracks {
		payload[i] = tracksJSON{
			Name:            t.Name,
			Artist:          t.Artist,
			Album:           t.Album,
			DurationSeconds: t.Duration.Seconds(),
		}
	}

	out, _ := json.Marshal(payload)
	return string(out)
}

// RenderNames formats a list of names, one per line, or "empty" when there are
// none. With asJSON it returns a JSON array instead.
func RenderNames(names []string, asJSON bool) string {
	if asJSON {
		if names == nil {
			names = []string{}
		}
		out, _ := json.Marshal(names)
		return string(out)
	}
	if len(names) == 0 {
		return "empty"
	}
	return strings.Join(names, "\n")
}

// RenderAlbums formats an album list, one "Artist — Album" per line (just the
// album when the artist is unknown), or "empty" when there are none. With
// asJSON it returns a JSON array of {name, artist}.
func RenderAlbums(albums []music.Album, asJSON bool) string {
	if asJSON {
		payload := make([]albumJSON, len(albums))
		for i, a := range albums {
			payload[i] = albumJSON{Name: a.Name, Artist: a.Artist}
		}
		out, _ := json.Marshal(payload)
		return string(out)
	}
	if len(albums) == 0 {
		return "empty"
	}
	lines := make([]string, len(albums))
	for i, a := range albums {
		if a.Artist == "" {
			lines[i] = a.Name
			continue
		}
		lines[i] = a.Artist + " — " + a.Name
	}
	return strings.Join(lines, "\n")
}

// albumJSON is the stable machine-readable shape of an album.
type albumJSON struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
}

// RenderPlaylists formats a playlist list, one "Name  (N)" per line, or a
// short placeholder when empty.
func RenderPlaylists(playlists []music.Playlist) string {
	if len(playlists) == 0 {
		return "no playlists"
	}

	lines := make([]string, len(playlists))
	for i, p := range playlists {
		lines[i] = fmt.Sprintf("%s  (%d)", p.Name, p.Count)
	}
	return strings.Join(lines, "\n")
}

// playlistsJSON is the stable machine-readable shape of a playlist list.
type playlistsJSON struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// RenderPlaylistsJSON formats a playlist list as a JSON array.
func RenderPlaylistsJSON(playlists []music.Playlist) string {
	payload := make([]playlistsJSON, len(playlists))
	for i, p := range playlists {
		payload[i] = playlistsJSON{Name: p.Name, Count: p.Count}
	}

	out, _ := json.Marshal(payload)
	return string(out)
}

// RenderNow formats a one-line now-playing summary: "Artist — Title", or a
// short placeholder when nothing is loaded.
func RenderNow(s music.Status) string {
	if !s.HasTrack() {
		return "nothing playing"
	}
	return artistTitle(s.Track)
}

// artistTitle joins artist and title, or returns the title alone when the
// artist is unknown.
func artistTitle(t music.Track) string {
	if t.Artist == "" {
		return t.Name
	}
	return t.Artist + " — " + t.Name
}

// timeLine renders "elapsed / duration  <bar>  NN%". When the duration is
// unknown it shows a placeholder and omits the bar and percentage.
func timeLine(s music.Status) string {
	if s.Track.Duration <= 0 {
		return FormatClock(s.Elapsed) + " / --:--"
	}
	return fmt.Sprintf("%s / %s  %s  %d%%",
		FormatClock(s.Elapsed),
		FormatClock(s.Track.Duration),
		ProgressBar(s.Progress(), progressBarWidth),
		int(math.Round(s.Progress()*100)),
	)
}

// statusJSON is the stable machine-readable shape of a status snapshot.
type statusJSON struct {
	State          string     `json:"state"`
	Volume         int        `json:"volume"`
	Shuffle        bool       `json:"shuffle"`
	Repeat         string     `json:"repeat"`
	ElapsedSeconds float64    `json:"elapsed_seconds"`
	Track          *trackJSON `json:"track,omitempty"`
}

type trackJSON struct {
	Name            string  `json:"name"`
	Artist          string  `json:"artist"`
	Album           string  `json:"album"`
	DurationSeconds float64 `json:"duration_seconds"`
}

// RenderStatusJSON formats a status snapshot as a single JSON object.
func RenderStatusJSON(s music.Status) string {
	payload := statusJSON{
		State:          s.State.String(),
		Volume:         s.Volume.Int(),
		Shuffle:        s.Shuffle,
		Repeat:         s.Repeat.String(),
		ElapsedSeconds: s.Elapsed.Seconds(),
	}
	if s.HasTrack() {
		payload.Track = &trackJSON{
			Name:            s.Track.Name,
			Artist:          s.Track.Artist,
			Album:           s.Track.Album,
			DurationSeconds: s.Track.Duration.Seconds(),
		}
	}

	out, _ := json.Marshal(payload)
	return string(out)
}

// ProgressBar renders a fixed-width bar for a fraction in [0, 1]: filled cells
// for the elapsed portion, empty cells for the rest. Out-of-range fractions are
// clamped, and the result is always exactly width cells wide.
func ProgressBar(fraction float64, width int) string {
	if width <= 0 {
		return ""
	}
	fraction = math.Max(0, math.Min(1, fraction))

	filled := int(math.Round(fraction * float64(width)))
	return strings.Repeat("━", filled) + strings.Repeat("─", width-filled)
}

// FormatClock renders a duration as m:ss (minutes are not zero-padded).
func FormatClock(d time.Duration) string {
	total := int(d.Round(time.Second).Seconds())
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}
