// Package applescript is the driven adapter that controls Music.app through
// osascript. It is the only package that knows how the engine is operated;
// everything above it depends on the port, not on this code.
package applescript

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/helmedeiros/amp/internal/music"
	"github.com/helmedeiros/amp/internal/port"
)

// Player controls Music.app and implements port.Player.
type Player struct {
	run runner
}

// New returns a Player backed by the real osascript binary.
func New() *Player {
	return newPlayer(execRunner{})
}

// newPlayer builds a Player with an explicit runner (used by tests).
func newPlayer(r runner) *Player {
	return &Player{run: r}
}

var _ port.Player = (*Player)(nil)

// tellMusic wraps an action in the AppleScript command that targets Music.app.
func tellMusic(action string) string {
	return `tell application "Music" to ` + action
}

// Status reads the full player snapshot in a single osascript call.
func (p *Player) Status(ctx context.Context) (music.Status, error) {
	out, err := p.run.Run(ctx, javaScript, statusScript)
	if err != nil {
		return music.Status{}, err
	}
	return parseStatus(out)
}

// Open launches Music.app and brings it to the front.
func (p *Player) Open(ctx context.Context) error {
	return p.tell(ctx, "activate")
}

// SaveArtwork writes the current track's album artwork to path.
func (p *Player) SaveArtwork(ctx context.Context, path string) error {
	_, err := p.run.Run(ctx, appleScript, saveArtworkScript(path))
	return err
}

// Search returns library tracks matching query, up to limit (<= 0 for all).
func (p *Player) Search(ctx context.Context, query string, limit int) ([]music.Track, error) {
	out, err := p.run.Run(ctx, javaScript, searchScript(query, limit))
	if err != nil {
		return nil, err
	}
	return parseTracks(out)
}

// PlaySearch loads the search results into the managed queue, rotated so the
// track at start is first, and plays from the top.
func (p *Player) PlaySearch(ctx context.Context, query string, limit, start int) error {
	_, err := p.run.Run(ctx, javaScript, playSearchScript(query, limit, start))
	return err
}

// PlayPlaylist plays the named user playlist.
func (p *Player) PlayPlaylist(ctx context.Context, name string) error {
	_, err := p.run.Run(ctx, javaScript, playPlaylistScript(name))
	return err
}

// PlayAlbum loads the named album into the queue in track order and plays it,
// returning how much of the album was available in the library.
func (p *Player) PlayAlbum(ctx context.Context, name string) (music.AlbumCoverage, error) {
	out, err := p.run.Run(ctx, javaScript, playAlbumScript(name))
	if err != nil {
		return music.AlbumCoverage{}, err
	}
	var res struct {
		Queued int `json:"queued"`
		Total  int `json:"total"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return music.AlbumCoverage{}, nil // played fine; coverage is just unknown
	}
	return music.AlbumCoverage{Queued: res.Queued, Total: res.Total}, nil
}

// AlbumCoverage reports how much of the named album is in the library without
// changing playback.
func (p *Player) AlbumCoverage(ctx context.Context, name string) (music.AlbumCoverage, error) {
	out, err := p.run.Run(ctx, javaScript, albumCoverageScript(name))
	if err != nil {
		return music.AlbumCoverage{}, err
	}
	var res struct {
		Queued int `json:"queued"`
		Total  int `json:"total"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return music.AlbumCoverage{}, fmt.Errorf("decode album coverage: %w", err)
	}
	return music.AlbumCoverage{Queued: res.Queued, Total: res.Total}, nil
}

// PlayQueueAt plays the queue starting at the given index.
func (p *Player) PlayQueueAt(ctx context.Context, index int) error {
	_, err := p.run.Run(ctx, javaScript, playQueueAtScript(index))
	return err
}

// Queue returns the tracks currently in the queue.
func (p *Player) Queue(ctx context.Context) ([]music.Track, error) {
	out, err := p.run.Run(ctx, javaScript, queueTracksScript())
	if err != nil {
		return nil, err
	}
	return parseTracks(out)
}

// QueueAdd appends the search results to the queue and returns how many were
// added.
func (p *Player) QueueAdd(ctx context.Context, query string, limit int) (int, error) {
	out, err := p.run.Run(ctx, javaScript, queueAddScript(query, limit))
	if err != nil {
		return 0, err
	}
	var res struct {
		Added int `json:"added"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return 0, fmt.Errorf("decode queue add: %w", err)
	}
	return res.Added, nil
}

// QueueClear empties the queue.
func (p *Player) QueueClear(ctx context.Context) error {
	_, err := p.run.Run(ctx, javaScript, queueClearScript())
	return err
}

// Playlists returns the user's playlists.
func (p *Player) Playlists(ctx context.Context) ([]music.Playlist, error) {
	out, err := p.run.Run(ctx, javaScript, playlistsScript)
	if err != nil {
		return nil, err
	}
	return parsePlaylists(out)
}

// Artists returns the distinct, sorted artist names in the library.
func (p *Player) Artists(ctx context.Context) ([]string, error) {
	return p.names(ctx, "artist")
}

// Albums returns the distinct, sorted albums in the library, each with its
// artist (or "Various Artists" for mixed albums).
func (p *Player) Albums(ctx context.Context) ([]music.Album, error) {
	out, err := p.run.Run(ctx, javaScript, albumsScript)
	if err != nil {
		return nil, err
	}
	return parseAlbums(out)
}

func (p *Player) names(ctx context.Context, field string) ([]string, error) {
	out, err := p.run.Run(ctx, javaScript, namesScript(field))
	if err != nil {
		return nil, err
	}
	return parseNames(out)
}

// Play resumes or starts playback.
func (p *Player) Play(ctx context.Context) error {
	return p.tell(ctx, "play")
}

// Pause halts playback.
func (p *Player) Pause(ctx context.Context) error {
	return p.tell(ctx, "pause")
}

// TogglePlayPause flips between playing and paused.
func (p *Player) TogglePlayPause(ctx context.Context) error {
	return p.tell(ctx, "playpause")
}

// Stop halts playback and unloads the current track.
func (p *Player) Stop(ctx context.Context) error {
	return p.tell(ctx, "stop")
}

// Next advances to the next track.
func (p *Player) Next(ctx context.Context) error {
	return p.tell(ctx, "next track")
}

// Previous returns to the previous track.
func (p *Player) Previous(ctx context.Context) error {
	return p.tell(ctx, "previous track")
}

// SetVolume sets the sound volume.
func (p *Player) SetVolume(ctx context.Context, v music.Volume) error {
	return p.tell(ctx, fmt.Sprintf("set sound volume to %d", v.Int()))
}

// SetPosition moves the player to an absolute position, in seconds.
func (p *Player) SetPosition(ctx context.Context, seconds float64) error {
	return p.tell(ctx, fmt.Sprintf("set player position to %g", seconds))
}

// SetShuffle enables or disables shuffle.
func (p *Player) SetShuffle(ctx context.Context, enabled bool) error {
	return p.tell(ctx, fmt.Sprintf("set shuffle enabled to %t", enabled))
}

// SetRepeat sets the repeat mode.
func (p *Player) SetRepeat(ctx context.Context, mode music.RepeatMode) error {
	return p.tell(ctx, "set song repeat to "+mode.String())
}

// tell runs an AppleScript action against Music.app, discarding its output.
func (p *Player) tell(ctx context.Context, action string) error {
	_, err := p.run.Run(ctx, appleScript, tellMusic(action))
	return err
}
