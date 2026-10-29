package applescript

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/helmedeiros/amp/internal/music"
)

// playlistsScript lists the user's playlists as JSON, skipping the built-in
// library playlists (which report a non-"none" special kind). It reads each
// playlist's artist column once and derives both the track count and the
// distinct artists from it — a single traversal, so it is no slower than
// counting alone, and it lets callers find playlists that contain an artist.
const playlistsScript = `
const Music = Application('Music');
const out = [];
if (Music.running()) {
  for (const p of Music.userPlaylists()) {
    if (p.specialKind() !== 'none') continue;
    let arts = [];
    try { arts = p.tracks.artist(); } catch (e) {}
    out.push({
      name: p.name(),
      count: arts.length,
      artists: [...new Set(arts.filter(a => a && a.length))],
    });
  }
}
JSON.stringify(out);
`

// librarySignatureScript is a fast change detector: the playlist names plus the
// library's track total, joined into one string. It is a fraction of the cost of
// playlistsScript because it reads no per-playlist tracks.
const librarySignatureScript = `
const Music = Application('Music');
let sig = '';
if (Music.running()) {
  sig = Music.userPlaylists.name().join('\n') + '|' + Music.libraryPlaylists[0].tracks.length;
}
sig;
`

// LibrarySignature returns a cheap fingerprint of the library's shape.
func (p *Player) LibrarySignature(ctx context.Context) (string, error) {
	out, err := p.run.Run(ctx, javaScript, librarySignatureScript)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type playlistDTO struct {
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Artists []string `json:"artists"`
}

// parsePlaylists decodes a JSON array of playlists into the domain type.
func parsePlaylists(raw []byte) ([]music.Playlist, error) {
	var dtos []playlistDTO
	if err := json.Unmarshal(raw, &dtos); err != nil {
		return nil, fmt.Errorf("decode playlists: %w", err)
	}

	playlists := make([]music.Playlist, len(dtos))
	for i, d := range dtos {
		playlists[i] = music.Playlist{Name: d.Name, Count: d.Count, Artists: d.Artists}
	}
	return playlists, nil
}
