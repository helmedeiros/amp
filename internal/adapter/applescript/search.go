package applescript

import (
	"encoding/json"
	"fmt"

	"github.com/helmedeiros/amp/internal/music"
)

const libraryTracksScript = `
const Music = Application('Music');
let out = {};
if (Music.running()) {
  const tr = Music.libraryPlaylists[0].tracks;
  out = {
    id: tr.persistentID(), name: tr.name(), artist: tr.artist(), album: tr.album(),
    duration: tr.duration(), plays: tr.playedCount(), number: tr.trackNumber(),
  };
}
JSON.stringify(out);
`

type libraryColumns struct {
	ID       []string  `json:"id"`
	Name     []string  `json:"name"`
	Artist   []string  `json:"artist"`
	Album    []string  `json:"album"`
	Duration []float64 `json:"duration"`
	Plays    []int     `json:"plays"`
	Number   []int     `json:"number"`
}

func parseLibraryTracks(raw []byte) ([]music.LibraryTrack, error) {
	var c libraryColumns
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("decode library: %w", err)
	}
	n := min(len(c.ID), len(c.Name), len(c.Artist), len(c.Album), len(c.Duration), len(c.Plays), len(c.Number))
	tracks := make([]music.LibraryTrack, n)
	for i := range n {
		tracks[i] = music.LibraryTrack{
			Track: music.Track{
				Name:     c.Name[i],
				Artist:   c.Artist[i],
				Album:    c.Album[i],
				Duration: seconds(c.Duration[i]),
			},
			ID:          c.ID[i],
			Plays:       c.Plays[i],
			TrackNumber: c.Number[i],
		}
	}
	return tracks, nil
}

func queueIDsScript(ids []string, play bool) string {
	want, _ := json.Marshal(ids)
	qn, _ := json.Marshal(queuePlaylistName)
	clear := ""
	if play {
		clear = "Music.delete(pl.tracks);"
	}
	return fmt.Sprintf(`
const Music = Application('Music');
const lib = Music.libraryPlaylists[0];
const want = %s;
const index = new Map();
const all = lib.tracks.persistentID();
for (let i = 0; i < all.length; i++) index.set(all[i], i);
let pl;
try { pl = Music.userPlaylists.byName(%s); pl.name(); %s }
catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: %s}}); }
let n = 0;
for (const id of want) {
  const i = index.get(id);
  if (i === undefined) continue;
  try { Music.duplicate(lib.tracks[i], {to: pl}); n++; } catch (e) {}
}
if (%t && n > 0) pl.play();
JSON.stringify({queued: n});
`, want, qn, clear, qn, play)
}

func trackIDs(tracks []music.LibraryTrack) []string {
	ids := make([]string, len(tracks))
	for i, t := range tracks {
		ids[i] = t.ID
	}
	return ids
}

func rotate[T any](s []T, start int) []T {
	n := len(s)
	if n == 0 {
		return s
	}
	i := ((start % n) + n) % n
	return append(append([]T{}, s[i:]...), s[:i]...)
}

// parseTracks decodes a JSON array of tracks into the domain type.
func parseTracks(raw []byte) ([]music.Track, error) {
	var dtos []trackDTO
	if err := json.Unmarshal(raw, &dtos); err != nil {
		return nil, fmt.Errorf("decode tracks: %w", err)
	}

	tracks := make([]music.Track, len(dtos))
	for i, d := range dtos {
		tracks[i] = d.toTrack()
	}
	return tracks, nil
}
