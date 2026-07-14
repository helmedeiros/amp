package applescript

import (
	"encoding/json"
	"fmt"
)

// queuePlaylistName is the amp-owned playlist used as the play queue.
const queuePlaylistName = "amp queue"

// queueTracksScript reads the tracks currently in the queue playlist. A missing
// playlist (or any unreadable track) yields fewer entries rather than an error.
func queueTracksScript() string {
	qn, _ := json.Marshal(queuePlaylistName)
	return fmt.Sprintf(`
const Music = Application('Music');
const out = [];
try {
  const pl = Music.userPlaylists.byName(%s);
  pl.name();
  for (const t of pl.tracks()) {
    try { out.push({name: t.name(), artist: t.artist(), album: t.album(), duration: t.duration()}); } catch (e) {}
  }
} catch (e) {}
JSON.stringify(out);
`, qn)
}

// queueAddScript appends the search results to the queue playlist without
// clearing it or starting playback.
func queueAddScript(query string, limit int) string {
	q, _ := json.Marshal(query)
	qn, _ := json.Marshal(queuePlaylistName)
	return fmt.Sprintf(`
const Music = Application('Music');
const lib = Music.libraryPlaylists[0];
let raw = Music.search(lib, {for: %s, only: 'all'});
const limit = %d;
if (limit > 0) raw = raw.slice(0, limit);
let res = [];
for (const t of raw) {
  try { t.name(); t.artist(); t.album(); t.duration(); res.push(t); } catch (e) {}
}
let pl;
try { pl = Music.userPlaylists.byName(%s); pl.name(); }
catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: %s}}); }
for (const t of res) Music.duplicate(t, {to: pl});
JSON.stringify({added: res.length});
`, q, limit, qn, qn)
}

// playQueueAtScript plays the queue from the given index by rotating it so that
// track is first, then playing from the top. It duplicates the rotated tracks
// back into the queue and removes the originals (Music can't reliably start a
// playlist mid-way; see ADR-0004).
func playQueueAtScript(index int) string {
	qn, _ := json.Marshal(queuePlaylistName)
	return fmt.Sprintf(`
const Music = Application('Music');
const out = {played: false};
try {
  const pl = Music.userPlaylists.byName(%s);
  const tracks = pl.tracks();
  const n = tracks.length;
  if (n > 0) {
    const i = (((%d) %% n) + n) %% n;
    const rotated = tracks.slice(i).concat(tracks.slice(0, i));
    for (const t of rotated) Music.duplicate(t, {to: pl});
    for (let k = 0; k < n; k++) Music.delete(pl.tracks[0]);
    pl.play();
    out.played = true;
  }
} catch (e) {}
JSON.stringify(out);
`, qn, index)
}

// queueClearScript empties the queue playlist, keeping the playlist itself.
func queueClearScript() string {
	qn, _ := json.Marshal(queuePlaylistName)
	return fmt.Sprintf(`
const Music = Application('Music');
try { const pl = Music.userPlaylists.byName(%s); pl.name(); Music.delete(pl.tracks); } catch (e) {}
JSON.stringify({cleared: true});
`, qn)
}

// playPlaylistScript loads the named user playlist's tracks into the managed
// queue (a fast bulk duplicate) and plays it from the top, so the queue always
// reflects what is playing.
func playPlaylistScript(name string) string {
	n, _ := json.Marshal(name)
	qn, _ := json.Marshal(queuePlaylistName)
	return fmt.Sprintf(`
const Music = Application('Music');
const src = Music.userPlaylists.byName(%s);
let pl;
try { pl = Music.userPlaylists.byName(%s); pl.name(); Music.delete(pl.tracks); }
catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: %s}}); }
Music.duplicate(src.tracks, {to: pl});
pl.play();
`, n, qn, qn)
}

// playAlbumScript loads the named album into the queue in track order and plays
// it from the top. It also reports the album's own track count (the highest
// "track N of M" seen), so callers can tell when only part of an album is in the
// library.
func playAlbumScript(name string) string {
	n, _ := json.Marshal(name)
	qn, _ := json.Marshal(queuePlaylistName)

	return fmt.Sprintf(`
const Music = Application('Music');
const lib = Music.libraryPlaylists[0];
const want = %s;
let tracks = [];
let total = 0;
for (const t of Music.search(lib, {for: want, only: 'albums'})) {
  try {
    if (t.album().toLowerCase() === want.toLowerCase()) {
      tracks.push(t);
      const c = t.trackCount();
      if (c > total) total = c;
    }
  } catch (e) {}
}
tracks.sort((a, b) => { try { return a.trackNumber() - b.trackNumber(); } catch (e) { return 0; } });
if (tracks.length > 0) {
  let pl;
  try { pl = Music.userPlaylists.byName(%s); pl.name(); Music.delete(pl.tracks); }
  catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: %s}}); }
  for (const t of tracks) Music.duplicate(t, {to: pl});
  pl.play();
}
JSON.stringify({queued: tracks.length, total: total});
`, n, qn, qn)
}

// addFileScript imports a local audio file into the library and the named
// playlist, creating the playlist when it does not exist.
func addFileScript(path, playlist string) string {
	p, _ := json.Marshal(path)
	pn, _ := json.Marshal(playlist)
	return fmt.Sprintf(`
const Music = Application('Music');
let pl;
try { pl = Music.userPlaylists.byName(%s); pl.name(); }
catch (e) { pl = Music.make({new: 'playlist', withProperties: {name: %s}}); }
Music.add([Path(%s)], {to: pl});
JSON.stringify({added: true});
`, pn, pn, p)
}

// trackExistsScript reports whether a track with the given name and artist is
// already in the library. Music's `whose` rejects two criteria at once, so it
// filters by name and matches the artist in JS.
func trackExistsScript(name, artist string) string {
	n, _ := json.Marshal(name)
	a, _ := json.Marshal(artist)
	return fmt.Sprintf(`
const Music = Application('Music');
const lib = Music.libraryPlaylists[0];
let found = false;
try {
  const want = %s;
  for (const t of lib.tracks.whose({name: %s})()) {
    try { if (t.artist() === want) { found = true; break; } } catch (e) {}
  }
} catch (e) {}
JSON.stringify({exists: found});
`, a, n)
}

// albumCoverageScript counts the named album's tracks in the library and reads
// the album's own track count (highest "track N of M"), without touching
// playback. It uses a direct library query rather than search so the count is
// exact — the number used to await an iCloud sync.
func albumCoverageScript(name string) string {
	n, _ := json.Marshal(name)
	return fmt.Sprintf(`
const Music = Application('Music');
const lib = Music.libraryPlaylists[0];
const want = %s;
let ts = [];
try { ts = lib.tracks.whose({album: want})(); } catch (e) {}
let total = 0;
for (const t of ts) { try { const c = t.trackCount(); if (c > total) total = c; } catch (e) {} }
JSON.stringify({queued: ts.length, total: total});
`, n)
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
let raw = Music.search(lib, {for: %s, only: 'all'});
const limit = %d;
if (limit > 0) raw = raw.slice(0, limit);
// Keep only tracks we can fully read, matching the search listing so the
// chosen index still lines up (some search hits are stale, throwing -1728).
let res = [];
for (const t of raw) {
  try { t.name(); t.artist(); t.album(); t.duration(); res.push(t); } catch (e) {}
}
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
