package applescript

import (
	"encoding/json"
	"fmt"

	"github.com/helmedeiros/amp/internal/music"
)

// searchScript builds a JXA program that searches the library and prints the
// matching tracks as JSON. The query is embedded as a JSON literal so it is
// safely escaped and cannot break out of the script. A limit <= 0 returns all
// matches.
func searchScript(query string, limit int) string {
	q, _ := json.Marshal(query) // string marshaling never fails

	return fmt.Sprintf(`
const Music = Application('Music');
const out = [];
if (Music.running()) {
  let res = Music.search(Music.libraryPlaylists[0], {for: %s, only: 'all'});
  const limit = %d;
  if (limit > 0) res = res.slice(0, limit);
  for (const t of res) {
    // Search can return stale references that throw -1728 on every property;
    // skip any track we cannot fully read.
    try {
      out.push({name: t.name(), artist: t.artist(), album: t.album(), duration: t.duration()});
    } catch (e) {}
  }
}
JSON.stringify(out);
`, q, limit)
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
