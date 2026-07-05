package applescript

import (
	"encoding/json"
	"fmt"

	"github.com/helmedeiros/amp/internal/music"
)

// namesScript builds a JXA program that returns the sorted, de-duplicated,
// non-empty values of a track field ("artist" or "album") across the library.
// field is a fixed internal literal, never user input.
func namesScript(field string) string {
	return fmt.Sprintf(`
const Music = Application('Music');
let out = [];
if (Music.running()) {
  const vals = Music.libraryPlaylists[0].tracks.%s();
  out = [...new Set(vals)].filter(v => v && v.length).sort((a, b) => a.localeCompare(b));
}
JSON.stringify(out);
`, field)
}

// parseNames decodes a JSON array of strings.
func parseNames(raw []byte) ([]string, error) {
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil, fmt.Errorf("decode names: %w", err)
	}
	return names, nil
}

// albumsScript returns the library's albums as JSON, each with the artist its
// tracks agree on, or "Various Artists" when they differ (a mixed album). It
// reads the album and artist columns in two bulk fetches and groups in memory,
// so it stays fast even on large libraries.
const albumsScript = `
const Music = Application('Music');
const out = [];
if (Music.running()) {
  const tr = Music.libraryPlaylists[0].tracks;
  const albums = tr.album();
  const artists = tr.artist();
  const byAlbum = new Map();
  for (let i = 0; i < albums.length; i++) {
    const al = albums[i];
    if (!al || !al.length) continue;
    let set = byAlbum.get(al);
    if (!set) { set = new Set(); byAlbum.set(al, set); }
    const ar = artists[i];
    if (ar && ar.length) set.add(ar);
  }
  const names = [...byAlbum.keys()].sort((a, b) => a.localeCompare(b));
  for (const al of names) {
    const set = byAlbum.get(al);
    let artist = '';
    if (set.size === 1) artist = [...set][0];
    else if (set.size > 1) artist = 'Various Artists';
    out.push({album: al, artist: artist});
  }
}
JSON.stringify(out);
`

type albumDTO struct {
	Album  string `json:"album"`
	Artist string `json:"artist"`
}

// parseAlbums decodes a JSON array of {album, artist} into the domain type.
func parseAlbums(raw []byte) ([]music.Album, error) {
	var dtos []albumDTO
	if err := json.Unmarshal(raw, &dtos); err != nil {
		return nil, fmt.Errorf("decode albums: %w", err)
	}
	albums := make([]music.Album, len(dtos))
	for i, d := range dtos {
		albums[i] = music.Album{Name: d.Album, Artist: d.Artist}
	}
	return albums, nil
}
