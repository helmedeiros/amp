package applescript

import (
	"encoding/json"
	"fmt"
)

// queuePlaylistName is the amp-owned playlist used as the play queue.
const queuePlaylistName = "amp queue"

// playPlaylistScript plays a named user playlist.
func playPlaylistScript(name string) string {
	n, _ := json.Marshal(name)
	return fmt.Sprintf(`
const Music = Application('Music');
Music.userPlaylists.byName(%s).play();
`, n)
}

// playAlbumScript loads the named album into the queue in track order and plays
// it from the top.
func playAlbumScript(name string) string {
	n, _ := json.Marshal(name)
	qn, _ := json.Marshal(queuePlaylistName)

	return fmt.Sprintf(`
const Music = Application('Music');
const lib = Music.libraryPlaylists[0];
const want = %s;
let tracks = Music.search(lib, {for: want, only: 'albums'})
  .filter(t => t.album().toLowerCase() === want.toLowerCase());
tracks.sort((a, b) => a.trackNumber() - b.trackNumber());
if (tracks.length > 0) {
  let pl;
  try { pl = Music.userPlaylists.byName(%s); pl.name(); Music.delete(pl.tracks); }
  catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: %s}}); }
  for (const t of tracks) Music.duplicate(t, {to: pl});
  pl.play();
}
JSON.stringify({queued: tracks.length});
`, n, qn, qn)
}

// playSearchScript builds a JXA program that re-runs the library search, loads
// the results into the managed queue playlist rotated so the chosen track is
// first, and plays the playlist from the top. Everything after the pick plays
// next; earlier results sit behind it. Music.app's live "Up Next" is not
// scriptable, so rotating and playing from the top is how we honour the pick
// (see ADR-0004).
func playSearchScript(query string, limit, start int) string {
	q, _ := json.Marshal(query)
	name, _ := json.Marshal(queuePlaylistName)

	return fmt.Sprintf(`
const Music = Application('Music');
const lib = Music.libraryPlaylists[0];
let res = Music.search(lib, {for: %s, only: 'all'});
const limit = %d;
if (limit > 0) res = res.slice(0, limit);
if (res.length > 0) {
  const s = (((%d) %% res.length) + res.length) %% res.length;
  res = res.slice(s).concat(res.slice(0, s));
  let pl;
  try { pl = Music.userPlaylists.byName(%s); pl.name(); Music.delete(pl.tracks); }
  catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: %s}}); }
  for (const t of res) Music.duplicate(t, {to: pl});
  pl.play();
}
JSON.stringify({queued: res.length});
`, q, limit, start, name, name)
}
